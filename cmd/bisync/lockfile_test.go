package bisync

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newLockfileTestRun(t *testing.T, basePath string) *bisyncRun {
	t.Helper()
	return &bisyncRun{
		basePath: basePath,
		opt:      &Options{},
	}
}

func TestLockfileAcquireAndRelease(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	b := newLockfileTestRun(t, basePath)

	require.NoError(t, b.setLockFile())
	metadata, err := readLockMetadata(basePath + ".lck")
	require.NoError(t, err)
	require.Equal(t, lockFileVersion, metadata.Version)
	require.Equal(t, "active", metadata.State)
	require.NotEmpty(t, metadata.OwnerToken)
	require.Empty(t, metadata.TimeExpires)

	require.NoError(t, b.removeLockFile())
	metadata, err = readLockMetadata(basePath + ".lck")
	require.NoError(t, err)
	require.Equal(t, "released", metadata.State)
	require.True(t, metadata.TimeExpires.Before(time.Now()))
	require.FileExists(t, basePath+".lck.guard")
}

func TestLockfileDryRunDoesNotCreateLockFiles(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	b := newLockfileTestRun(t, basePath)
	b.opt.DryRun = true

	require.NoError(t, b.setLockFile())
	require.Empty(t, b.lockFile)
	require.NoFileExists(t, basePath+".lck")
	require.NoFileExists(t, basePath+".lck.guard")
	require.NoError(t, b.removeLockFile())
}

func TestLockfileRejectsSecondOwnerInSameProcess(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	first := newLockfileTestRun(t, basePath)
	second := newLockfileTestRun(t, basePath)

	require.NoError(t, first.setLockFile())
	err := second.setLockFile()
	require.ErrorContains(t, err, "prior Bisync run is active")
	require.NoError(t, first.removeLockFile())
	require.NoError(t, second.setLockFile())
	require.NoError(t, second.removeLockFile())
}

