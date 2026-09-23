package bisync

import (
	"context"
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/require"
)

func TestInspectStateDoesNotCreateMissingWorkdir(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fs1, fs2 := newInspectTestFilesystems(t, ctx)
	workDir := filepath.Join(t.TempDir(), "missing")

	result := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equal(t, StateAbsent, result.Status)
	require.Equal(t, "WORKDIR_ABSENT", result.Reason)
	_, err := os.Stat(workDir)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestInspectStateCompatibleAndReadOnlyForListings(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fs1, fs2 := newInspectTestFilesystems(t, ctx)
	workDir := t.TempDir()
	base := bilib.SessionName(fs1, fs2)
	writeInspectTestLock(t, base, workDir, "released")
	for _, side := range []string{".path1.lst", ".path2.lst"} {
		list := newFileList()
		list.put("nested/file.txt", 12, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), "", "-", "-")
		require.NoError(t, list.save(filepath.Join(workDir, base+side)))
	}
	before1, err := os.ReadFile(filepath.Join(workDir, base+".path1.lst"))
	require.NoError(t, err)
	before2, err := os.ReadFile(filepath.Join(workDir, base+".path2.lst"))
	require.NoError(t, err)

	result := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equal(t, StateCompatible, result.Status)
	require.Equal(t, "LISTINGS_COMPATIBLE", result.Reason)
	require.Equal(t, before1, mustRead(t, filepath.Join(workDir, base+".path1.lst")))
	require.Equal(t, before2, mustRead(t, filepath.Join(workDir, base+".path2.lst")))
}

