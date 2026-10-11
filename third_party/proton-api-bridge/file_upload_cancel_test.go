package proton_api_bridge

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"golang.org/x/sync/semaphore"
)

func TestBlockUploadsCancelledWhileWaitingForPermit(t *testing.T) {
	permits := semaphore.NewWeighted(1)
	if err := permits.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	err := runBlockUploads(ctx, permits, 8, func(context.Context, int) error {
		calls.Add(1)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquisition: got %v, want context.Canceled", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("upload proceeded after cancelled acquisition: %d calls", calls.Load())
	}
	permits.Release(1) // catches releasing a permit never acquired by an upload worker
	if !permits.TryAcquire(1) {
		t.Fatal("permit was leaked by a cancelled worker")
	}
	permits.Release(1)
}

func TestBlockUploadsJoinWorkersAfterFailure(t *testing.T) {
	const count = 12
	permits := semaphore.NewWeighted(count)
	failed := errors.New("block upload rejected")
	var completed atomic.Int32
	var started atomic.Int32
	allStarted := make(chan struct{})
	err := runBlockUploads(context.Background(), permits, count, func(ctx context.Context, index int) error {
		defer completed.Add(1)
		if started.Add(1) == count {
			close(allStarted)
		}
		<-allStarted
		if index == 0 {
			return failed
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, failed) {
		t.Fatalf("first upload failure lost: got %v, want %v", err, failed)
	}
	if completed.Load() != count {
		t.Fatalf("returned with running upload workers: completed %d of %d", completed.Load(), count)
	}
	if !permits.TryAcquire(count) {
		t.Fatal("some workers retained their upload semaphore permits")
	}
	permits.Release(count)
}

func TestBlockUploadsSuccessfulBatch(t *testing.T) {
	permits := semaphore.NewWeighted(2)
	var completed atomic.Int32
	err := runBlockUploads(context.Background(), permits, 9, func(context.Context, int) error {
		completed.Add(1)
		return nil
	})
	if err != nil || completed.Load() != 9 || !permits.TryAcquire(2) {
		t.Fatalf("successful batch did not complete and release permits: completed %d, error %v", completed.Load(), err)
	}
	permits.Release(2)
}
