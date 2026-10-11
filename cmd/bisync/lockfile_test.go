package bisync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockfs"
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

func TestLockfileConcurrentAcquisitionHasOneOwner(t *testing.T) {
	const contenders = 12
	base := filepath.Join(t.TempDir(), "pair")
	start := make(chan struct{})
	results := make(chan *bisyncRun, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := &bisyncRun{basePath: base, opt: &Options{}}
			<-start
			if b.setLockFile() == nil {
				results <- b
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var owners []*bisyncRun
	for b := range results {
		owners = append(owners, b)
	}
	require.Len(t, owners, 1)
	require.NoError(t, owners[0].removeLockFile())
	require.NoError(t, owners[0].removeLockFile(), "cleanup must be idempotent")
	next := &bisyncRun{basePath: base, opt: &Options{}}
	require.NoError(t, next.setLockFile())
	require.NoError(t, next.removeLockFile())
}

func TestLockfileCleanupDoesNotRemoveReplacement(t *testing.T) {
	b := &bisyncRun{basePath: filepath.Join(t.TempDir(), "pair"), opt: &Options{}}
	require.NoError(t, b.setLockFile())
	path := b.lockFile
	require.NoError(t, os.Rename(path, path+".original"))
	require.NoError(t, os.WriteFile(path, []byte("new owner"), 0600))
	require.Error(t, b.removeLockFile())
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "new owner", string(contents))
}

func TestLockfileReversePairCannotRunConcurrently(t *testing.T) {
	ctx := context.Background()
	left, err := mockfs.NewFs(ctx, "left", "one", nil)
	require.NoError(t, err)
	right, err := mockfs.NewFs(ctx, "right", "two", nil)
	require.NoError(t, err)

	workDir := t.TempDir()
	forward := &bisyncRun{
		fs1: left, fs2: right,
		basePath: filepath.Join(workDir, "forward"),
		opt:      &Options{},
	}
	reversed := &bisyncRun{
		fs1: right, fs2: left,
		basePath: filepath.Join(workDir, "reversed"),
		opt:      &Options{},
	}
	require.NoError(t, forward.setLockFile())
	require.ErrorContains(t, reversed.setLockFile(), "another bisync run")
	require.NoError(t, forward.removeLockFile())
	require.NoError(t, reversed.setLockFile())
	require.NoError(t, reversed.removeLockFile())
}
