package protondrive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/require"
)

type trackedReader struct {
	*bytes.Reader
	closed int
	err    error
}

func (r *trackedReader) Close() error { r.closed++; return r.err }

func TestDownloadClosePropagatesExactlyOnce(t *testing.T) {
	for _, limit := range []int64{-1, 0, 3} {
		r := &trackedReader{Reader: bytes.NewReader([]byte("abcdef"))}
		stream := limitedDownload(r, limit)
		_, err := io.Copy(io.Discard, stream)
		require.NoError(t, err)
		require.NoError(t, stream.Close())
		require.NoError(t, stream.Close())
		require.Equal(t, 1, r.closed)
	}
}

func TestDownloadEarlyCloseRetainsError(t *testing.T) {
	expected := errors.New("cleanup failed")
	r := &trackedReader{Reader: bytes.NewReader([]byte("abcdef")), err: expected}
	stream := limitedDownload(r, 5)
	require.ErrorIs(t, stream.Close(), expected)
	require.ErrorIs(t, stream.Close(), expected)
	require.Equal(t, 1, r.closed)
}

func TestDownloadRangeBoundaryDoesNotDiscardCloseError(t *testing.T) {
	expected := errors.New("range cleanup failed")
	r := &trackedReader{Reader: bytes.NewReader([]byte("abcdef")), err: expected}
	stream := limitedDownload(r, 3)
	data, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.Equal(t, "abc", string(data))
	require.ErrorIs(t, stream.Close(), expected)
	require.Equal(t, 1, r.closed)
}

func TestPlaintextMetadataDoesNotUseCiphertextSize(t *testing.T) {
	o := &Object{fs: &Fs{opt: Options{ReportOriginalSize: true}}, size: 1234}
	require.Equal(t, int64(-1), o.Size())
	zero := int64(0)
	o.originalSize = &zero
	require.Equal(t, int64(0), o.Size())
	o.originalSize = nil
	_, err := o.Open(context.Background(), &fs.RangeOption{Start: 0, End: 20})
	require.ErrorContains(t, err, "plaintext size")
}

func TestSHA1NormalisationAndMalformedMetadata(t *testing.T) {
	valid := "DA39A3EE5E6B4B0D3255BFEF95601890AFD80709"
	o := &Object{digests: &valid}
	value, err := o.Hash(context.Background(), hash.SHA1)
	require.NoError(t, err)
	require.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", value)
	for _, invalid := range []string{"not-a-hash", "012345", "zz39a3ee5e6b4b0d3255bfef95601890afd80709"} {
		_, err := normaliseSHA1(invalid)
		require.Error(t, err)
	}
	value, err = normaliseSHA1("")
	require.NoError(t, err)
	require.Empty(t, value)
}
