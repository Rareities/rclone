package bisync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLockfileBisyncRun(t *testing.T, lockContent string, maxLock fs.Duration) *bisyncRun {
	t.Helper()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lck")
	require.NoError(t, os.WriteFile(lockPath, []byte(lockContent), 0600))

	listing1 := filepath.Join(dir, "listing1")
	listing2 := filepath.Join(dir, "listing2")
	require.NoError(t, os.WriteFile(listing1, []byte(""), 0600))
	require.NoError(t, os.WriteFile(listing2, []byte(""), 0600))

	return &bisyncRun{
		lockFile: lockPath,
		opt:      &Options{MaxLock: maxLock},
		listing1: listing1,
		listing2: listing2,
	}
}

func TestLockfileIsExpired_UnreadableWithMaxLock(t *testing.T) {
	b := newTestLockfileBisyncRun(t, "not json!!!", fs.Duration(5*time.Minute))
	assert.True(t, b.lockFileIsExpired(), "unreadable lockfile with --max-lock set should be treated as expired")
}

func TestLockfileIsExpired_UnreadableWithoutMaxLock(t *testing.T) {
	b := newTestLockfileBisyncRun(t, "not json!!!", basicallyforever)
	assert.False(t, b.lockFileIsExpired(), "unreadable lockfile without --max-lock should not be treated as expired")
}

func TestLockfileIsExpired_ValidExpired(t *testing.T) {
	data := struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}{
		Session:     "test",
		PID:         "12345",
		TimeRenewed: time.Now().Add(-10 * time.Minute),
		TimeExpires: time.Now().Add(-5 * time.Minute),
	}
	content, err := json.Marshal(data)
	require.NoError(t, err)

	b := newTestLockfileBisyncRun(t, string(content), fs.Duration(5*time.Minute))
	assert.True(t, b.lockFileIsExpired(), "valid lockfile with past expiry should be expired")
}

func TestLockfileIsExpired_ValidNotExpired(t *testing.T) {
	data := struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}{
		Session:     "test",
		PID:         "12345",
		TimeRenewed: time.Now(),
		TimeExpires: time.Now().Add(10 * time.Minute),
	}
	content, err := json.Marshal(data)
	require.NoError(t, err)

	b := newTestLockfileBisyncRun(t, string(content), fs.Duration(5*time.Minute))
	assert.False(t, b.lockFileIsExpired(), "valid lockfile with future expiry should not be expired")
}

func TestLockfileExclusiveCreation(t *testing.T) {
	base := filepath.Join(t.TempDir(), "pair")
	var wg sync.WaitGroup
	winners := make(chan *bisyncRun, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := &bisyncRun{basePath: base, opt: &Options{}}
			if b.setLockFile() == nil {
				winners <- b
			}
		}()
	}
	wg.Wait()
	close(winners)
	var owners []*bisyncRun
	for b := range winners {
		owners = append(owners, b)
	}
	require.Len(t, owners, 1)
	require.NoError(t, owners[0].removeLockFile())
}

func TestLockfileDoesNotModifyReplacement(t *testing.T) {
	b := &bisyncRun{basePath: filepath.Join(t.TempDir(), "pair"), opt: &Options{}}
	require.NoError(t, b.setLockFile())
	path := b.lockFile
	require.NoError(t, os.Rename(path, path+".old"))
	replacement := []byte("replacement owner")
	require.NoError(t, os.WriteFile(path, replacement, 0600))
	b.renewLockFile()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, replacement, data)
	assert.Error(t, b.removeLockFile())
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, replacement, data)
}

func TestLockfileExpiredTakeover(t *testing.T) {
	b := newTestLockfileBisyncRun(t, "invalid expired legacy lock", fs.Duration(5*time.Minute))
	b.basePath = strings.TrimSuffix(b.lockFile, ".lck")
	require.NoError(t, b.setLockFile())
	require.FileExists(t, b.listing1+"-err")
	require.FileExists(t, b.listing2+"-err")
	require.NoError(t, b.removeLockFile())
}

func TestLockfileRenewAndRemove(t *testing.T) {
	b := &bisyncRun{basePath: filepath.Join(t.TempDir(), "pair"), opt: &Options{MaxLock: fs.Duration(2 * time.Minute)}}
	require.NoError(t, b.setLockFile())
	b.renewLockFile()
	assert.False(t, b.lockFileIsExpired())
	require.NoError(t, b.removeLockFile())
	require.NoError(t, b.removeLockFile())
	assert.NoFileExists(t, b.basePath+".lck")
}

func TestLockfileConcurrentRemoval(t *testing.T) {
	for range 30 {
		b := &bisyncRun{basePath: filepath.Join(t.TempDir(), "pair"), opt: &Options{MaxLock: fs.Duration(2 * time.Minute)}}
		require.NoError(t, b.setLockFile())
		var wg sync.WaitGroup
		errors := make(chan error, 2)
		wg.Go(b.renewLockFile)
		for range 2 {
			wg.Go(func() { errors <- b.removeLockFile() })
		}
		wg.Wait()
		close(errors)
		for err := range errors {
			require.NoError(t, err)
		}
		require.NoFileExists(t, b.basePath+".lck")
	}
}

func TestLockfileDoesNotModifyRewrittenOwner(t *testing.T) {
	b := &bisyncRun{basePath: filepath.Join(t.TempDir(), "pair"), opt: &Options{}}
	require.NoError(t, b.setLockFile())
	path := b.lockFile
	replacement := []byte(`{"Owner":"another session"}`)
	require.NoError(t, os.WriteFile(path, replacement, 0600))
	b.renewLockFile()
	assert.Error(t, b.removeLockFile())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, replacement, data)
}

func TestLockfileProcessHelper(t *testing.T) {
	base := os.Getenv("RCLONE_BISYNC_LOCK_TEST_PATH")
	if base == "" {
		return
	}
	fmt.Println("started")
	b := &bisyncRun{basePath: base, opt: &Options{}}
	require.NoError(t, b.setLockFile())
	fmt.Println("acquired")
	require.NoError(t, b.removeLockFile())
}

func TestLockfileGuardAcrossProcesses(t *testing.T) {
	base := filepath.Join(t.TempDir(), "pair")
	guard, err := lockGuard(base + ".lck")
	require.NoError(t, err)
	defer func() { _ = guard.Close() }()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockfileProcessHelper$")
	cmd.Env = append(os.Environ(), "RCLONE_BISYNC_LOCK_TEST_PATH="+base)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "started\n", line)
	result := make(chan string, 1)
	go func() { line, _ := reader.ReadString('\n'); result <- line }()
	select {
	case line := <-result:
		t.Fatalf("child bypassed guard: %q", line)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, guard.Close())
	select {
	case line := <-result:
		require.Equal(t, "acquired\n", line)
	case <-time.After(10 * time.Second):
		t.Fatal("child did not acquire released guard")
	}
	require.NoError(t, cmd.Wait())
}
