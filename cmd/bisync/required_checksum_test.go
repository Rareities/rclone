package bisync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/require"
)

type checksumTestFS struct {
	*mockfs.Fs
	mutations int
}

func (f *checksumTestFS) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, opts ...fs.OpenOption) (fs.Object, error) {
	f.mutations++
	return nil, mockfs.ErrNotImplemented
}

type missingChecksumObject struct{ *mockobject.ContentMockObject }

func (o missingChecksumObject) Hash(context.Context, hash.Type) (string, error) { return "", nil }

func checksumTestRun(t *testing.T) (*bisyncRun, context.Context, *checksumTestFS, *checksumTestFS) {
	t.Helper()
	ctx, ci := fs.AddConfig(context.Background())
	ci.CheckSum = true
	newFS := func(name string) *checksumTestFS {
		f, err := mockfs.NewFs(ctx, name, "/"+name, nil)
		require.NoError(t, err)
		m := f.(*mockfs.Fs)
		m.SetHashes(hash.NewHashSet(hash.SHA1))
		return &checksumTestFS{Fs: m}
	}
	f1, f2 := newFS("side1"), newFS("side2")
	dir := t.TempDir()
	b := &bisyncRun{
		fs1: f1, fs2: f2,
		opt:         &Options{RequireChecksum: true, CompareFlag: "size,checksum", Resync: true},
		newListing1: filepath.Join(dir, "one.lst-new"),
		newListing2: filepath.Join(dir, "two.lst-new"),
		aliases:     bilib.AliasMap{},
	}
	require.NoError(t, b.setCompareDefaults(ctx))
	return b, ctx, f1, f2
}

func TestRequireChecksumResyncPreflight(t *testing.T) {
	for _, test := range []struct {
		name        string
		content     []byte
		unknownSize bool
		missingHash bool
	}{
		{"missing hash on populated file", []byte("changed"), false, true},
		{"missing hash on empty file", nil, false, true},
		{"unknown plaintext size", []byte("data"), true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, ctx, f1, f2 := checksumTestRun(t)
			o := mockobject.New("file").WithContent(test.content, mockobject.SeekModeNone)
			o.SetUnknownSize(test.unknownSize)
			if test.missingHash {
				f2.AddObject(missingChecksumObject{o})
			} else {
				f2.AddObject(o)
			}
			err := b.resync(ctx)
			require.ErrorContains(t, err, "--require-checksum")
			require.Zero(t, f1.mutations+f2.mutations, "metadata rejection must precede every transfer")
		})
	}
}

func TestRequireChecksumAcceptsEmptyFileWithHash(t *testing.T) {
	b, ctx, f1, _ := checksumTestRun(t)
	f1.AddObject(mockobject.New("empty").WithContent(nil, mockobject.SeekModeNone))
	ls1, _, err := b.makeMarchListing(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), ls1.getSize("empty"))
	require.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", ls1.getHash("empty"))
}

func TestRequireChecksumRejectsNoCommonHash(t *testing.T) {
	b, ctx, _, f2 := checksumTestRun(t)
	f2.SetHashes(hash.NewHashSet(hash.MD5))
	require.ErrorContains(t, b.setCompareDefaults(ctx), "common native checksum")
}

func TestRequireChecksumRejectsUnsafeOptions(t *testing.T) {
	for _, option := range []string{"ignore-checksum", "ignore-listing-checksum", "no-slow-hash", "slow-hash-sync-only", "download-hash"} {
		t.Run(option, func(t *testing.T) {
			b, ctx, _, _ := checksumTestRun(t)
			switch option {
			case "ignore-checksum":
				fs.GetConfig(ctx).IgnoreChecksum = true
			case "ignore-listing-checksum":
				b.opt.IgnoreListingChecksum = true
			case "no-slow-hash":
				b.opt.Compare.NoSlowHash = true
			case "slow-hash-sync-only":
				b.opt.Compare.SlowHashSyncOnly = true
			case "download-hash":
				b.opt.Compare.DownloadHash = true
			}
			require.ErrorContains(t, b.setCompareDefaults(ctx), "--require-checksum")
		})
	}
}

func TestRequireChecksumRejectsDamagedBaselines(t *testing.T) {
	b, _, _, _ := checksumTestRun(t)
	file := filepath.Join(t.TempDir(), "baseline.lst")
	ls := newFileList()
	ls.hash = hash.SHA1
	ls.put("file", 0, time.Time{}, "da39a3ee5e6b4b0d3255bfef95601890afd80709", "-", "-")
	require.NoError(t, ls.save(file))
	_, err := b.loadListing(file)
	require.NoError(t, err)
	valid, err := os.ReadFile(file)
	require.NoError(t, err)
	for _, contents := range [][]byte{append(append([]byte{}, valid...), []byte("broken row\n")...), valid[:len(valid)-1]} {
		require.NoError(t, os.WriteFile(file, contents, 0600))
		_, err := b.loadListing(file)
		require.Error(t, err)
	}
}
