package protondrive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	protonDriveAPI "github.com/rclone/Proton-API-Bridge"
	"github.com/rclone/go-proton-api"

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

func TestListedObjectAttributesPreserveUnknownSize(t *testing.T) {
	f := &Fs{opt: Options{ReportOriginalSize: true}}
	link := &proton.Link{LinkID: "listed", Size: 999, ModifyTime: 100}
	o := f.newObjectWithLinkAndAttrs("file", link, nil)
	if o.Size() != -1 || o.ModTime(context.Background()).Unix() != 100 || o.digests != nil {
		t.Fatalf("missing attrs must preserve unknown plaintext size: %+v", o)
	}
	attrs := &protonDriveAPI.FileSystemAttrs{RevisionID: "verified", Size: 12, ModificationTime: time.Unix(200, 0), Digests: strings.Repeat("a", 40)}
	o = f.newObjectWithLinkAndAttrs("file", link, attrs)
	got, err := o.Hash(context.Background(), hash.SHA1)
	if o.revisionID != "verified" || o.Size() != 12 || o.ModTime(context.Background()).Unix() != 200 || err != nil || got != attrs.Digests {
		t.Fatalf("listed plaintext attributes lost: size=%d hash=%q error=%v", o.Size(), got, err)
	}
}

func TestDownloadRejectsChangedSnapshotWithoutMutation(t *testing.T) {
	for _, change := range []string{"revision", "hash", "size", "modtime", "missing"} {
		t.Run(change, func(t *testing.T) {
			f := &Fs{opt: Options{ReportOriginalSize: true}}
			old := &protonDriveAPI.FileSystemAttrs{RevisionID: "same", Size: 29, ModificationTime: time.Unix(200, 0), Digests: strings.Repeat("a", 40)}
			o := f.newObjectWithLinkAndAttrs(".nomedia", &proton.Link{LinkID: "file"}, old)
			attrs := *old
			got := &attrs
			switch change {
			case "revision":
				attrs.RevisionID = "next"
			case "hash":
				attrs.Digests = strings.Repeat("b", 40)
			case "size":
				attrs.Size++
			case "modtime":
				attrs.ModificationTime = attrs.ModificationTime.Add(time.Second)
			case "missing":
				got = nil
			}
			closeErr := errors.New("reader close failed")
			r := &trackedReader{Reader: bytes.NewReader([]byte("unread")), err: closeErr}
			stream, err := o.finishDownload(r, 999, got, -1)
			require.Nil(t, stream)
			require.ErrorContains(t, err, "metadata changed since listing")
			require.ErrorIs(t, err, closeErr)
			require.Equal(t, 1, r.closed)
			require.Equal(t, int64(0), r.Size()-int64(r.Len()), "no content may be consumed")
			require.Equal(t, old.RevisionID, o.revisionID)
			require.Equal(t, old.Size, o.Size())
			require.Equal(t, old.ModificationTime, o.modTime)
			require.Equal(t, old.Digests, *o.digests)
		})
	}
}

func TestDownloadPreservesMatchingSnapshotAndUnknownMetadata(t *testing.T) {
	f := &Fs{opt: Options{ReportOriginalSize: true}}
	attrs := &protonDriveAPI.FileSystemAttrs{RevisionID: "same", Size: 3, ModificationTime: time.Unix(200, 0), Digests: strings.Repeat("a", 40)}
	o := f.newObjectWithLinkAndAttrs("file", &proton.Link{LinkID: "file"}, attrs)
	upper := *attrs
	upper.Digests = strings.ToUpper(attrs.Digests)
	r := &trackedReader{Reader: bytes.NewReader([]byte("abc"))}
	stream, err := o.finishDownload(r, 999, &upper, 2)
	require.NoError(t, err)
	data, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.Equal(t, "ab", string(data))
	require.NoError(t, stream.Close())
	require.Equal(t, 1, r.closed)
	unknown := f.newObjectWithLinkAndAttrs("unknown", &proton.Link{LinkID: "unknown"}, nil)
	stream, err = unknown.finishDownload(&trackedReader{Reader: bytes.NewReader(nil)}, 999, nil, -1)
	require.NoError(t, err)
	require.NoError(t, stream.Close())
	require.Equal(t, int64(-1), unknown.Size())
}
