package bisync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
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
		opt:      &Options{Resync: true},
	}
	reversed := &bisyncRun{
		fs1: right, fs2: left,
		basePath: filepath.Join(workDir, "reversed"),
		opt:      &Options{Resync: true},
	}
	require.NoError(t, forward.setLockFile())
	require.ErrorContains(t, reversed.setLockFile(), "another bisync run")
	require.NoError(t, forward.removeLockFile())
	require.NoError(t, reversed.setLockFile())
	require.NoError(t, reversed.removeLockFile())
}

func TestCollidingBasenamesExcludeDryAndLiveRuns(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	one, err := mockfs.NewFs(ctx, "local", "parent/child", nil)
	require.NoError(t, err)
	alias, err := mockfs.NewFs(ctx, "local", "parent_child", nil)
	require.NoError(t, err)
	other, err := mockfs.NewFs(ctx, "local", "other", nil)
	require.NoError(t, err)
	first := &bisyncRun{fs1: one, fs2: other, basePath: bilib.BasePath(ctx, root, one, other), opt: &Options{DryRun: true, Resync: true}}
	second := &bisyncRun{fs1: alias, fs2: other, basePath: bilib.BasePath(ctx, root, alias, other), opt: &Options{DryRun: true, Resync: true}}
	require.Equal(t, first.basePath, second.basePath)
	require.NotEqual(t, first.pairGuardPath(), second.pairGuardPath())
	require.NoError(t, first.setLockFile())
	require.ErrorContains(t, second.setLockFile(), "another bisync run")
	require.NoError(t, first.removeLockFile())
	require.NoError(t, second.setLockFile())
	require.NoError(t, second.removeLockFile())

	first.opt.DryRun = false
	first.opt.Resync = true
	second.opt.DryRun = true
	require.NoError(t, first.setLockFile())
	require.ErrorContains(t, second.setLockFile(), "another bisync run")
	require.NoError(t, first.removeLockFile())
}

func TestBisyncBaselineIdentityRequiresExplicitReviewedResync(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	a, err := mockfs.NewFs(ctx, "local", "folder/nested", nil)
	require.NoError(t, err)
	b, err := mockfs.NewFs(ctx, "local", "folder_nested", nil)
	require.NoError(t, err)
	right, err := mockfs.NewFs(ctx, "local", "second", nil)
	require.NoError(t, err)
	makeRun := func(left fs.Fs, resync bool) *bisyncRun {
		base := bilib.BasePath(ctx, root, left, right)
		return &bisyncRun{fs1: left, fs2: right, basePath: base,
			listing1: base + ".path1.lst", listing2: base + ".path2.lst", opt: &Options{Resync: resync}}
	}
	legacy := makeRun(a, false)
	require.Equal(t, legacy.basePath, makeRun(b, false).basePath)
	require.NoError(t, os.WriteFile(legacy.listing1, []byte("legacy state"), 0600))
	require.NoError(t, os.WriteFile(legacy.listing2, []byte("legacy state"), 0600))
	require.ErrorContains(t, legacy.setLockFile(), "--resync")
	require.ErrorContains(t, makeRun(b, false).setLockFile(), "--resync")
	// Preview never binds a legacy identity.
	preview := makeRun(a, true)
	preview.opt.DryRun = true
	require.NoError(t, preview.setLockFile())
	require.NoError(t, preview.removeLockFile())
	require.ErrorContains(t, legacy.setLockFile(), "--resync")

	initial := makeRun(a, true)
	require.NoError(t, initial.setLockFile())
	initial.abort = true
	require.ErrorContains(t, initial.commitBaselineIdentity(), "did not finish cleanly")
	require.NoError(t, initial.removeLockFile())
	// Unsuccessful/interrupted resync leaves a pending, unusable identity.
	require.ErrorContains(t, makeRun(a, false).setLockFile(), "--resync")
	initial = makeRun(a, true)
	require.NoError(t, initial.setLockFile())
	require.NoError(t, initial.commitBaselineIdentity())
	require.NoError(t, initial.removeLockFile())
	matching := makeRun(a, false)
	require.NoError(t, matching.setLockFile())
	require.NoError(t, matching.removeLockFile())
	require.ErrorContains(t, makeRun(b, false).setLockFile(), "different")

	// Explicit resync changes ownership only when successful.
	changeOwner := makeRun(b, true)
	require.NoError(t, changeOwner.setLockFile())
	require.NoError(t, changeOwner.commitBaselineIdentity())
	require.NoError(t, changeOwner.removeLockFile())
	require.ErrorContains(t, makeRun(a, false).setLockFile(), "different")
	final := makeRun(b, false)
	require.NoError(t, final.setLockFile())
	require.NoError(t, final.removeLockFile())
}

