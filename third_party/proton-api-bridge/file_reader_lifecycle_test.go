package proton_api_bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestFileDownloadReaderCloseCancelsAndRejectsBufferedData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &FileDownloadReader{
		ctx:    ctx,
		cancel: cancel,
		data:   bytes.NewBufferString("authenticated plaintext"),
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("repeated close: %v", err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("close did not cancel in-flight requests: %v", ctx.Err())
	}
	buffer := make([]byte, 100)
	if n, err := reader.Read(buffer); n != 0 || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("read after close = (%d, %v), want (0, os.ErrClosed)", n, err)
	}
}
