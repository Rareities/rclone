package bilib

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/require"
)

func TestSessionNameLongPairFitsFilesystemComponent(t *testing.T) {
	ctx := context.Background()
	base := strings.Repeat("long-name-", 22)
	first, err := mockfs.NewFs(ctx, "one", base+"first", nil)
	require.NoError(t, err)
	second, err := mockfs.NewFs(ctx, "two", base+"second", nil)
	require.NoError(t, err)
	another, err := mockfs.NewFs(ctx, "two", base+"third", nil)
	require.NoError(t, err)

	name := SessionName(first, second)
	require.LessOrEqual(t, len(name), 100, "listing suffixes must fit a Windows component")
	require.NotEqual(t, name, SessionName(first, another), "different long pairs require distinct state")
	require.NotEqual(t, name, SessionName(second, first), "reversed order must use separate Path1/Path2 listings")
	require.Equal(t, name, filepath.Base(BasePath(ctx, t.TempDir(), first, second)))
}

func TestSessionNameKeepsExistingShortPair(t *testing.T) {
	ctx := context.Background()
	first, err := mockfs.NewFs(ctx, "one", "short", nil)
	require.NoError(t, err)
	second, err := mockfs.NewFs(ctx, "two", "other", nil)
	require.NoError(t, err)
	require.Equal(t,
		StripHexString(CanonicalPath(FsPath(first)))+".."+StripHexString(CanonicalPath(FsPath(second))),
		SessionName(first, second))
}

func TestLongPairRetainsExistingLegacyBaseline(t *testing.T) {
	ctx := context.Background()
	first, err := mockfs.NewFs(ctx, "one", strings.Repeat("a", 103), nil)
	require.NoError(t, err)
	second, err := mockfs.NewFs(ctx, "two", strings.Repeat("b", 103), nil)
	require.NoError(t, err)
	workDir := t.TempDir()
	legacy := filepath.Join(workDir, legacySessionName(first, second))
	require.NotEqual(t, legacy, filepath.Join(workDir, SessionName(first, second)))
	require.NoError(t, os.WriteFile(legacy+".path1.lst", []byte("old listing"), 0600))
	require.Equal(t, legacy, BasePath(ctx, workDir, first, second))
}