func TestBaselineIdentityHandlesDotDelimitersAndConfigSuffixes(t *testing.T) {
	ctx := context.Background()
	first, err := mockfs.NewFs(ctx, "local", "a..b", nil)
	require.NoError(t, err)
	second, err := mockfs.NewFs(ctx, "local", "c", nil)
	require.NoError(t, err)
	third, err := mockfs.NewFs(ctx, "local", "a", nil)
	require.NoError(t, err)
	fourth, err := mockfs.NewFs(ctx, "local", "b..c", nil)
	require.NoError(t, err)
	require.Equal(t, bilib.SessionName(first, second), bilib.SessionName(third, fourth))
	require.NotEqual(t, orderedBaselineIdentity(first, second), orderedBaselineIdentity(third, fourth))
	leftA, err := mockfs.NewFs(ctx, "remote", "data{abc}", nil)
	require.NoError(t, err)
	leftB, err := mockfs.NewFs(ctx, "remote", "data{def}", nil)
	require.NoError(t, err)
	require.Equal(t, bilib.SessionName(leftA, second), bilib.SessionName(leftB, second))
	require.NotEqual(t, orderedBaselineIdentity(leftA, second), orderedBaselineIdentity(leftB, second))
	require.NotEqual(t, orderedBaselineIdentity(second, leftA), orderedBaselineIdentity(leftA, second))

	for _, tc := range []struct {
		name string
		one  fs.Fs
		two  fs.Fs
		p1   fs.Fs
		p2   fs.Fs
	}{
		{name: "dot delimiter", one: first, two: second, p1: third, p2: fourth},
		{name: "config suffix", one: leftA, two: second, p1: leftB, p2: second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			base := bilib.BasePath(ctx, workDir, tc.one, tc.two)
			require.Equal(t, base, bilib.BasePath(ctx, workDir, tc.p1, tc.p2))
			owner := &bisyncRun{fs1: tc.one, fs2: tc.two, basePath: base, opt: &Options{Resync: true}}
			require.NoError(t, owner.setLockFile())
			require.NoError(t, owner.commitBaselineIdentity())
			require.NoError(t, owner.removeLockFile())
			alias := &bisyncRun{fs1: tc.p1, fs2: tc.p2, basePath: base, opt: &Options{}}
			require.ErrorContains(t, alias.setLockFile(), "different")
		})
	}
}

func TestCheckSyncOnlyCannotBindAResyncIdentity(t *testing.T) {
	ctx := context.Background()
	left, err := mockfs.NewFs(ctx, "local", "left", nil)
	require.NoError(t, err)
	right, err := mockfs.NewFs(ctx, "local", "right", nil)
	require.NoError(t, err)
	b := &bisyncRun{
		fs1: left, fs2: right,
		basePath: bilib.BasePath(ctx, t.TempDir(), left, right),
		opt:      &Options{Resync: true, CheckSync: CheckSyncOnly},
	}
	require.NoError(t, b.setLockFile())
	require.ErrorContains(t, b.commitBaselineIdentity(), "cannot establish")
	require.NoError(t, b.removeLockFile())
	b.opt = &Options{}
	require.ErrorContains(t, b.setLockFile(), "--resync")
}
