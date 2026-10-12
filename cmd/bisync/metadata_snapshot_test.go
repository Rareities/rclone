package bisync

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/require"
)

// metadataSnapshotFS models a backend whose history and detailed download
// metadata differ. Only the canonical case resolves detail before logging.
type metadataSnapshotFS struct {
	fs.Fs
	canonical   bool
	historyHash string
}

func (f *metadataSnapshotFS) Hashes() hash.Set { return hash.NewHashSet(hash.SHA1) }
func (f *metadataSnapshotFS) Features() *fs.Features {
	features := *f.Fs.Features()
	features.ListR = nil
	return &features
}
func (f *metadataSnapshotFS) wrap(o fs.Object) fs.Object {
	if o.Remote() != ".nomedia" {
		return o
	}
	obj := &metadataSnapshotObject{Object: o, owner: f}
	obj.detailed.Store(f.canonical)
	return obj
}
func (f *metadataSnapshotFS) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	return f.wrap(o), nil
}
func (f *metadataSnapshotFS) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	entries, err := f.Fs.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	for i, entry := range entries {
		if o, ok := entry.(fs.Object); ok {
			entries[i] = f.wrap(o)
		}
	}
	return entries, nil
}

type metadataSnapshotObject struct {
	fs.Object
	owner    *metadataSnapshotFS
	detailed atomic.Bool
}

func (o *metadataSnapshotObject) Fs() fs.Info { return o.owner }
func (o *metadataSnapshotObject) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t == hash.SHA1 && !o.detailed.Load() {
		return o.owner.historyHash, nil
	}
	return o.Object.Hash(ctx, t)
}
func (o *metadataSnapshotObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	o.detailed.Store(true)
	return o.Object.Open(ctx, options...)
}

func TestResyncRequiresConsistentMetadataSnapshot(t *testing.T) {
	for _, content := range [][]byte{[]byte{}, []byte("current authenticated content")} {
		for _, canonical := range []bool{false, true} {
			if len(content) == 0 && !canonical {
				continue
			}
			t.Run(fmt.Sprintf("bytes=%d/canonical=%v", len(content), canonical), func(t *testing.T) {
				ctx, ci := fs.AddConfig(context.Background())
				ci.CheckSum = true
				root := t.TempDir()
				left := filepath.Join(root, "left")
				right := filepath.Join(root, "right")
				require.NoError(t, os.MkdirAll(left, 0700))
				require.NoError(t, os.MkdirAll(right, 0700))
				require.NoError(t, os.WriteFile(filepath.Join(left, ".nomedia"), []byte("local differing marker"), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(right, ".nomedia"), content, 0600))
				for i := 0; i < 20; i++ {
					for _, dir := range []string{left, right} {
						require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("same-%02d.txt", i)), []byte("unchanged companion"), 0600))
					}
				}
				one, err := local.NewFs(ctx, "local", left, configmap.Simple{})
				require.NoError(t, err)
				inner, err := local.NewFs(ctx, "proton-model", right, configmap.Simple{})
				require.NoError(t, err)
				two := &metadataSnapshotFS{Fs: inner, canonical: canonical, historyHash: fmt.Sprintf("%x", sha1.Sum([]byte("older authenticated content!!")))}
				opt := &Options{Workdir: filepath.Join(root, "state"), Resync: true, ResyncMode: PreferPath2, CompareFlag: "size,checksum", RequireChecksum: true, CheckSync: CheckSyncTrue}
				err = Bisync(ctx, one, two, opt)
				if !canonical {
					require.ErrorIs(t, err, ErrBisyncAborted)
					return
				}
				require.NoError(t, err)
				got, err := os.ReadFile(filepath.Join(left, ".nomedia"))
				require.NoError(t, err)
				require.Equal(t, content, got)
				remote, err := os.ReadFile(filepath.Join(right, ".nomedia"))
				require.NoError(t, err)
				require.Equal(t, content, remote)
				opt.Resync = false
				opt.ResyncMode = PreferNone
				require.NoError(t, Bisync(ctx, one, two, opt))
				for i := 0; i < 20; i++ {
					got, err := os.ReadFile(filepath.Join(left, fmt.Sprintf("same-%02d.txt", i)))
					require.NoError(t, err)
					require.Equal(t, []byte("unchanged companion"), got)
				}
			})
		}
	}
}
