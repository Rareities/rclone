package proton_api_bridge

import (
    "bytes"
    "errors"
    "io"
    "testing"

    "github.com/rclone/go-proton-api"
)

func TestDownloadSkipOffsetWithoutSignedAttributes(t *testing.T) {
    // A buffered plaintext reader stands in for an already decrypted block.
    // Legacy metadata must not silently discard the requested range.
    reader := &FileDownloadReader{data: bytes.NewBufferString("abcdefgh"), revision: &proton.Revision{}}
    // No network blocks are needed when the buffer already contains plaintext.
    if err := reader.skipOffset(3, nil); err != nil { t.Fatal(err) }
    rest, err := io.ReadAll(reader.data)
    if err != nil || string(rest) != "defgh" { t.Fatalf("legacy range = %q, %v", rest, err) }
}

func TestDownloadSkipOffsetRejectsInvalidOffsets(t *testing.T) {
    for _, offset := range []int64{-1, 9} {
        reader := &FileDownloadReader{data: bytes.NewBufferString("abcdefgh"), revision: &proton.Revision{}}
        err := reader.skipOffset(offset, nil)
        if offset < 0 && !errors.Is(err, ErrSeekOffsetAfterSkippingBlocks) { t.Fatalf("negative offset: %v", err) }
        if offset > 8 && !errors.Is(err, io.EOF) { t.Fatalf("out of range: %v", err) }
    }
}

func TestDownloadSkipOffsetZeroWithoutRevision(t *testing.T) {
    reader := &FileDownloadReader{}
    if err := reader.skipOffset(0, nil); err != nil { t.Fatal(err) }
}