func TestDryRunPreservesRootBytesAndCanonicalListings(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	leftRoot, rightRoot := filepath.Join(t.TempDir(), "left"), filepath.Join(t.TempDir(), "right")
	workDir := t.TempDir()
	for _, root := range []string{leftRoot, rightRoot} {
		require.NoError(t, os.MkdirAll(root, 0o700))
	}
	leftFile, rightFile := filepath.Join(leftRoot, "same-metadata.txt"), filepath.Join(rightRoot, "same-metadata.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("alpha-1234"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("bravo-1234"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(leftRoot, "unchanged.txt"), []byte("stable"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rightRoot, "unchanged.txt"), []byte("stable"), 0o600))
	sharedTime := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	require.NoError(t, os.Chtimes(leftFile, sharedTime, sharedTime))
	require.NoError(t, os.Chtimes(rightFile, sharedTime, sharedTime))

	fs1, err := local.NewFs(ctx, "local", leftRoot, configmap.Simple{})
	require.NoError(t, err)
	fs2, err := local.NewFs(ctx, "local", rightRoot, configmap.Simple{})
	require.NoError(t, err)
	fs1 = stateInspectionTestFs{Fs: fs1, name: "local", root: "left"}
	fs2 = stateInspectionTestFs{Fs: fs2, name: "local", root: "right"}

	// An initialization preview has no accepted listing to copy. It must leave both roots
	// untouched and must still inspect as absent after native dry-run scratch files are written.
	require.NoError(t, Bisync(ctx, fs1, fs2, &Options{
		Workdir:        workDir,
		Resync:         true,
		ResyncMode:     PreferPath1,
		MaxDeleteCount: DefaultMaxDeleteCount,
		CompareFlag:    "size,checksum",
		DryRun:         true,
	}))
	require.Equal(t, "alpha-1234", string(mustRead(t, leftFile)))
	require.Equal(t, "bravo-1234", string(mustRead(t, rightFile)))
	inspection := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equal(t, StateAbsent, inspection.Status,
		"a successful dry-run initialization must not appear as native initialization or recovery state")

	require.NoError(t, Bisync(ctx, fs1, fs2, &Options{
		Workdir:        workDir,
		Resync:         true,
		ResyncMode:     PreferPath1,
		MaxDeleteCount: DefaultMaxDeleteCount,
		CompareFlag:    "size,checksum",
	}))
	require.Equal(t, "alpha-1234", string(mustRead(t, leftFile)))
	require.Equal(t, "alpha-1234", string(mustRead(t, rightFile)))

	// Change the bytes without changing size or modification time. This forces the native
	// comparison/listing logic to handle a same-size/same-time content change during preview.
	require.NoError(t, os.WriteFile(leftFile, []byte("bravo-1234"), 0o600))
	require.NoError(t, os.Chtimes(leftFile, sharedTime, sharedTime))
	base := bilib.BasePath(ctx, workDir, fs1, fs2)
	listing1, listing2 := base+".path1.lst", base+".path2.lst"
	beforeListing1, err := os.ReadFile(listing1)
	require.NoError(t, err)
	beforeListing2, err := os.ReadFile(listing2)
	require.NoError(t, err)

	err = Bisync(ctx, fs1, fs2, &Options{
		Workdir:        workDir,
		MaxDeleteCount: DefaultMaxDeleteCount,
		CompareFlag:    "size,checksum",
		DryRun:         true,
	})
	require.NoError(t, err)
	require.Equal(t, "bravo-1234", string(mustRead(t, leftFile)), "dry-run must not change Path1")
	require.Equal(t, "alpha-1234", string(mustRead(t, rightFile)), "dry-run must not change Path2")
	require.Equal(t, beforeListing1, mustRead(t, listing1), "dry-run must preserve Path1's accepted listing byte-for-byte")
	require.Equal(t, beforeListing2, mustRead(t, listing2), "dry-run must preserve Path2's accepted listing byte-for-byte")

	inspection = InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equal(t, StateCompatible, inspection.Status,
		"preview scratch artifacts must not invalidate the native accepted baseline")
}

func TestInspectStateRejectsMalformedListingInsteadOfSkippingIt(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fs1, fs2 := newInspectTestFilesystems(t, ctx)
	workDir := t.TempDir()
	base := bilib.SessionName(fs1, fs2)
	writeInspectTestLock(t, base, workDir, "released")
	for _, side := range []string{".path1.lst", ".path2.lst"} {
		content := ListingHeader + " " + time.Now().UTC().Format(timeFormat) + "\nnot a listing row\n"
		require.NoError(t, os.WriteFile(filepath.Join(workDir, base+side), []byte(content), 0o600))
	}

	result := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equal(t, StateIncompatible, result.Status)
	require.Equal(t, "PATH1_LISTING_INVALID", result.Reason)
}

func TestInspectStateRejectsFileDirectoryTypeMismatch(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fs1, fs2 := newInspectTestFilesystems(t, ctx)
	workDir := t.TempDir()
	base := bilib.SessionName(fs1, fs2)
	writeInspectTestLock(t, base, workDir, "released")
	mtime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	fileList1, fileList2 := newFileList(), newFileList()
	fileList1.put("node", 4, mtime, "", "-", "-")
	fileList2.put("node", 4, mtime, "", "-", "d")
	require.NoError(t, fileList1.save(filepath.Join(workDir, base+".path1.lst")))
	require.NoError(t, fileList2.save(filepath.Join(workDir, base+".path2.lst")))

	result := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equal(t, StateIncompatible, result.Status)
	require.Equal(t, "LISTINGS_DIVERGED", result.Reason)
}

func TestInspectStateDoesNotMigrateLegacyListingNames(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fs1, fs2 := newInspectTestFilesystems(t, ctx)
	fs1 = stateInspectionTestFs{Fs: fs1, name: "left{abcdef}", root: "root"}
	fs2 = stateInspectionTestFs{Fs: fs2, name: "right{012345}", root: "root"}
	workDir := t.TempDir()
	legacySession := bilib.CanonicalPath(bilib.FsPath(fs1)) + ".." + bilib.CanonicalPath(bilib.FsPath(fs2))
	legacyListing := filepath.Join(workDir, legacySession+".path1.lst")
	require.NoError(t, os.WriteFile(legacyListing, []byte("legacy state"), 0o600))
	require.True(t, bilib.HasHexString(legacySession), "fixture must use a canonical legacy suffix: %q", legacySession)
	require.FileExists(t, legacyListing)

	result := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equalf(t, StateUnknown, result.Status, "inspection: %+v", result)
	require.Equal(t, "LEGACY_NAME_REQUIRES_EXPLICIT_MIGRATION", result.Reason)
	require.Equal(t, []byte("legacy state"), mustRead(t, legacyListing))
	currentListing := filepath.Join(workDir, bilib.SessionName(fs1, fs2)+".path1.lst")
	_, err := os.Stat(currentListing)
	require.True(t, errors.Is(err, os.ErrNotExist), "legacy listing must not be renamed into the current format")
}

func TestInspectStateReportsRecoverableInterruptedStateWithoutRestoring(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fs1, fs2 := newInspectTestFilesystems(t, ctx)
	workDir := t.TempDir()
	base := bilib.SessionName(fs1, fs2)
	writeInspectTestLock(t, base, workDir, "released")
	for _, side := range []string{".path1.lst-old", ".path2.lst-old"} {
		list := newFileList()
		list.put("keep.txt", 9, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), "", "-", "-")
		require.NoError(t, list.save(filepath.Join(workDir, base+side)))
	}
	old1 := mustRead(t, filepath.Join(workDir, base+".path1.lst-old"))
	old2 := mustRead(t, filepath.Join(workDir, base+".path2.lst-old"))

	result := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equalf(t, StateInterrupted, result.Status, "inspection: %+v", result)
	require.Equal(t, "CURRENT_LISTINGS_MISSING", result.Reason)
	require.True(t, result.RecoveryListingsValid)
	require.Equal(t, old1, mustRead(t, filepath.Join(workDir, base+".path1.lst-old")))
	require.Equal(t, old2, mustRead(t, filepath.Join(workDir, base+".path2.lst-old")))
	_, err := os.Stat(filepath.Join(workDir, base+".path1.lst"))
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestInspectStateBlocksWhileNativeOwnerHoldsGuard(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	fs1, fs2 := newInspectTestFilesystems(t, ctx)
	workDir := t.TempDir()
	base := bilib.SessionName(fs1, fs2)
	guard := flock.New(filepath.Join(workDir, base+".lck.guard"),
		flock.SetFlag(os.O_CREATE|os.O_RDWR), flock.SetPermissions(bilib.PermSecure))
	acquired, err := guard.TryLock()
	require.NoError(t, err)
	require.True(t, acquired)
	defer func() { require.NoError(t, guard.Unlock()) }()

	result := InspectState(ctx, fs1, fs2, &Options{Workdir: workDir})
	require.Equal(t, StateUnknown, result.Status)
	require.Equal(t, "NATIVE_RUN_ACTIVE", result.Reason)
}

func TestResyncBackupDirPreservesSameSizeSameModtimeLoser(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	config := fs.GetConfig(ctx)
	priorIgnoreTimes := config.IgnoreTimes
	config.IgnoreTimes = true
	t.Cleanup(func() { config.IgnoreTimes = priorIgnoreTimes })

	leftRoot, rightRoot := filepath.Join(t.TempDir(), "left"), filepath.Join(t.TempDir(), "right")
	workDir, backup1, backup2 := t.TempDir(), filepath.Join(t.TempDir(), "backup1"), filepath.Join(t.TempDir(), "backup2")
	for _, root := range []string{leftRoot, rightRoot, backup1, backup2} {
		require.NoError(t, os.MkdirAll(root, 0o700))
	}
	leftFile, rightFile := filepath.Join(leftRoot, "conflict.txt"), filepath.Join(rightRoot, "conflict.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("alpha-version"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("bravo-version"), 0o600))
	sharedTime := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	require.NoError(t, os.Chtimes(leftFile, sharedTime, sharedTime))
	require.NoError(t, os.Chtimes(rightFile, sharedTime, sharedTime))

	left, err := local.NewFs(ctx, "local", leftRoot, configmap.Simple{})
	require.NoError(t, err)
	right, err := local.NewFs(ctx, "local", rightRoot, configmap.Simple{})
	require.NoError(t, err)
	left = stateInspectionTestFs{Fs: left, name: "local", root: "left"}
	right = stateInspectionTestFs{Fs: right, name: "local", root: "right"}

	err = Bisync(ctx, left, right, &Options{
		Workdir:        workDir,
		Resync:         true,
		ResyncMode:     PreferPath1,
		MaxDeleteCount: DefaultMaxDeleteCount,
		BackupDir1:     backup1,
		BackupDir2:     backup2,
	})
	require.NoError(t, err)
	require.Equal(t, "alpha-version", string(mustRead(t, leftFile)))
	require.Equal(t, "alpha-version", string(mustRead(t, rightFile)))

	var retained []string
	err = filepath.WalkDir(backup2, func(file string, entry iofs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			bytes, readErr := os.ReadFile(file)
			if readErr != nil {
				return readErr
			}
			retained = append(retained, string(bytes))
		}
		return nil
	})
	require.NoError(t, err)
	require.True(t, strings.Contains(strings.Join(retained, "\n"), "bravo-version"),
		"the overwritten Path2 bytes must remain in its run-scoped backup directory")
	restored := filepath.Join(t.TempDir(), "restored.txt")
	for _, content := range retained {
		if content == "bravo-version" {
			err = os.WriteFile(restored, []byte(content), 0o600)
			require.NoError(t, err)
			break
		}
	}
	require.Equal(t, "bravo-version", string(mustRead(t, restored)), "backup bytes must restore exactly to a separate disposable target")
}

func TestResyncRejectsUnusableBackupTargetBeforeChangingRoots(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	leftRoot, rightRoot := filepath.Join(t.TempDir(), "left"), filepath.Join(t.TempDir(), "right")
	workDir, backup1 := t.TempDir(), filepath.Join(t.TempDir(), "backup1")
	backup2File := filepath.Join(t.TempDir(), "backup-target-is-a-file")
	for _, root := range []string{leftRoot, rightRoot, backup1} {
		require.NoError(t, os.MkdirAll(root, 0o700))
	}
	require.NoError(t, os.WriteFile(backup2File, []byte("do-not-touch"), 0o600))
	leftFile, rightFile := filepath.Join(leftRoot, "conflict.txt"), filepath.Join(rightRoot, "conflict.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("Path1-version"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("Path2-version"), 0o600))
	leftTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	rightTime := leftTime.Add(time.Hour)
	require.NoError(t, os.Chtimes(leftFile, leftTime, leftTime))
	require.NoError(t, os.Chtimes(rightFile, rightTime, rightTime))
	left, err := local.NewFs(ctx, "local", leftRoot, configmap.Simple{})
	require.NoError(t, err)
	right, err := local.NewFs(ctx, "local", rightRoot, configmap.Simple{})
	require.NoError(t, err)
	left = stateInspectionTestFs{Fs: left, name: "local", root: "left"}
	right = stateInspectionTestFs{Fs: right, name: "local", root: "right"}

	err = Bisync(ctx, left, right, &Options{
		Workdir:        workDir,
		Resync:         true,
		ResyncMode:     PreferPath1,
		MaxDeleteCount: DefaultMaxDeleteCount,
		BackupDir1:     backup1,
		BackupDir2:     backup2File,
	})
	require.ErrorIs(t, err, ErrBisyncAborted)
	require.Equal(t, "Path1-version", string(mustRead(t, leftFile)))
	require.Equal(t, "Path2-version", string(mustRead(t, rightFile)))
	require.Equal(t, []byte("do-not-touch"), mustRead(t, backup2File))
}

func TestResyncInitializesEmptyAndPopulatedRoots(t *testing.T) {
	cases := []struct {
		name       string
		leftBytes  string
		rightBytes string
	}{
		{name: "both empty"},
		{name: "Path1 empty", rightBytes: "from-path2"},
		{name: "Path2 empty", leftBytes: "from-path1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := fs.AddConfig(context.Background())
			leftRoot, rightRoot := filepath.Join(t.TempDir(), "left"), filepath.Join(t.TempDir(), "right")
			workDir := t.TempDir()
			for _, root := range []string{leftRoot, rightRoot} {
				require.NoError(t, os.MkdirAll(root, 0o700))
			}
			if tc.leftBytes != "" {
				require.NoError(t, os.WriteFile(filepath.Join(leftRoot, "item.txt"), []byte(tc.leftBytes), 0o600))
			}
			if tc.rightBytes != "" {
				require.NoError(t, os.WriteFile(filepath.Join(rightRoot, "item.txt"), []byte(tc.rightBytes), 0o600))
			}
			left, err := local.NewFs(ctx, "local", leftRoot, configmap.Simple{})
			require.NoError(t, err)
			right, err := local.NewFs(ctx, "local", rightRoot, configmap.Simple{})
			require.NoError(t, err)
			left = stateInspectionTestFs{Fs: left, name: "local", root: "left"}
			right = stateInspectionTestFs{Fs: right, name: "local", root: "right"}

			err = Bisync(ctx, left, right, &Options{
				Workdir:        workDir,
				Resync:         true,
				ResyncMode:     PreferPath1,
				MaxDeleteCount: DefaultMaxDeleteCount,
			})
			require.NoError(t, err)
			want := tc.leftBytes
			if want == "" {
				want = tc.rightBytes
			}
			if want == "" {
				_, err = os.Stat(filepath.Join(leftRoot, "item.txt"))
				require.True(t, errors.Is(err, os.ErrNotExist))
				_, err = os.Stat(filepath.Join(rightRoot, "item.txt"))
				require.True(t, errors.Is(err, os.ErrNotExist))
			} else {
				require.Equal(t, want, string(mustRead(t, filepath.Join(leftRoot, "item.txt"))))
				require.Equal(t, want, string(mustRead(t, filepath.Join(rightRoot, "item.txt"))))
			}
		})
	}
}

func TestResyncPreservesFileAndDirectoryCollisionVersions(t *testing.T) {
	ctx, _ := fs.AddConfig(context.Background())
	leftRoot, rightRoot := filepath.Join(t.TempDir(), "left"), filepath.Join(t.TempDir(), "right")
	workDir, backup1, backup2 := t.TempDir(), filepath.Join(t.TempDir(), "backup1"), filepath.Join(t.TempDir(), "backup2")
	for _, root := range []string{leftRoot, rightRoot, backup1, backup2} {
		require.NoError(t, os.MkdirAll(root, 0o700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(leftRoot, "node"), []byte("file-version"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(rightRoot, "node"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(rightRoot, "node", "child.txt"), []byte("directory-version"), 0o600))
	left, err := local.NewFs(ctx, "local", leftRoot, configmap.Simple{})
	require.NoError(t, err)
	right, err := local.NewFs(ctx, "local", rightRoot, configmap.Simple{})
	require.NoError(t, err)
	left = stateInspectionTestFs{Fs: left, name: "local", root: "left"}
	right = stateInspectionTestFs{Fs: right, name: "local", root: "right"}

	err = Bisync(ctx, left, right, &Options{
		Workdir:        workDir,
		Resync:         true,
		ResyncMode:     PreferPath1,
		MaxDeleteCount: DefaultMaxDeleteCount,
		BackupDir1:     backup1,
		BackupDir2:     backup2,
	})
	require.ErrorIs(t, err, ErrBisyncAborted, "file/directory collisions must block initialization")
	require.Equal(t, "file-version", string(mustRead(t, filepath.Join(leftRoot, "node"))))
	require.Equal(t, "directory-version", string(mustRead(t, filepath.Join(rightRoot, "node", "child.txt"))))
	inspection := InspectState(ctx, left, right, &Options{Workdir: workDir})
	require.Equal(t, StateInterrupted, inspection.Status)
	require.False(t, inspection.RecoveryListingsValid,
		"a failed first initialization has no prior listings to recover automatically")
}

func newInspectTestFilesystems(t *testing.T, ctx context.Context) (fs.Fs, fs.Fs) {
	t.Helper()
	left, right := filepath.Join(t.TempDir(), "left"), filepath.Join(t.TempDir(), "right")
	require.NoError(t, os.MkdirAll(left, 0o700))
	require.NoError(t, os.MkdirAll(right, 0o700))
	fs1, err := local.NewFs(ctx, "local", left, configmap.Simple{})
	require.NoError(t, err)
	fs2, err := local.NewFs(ctx, "local", right, configmap.Simple{})
	require.NoError(t, err)
	return stateInspectionTestFs{Fs: fs1, name: "local", root: "left"},
		stateInspectionTestFs{Fs: fs2, name: "local", root: "right"}
}

type stateInspectionTestFs struct {
	fs.Fs
	name string
	root string
}

func (f stateInspectionTestFs) Name() string { return f.name }
func (f stateInspectionTestFs) Root() string { return f.root }

func writeInspectTestLock(t *testing.T, base, workDir, state string) {
	t.Helper()
	metadata := bisyncLockMetadata{
		Version:     lockFileVersion,
		State:       state,
		OwnerToken:  "0123456789abcdef0123456789abcdef",
		PID:         "test",
		TimeRenewed: time.Now().UTC(),
	}
	require.NoError(t, writeLockMetadata(filepath.Join(workDir, base+".lck"), metadata, true))
}

func mustRead(t *testing.T, file string) []byte {
	t.Helper()
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	return data
}