func TestLockfileRejectsLegacyOrUnreadableMetadata(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "legacy", content: `{"Session":"old","PID":"123","TimeExpires":"2000-01-01T00:00:00Z"}`},
		{name: "unreadable", content: `not json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			basePath := filepath.Join(t.TempDir(), "profile")
			lockPath := basePath + ".lck"
			require.NoError(t, os.WriteFile(lockPath, []byte(tc.content), 0600))

			b := newLockfileTestRun(t, basePath)
			err := b.setLockFile()
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), "legacy or unsupported") || strings.Contains(err.Error(), "unreadable"), err)
			got, readErr := os.ReadFile(lockPath)
			require.NoError(t, readErr)
			require.Equal(t, tc.content, string(got), "failed acquisition must not rewrite untrusted metadata")
		})
	}
}

func TestLockfileRejectsValidLegacyMetadata(t *testing.T) {
	legacy, err := json.Marshal(struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}{
		Session:     "old",
		PID:         "123",
		TimeRenewed: time.Now().Add(-time.Hour),
		TimeExpires: time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)

	basePath := filepath.Join(t.TempDir(), "profile")
	lockPath := basePath + ".lck"
	require.NoError(t, os.WriteFile(lockPath, legacy, 0600))
	err = newLockfileTestRun(t, basePath).setLockFile()
	require.ErrorContains(t, err, "legacy or unsupported")
	got, readErr := os.ReadFile(lockPath)
	require.NoError(t, readErr)
	require.Equal(t, legacy, got, "legacy metadata is preserved for explicit recovery")
}

func TestLockfileStaleHeartbeatDoesNotPermitConcurrentOwner(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	owner := newLockfileTestRun(t, basePath)
	require.NoError(t, owner.setLockFile())

	metadata, err := readLockMetadata(basePath + ".lck")
	require.NoError(t, err)
	metadata.TimeRenewed = time.Now().Add(-24 * time.Hour)
	require.NoError(t, writeLockMetadata(basePath+".lck", metadata, false))

	contender := newLockfileTestRun(t, basePath)
	require.ErrorContains(t, contender.setLockFile(), "prior Bisync run is active")
	require.NoError(t, owner.removeLockFile())
}

func TestLockfileRenewalAndReleaseSerialize(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	b := newLockfileTestRun(t, basePath)
	require.NoError(t, b.setLockFile())

	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 32 {
		workers.Go(func() {
			<-start
			for range 8 {
				b.renewLockFile()
			}
		})
	}
	close(start)
	require.NoError(t, b.removeLockFile())
	workers.Wait()

	metadata, err := readLockMetadata(basePath + ".lck")
	require.NoError(t, err)
	require.Equal(t, "released", metadata.State, "late heartbeat must not reactivate a released owner")
}

func TestLockfileDelayedFormerOwnerCannotReleaseNewOwner(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	former := newLockfileTestRun(t, basePath)
	require.NoError(t, former.setLockFile())
	formerToken := former.lockFileOpt.ownerToken
	require.NoError(t, former.lockFileOpt.guard.Unlock(), "simulate an owner whose process lock was lost")

	current := newLockfileTestRun(t, basePath)
	require.NoError(t, current.setLockFile())
	currentToken := current.lockFileOpt.ownerToken
	require.NotEqual(t, formerToken, currentToken)
	require.ErrorContains(t, former.removeLockFile(), "ownership was lost")

	metadata, err := readLockMetadata(basePath + ".lck")
	require.NoError(t, err)
	require.Equal(t, currentToken, metadata.OwnerToken, "stale release must not overwrite the new owner's metadata")
	require.Equal(t, "active", metadata.State)
	require.NoError(t, current.removeLockFile())
}

func TestLockfileHelperProcess(t *testing.T) {
	basePath := os.Getenv("RCLONE_BISYNC_LOCK_HELPER_BASE")
	if basePath == "" {
		return
	}

	b := newLockfileTestRun(t, basePath)
	if err := b.setLockFile(); err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "error: %v\n", err)
		os.Exit(2)
	}
	_, _ = fmt.Fprintln(os.Stdout, "locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := b.removeLockFile(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "release error: %v\n", err)
		os.Exit(3)
	}
}

func TestLockfileSerializesIndependentProcesses(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	command := exec.Command(os.Args[0], "-test.run=^TestLockfileHelperProcess$")
	command.Env = append(os.Environ(), "RCLONE_BISYNC_LOCK_HELPER_BASE="+basePath)
	stdin, err := command.StdinPipe()
	require.NoError(t, err)
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	require.NoError(t, command.Start())
	processDone := false
	t.Cleanup(func() {
		if !processDone {
			_ = stdin.Close()
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err, stderr.String())
	require.Equal(t, "locked\n", line, stderr.String())

	contender := newLockfileTestRun(t, basePath)
	require.ErrorContains(t, contender.setLockFile(), "prior Bisync run is active")
	require.NoError(t, stdin.Close())
	require.NoError(t, command.Wait(), stderr.String())
	processDone = true

	// Acquiring after the helper exits proves ownership is released without
	// deleting/replacing the persistent guard inode.
	require.NoError(t, contender.setLockFile())
	require.NoError(t, contender.removeLockFile())
}

func TestLockfileReclaimsAfterOwnerProcessCrash(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "profile")
	command := exec.Command(os.Args[0], "-test.run=^TestLockfileHelperProcess$")
	command.Env = append(os.Environ(), "RCLONE_BISYNC_LOCK_HELPER_BASE="+basePath)
	stdin, err := command.StdinPipe()
	require.NoError(t, err)
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	require.NoError(t, command.Start())
	processDone := false
	t.Cleanup(func() {
		if !processDone {
			_ = stdin.Close()
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err, stderr.String())
	require.Equal(t, "locked\n", line, stderr.String())
	formerOwner, err := readLockMetadata(basePath + ".lck")
	require.NoError(t, err)

	require.NoError(t, command.Process.Kill())
	require.Error(t, command.Wait(), "forced process termination must be observable")
	processDone = true

	contender := newLockfileTestRun(t, basePath)
	require.NoError(t, contender.setLockFile(), "the OS releases process ownership after a crash")
	currentOwner, err := readLockMetadata(basePath + ".lck")
	require.NoError(t, err)
	require.NotEqual(t, formerOwner.OwnerToken, currentOwner.OwnerToken)
	require.Equal(t, "active", currentOwner.State)
	require.NoError(t, contender.removeLockFile())
}
