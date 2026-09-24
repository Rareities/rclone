// Copyright (c) 2026 The rclone authors

package bisync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/require"
)

func TestBisyncBackupDirReuseReplacesExistingPreservedPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		backupDir int
	}{
		{name: "Path1", backupDir: 1},
		{name: "Path2", backupDir: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := fs.AddConfig(context.Background())
			root := t.TempDir()
			leftRoot, rightRoot := filepath.Join(root, "left"), filepath.Join(root, "right")
			workDir := filepath.Join(root, "work")
			backup1, backup2 := filepath.Join(root, "backup1"), filepath.Join(root, "backup2")
			for _, dir := range []string{leftRoot, rightRoot, workDir, backup1, backup2} {
				require.NoError(t, os.MkdirAll(dir, 0o700))
			}

			item := "reused-path.txt"
			accepted := []byte("accepted-endpoint-version")
			previousBackup := []byte("older-version-from-a-previous-run")
			for _, endpoint := range []string{leftRoot, rightRoot} {
				require.NoError(t, os.WriteFile(filepath.Join(endpoint, item), accepted, 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(endpoint, "stable-sentinel.txt"), []byte("stable"), 0o600))
			}
			left, err := local.NewFs(ctx, "local", leftRoot, configmap.Simple{})
			require.NoError(t, err)
			right, err := local.NewFs(ctx, "local", rightRoot, configmap.Simple{})
			require.NoError(t, err)
			left = stateInspectionTestFs{Fs: left, name: "local", root: "left"}
			right = stateInspectionTestFs{Fs: right, name: "local", root: "right"}

			baseOptions := &Options{Workdir: workDir, MaxDeleteCount: DefaultMaxDeleteCount, CompareFlag: "size,checksum"}
			require.NoError(t, Bisync(ctx, left, right, &Options{
				Workdir: workDir, Resync: true, ResyncMode: PreferPath1,
				MaxDeleteCount: DefaultMaxDeleteCount, CompareFlag: "size,checksum",
			}))
			require.NoError(t, Bisync(ctx, left, right, baseOptions), "establish a compatible accepted baseline")

			backupRoot, changedRoot := backup2, leftRoot
			if tc.backupDir == 1 {
				backupRoot, changedRoot = backup1, rightRoot
			}
			require.NoError(t, os.WriteFile(filepath.Join(backupRoot, item), previousBackup, 0o600))
			updated := []byte("modified-endpoint-version")
			require.Len(t, updated, len(accepted), "keep the same content length")
			require.NoError(t, os.WriteFile(filepath.Join(changedRoot, item), updated, 0o600))

			run := *baseOptions
			run.BackupDir1 = backup1
			run.BackupDir2 = backup2
			require.NoError(t, Bisync(ctx, left, right, &run))

			for _, endpoint := range []string{leftRoot, rightRoot} {
				require.Equal(t, updated, mustRead(t, filepath.Join(endpoint, item)))
				require.Equal(t, []byte("stable"), mustRead(t, filepath.Join(endpoint, "stable-sentinel.txt")))
			}
			require.Equal(t, accepted, mustRead(t, filepath.Join(backupRoot, item)),
				"reusing a backup root replaces the older file at the same relative path")
			require.NotEqual(t, previousBackup, mustRead(t, filepath.Join(backupRoot, item)))
		})
	}
}
