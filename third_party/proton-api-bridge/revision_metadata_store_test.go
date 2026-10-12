package proton_api_bridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/go-proton-api"
)

func TestRevisionMetadataStoreCoalescesAndReusesConcurrentReads(t *testing.T) {
	var store revisionMetadataStore
	var calls atomic.Int32
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := store.load(context.Background(), "key", func() (proton.RevisionMetadata, error) {
				calls.Add(1)
				<-gate
				return proton.RevisionMetadata{ID: "revision", XAttr: "signed metadata"}, nil
			})
			if err != nil || v.ID != "revision" {
				t.Errorf("read failed: %+v %v", v, err)
			}
		}()
	}
	close(gate)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("same revision fetched %d times", calls.Load())
	}
}

func TestRevisionMetadataStoreLimitsRequestsAndCancelsWaitingOwner(t *testing.T) {
	var store revisionMetadataStore
	var active, maxActive atomic.Int32
	gate := make(chan struct{})
	started := make(chan struct{}, 12)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.load(context.Background(), fmt.Sprint(i), func() (proton.RevisionMetadata, error) {
				n := active.Add(1)
				defer active.Add(-1)
				for {
					old := maxActive.Load()
					if n <= old || maxActive.CompareAndSwap(old, n) {
						break
					}
				}
				started <- struct{}{}
				<-gate
				return proton.RevisionMetadata{ID: fmt.Sprint(i), XAttr: "signed metadata"}, nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	for range 4 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("metadata requests did not start")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := store.load(ctx, "cancelled", func() (proton.RevisionMetadata, error) {
			t.Error("cancelled queued request started")
			return proton.RevisionMetadata{}, nil
		})
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued cancellation blocked")
	}
	close(gate)
	wg.Wait()
	if maxActive.Load() > 4 {
		t.Fatalf("request concurrency reached %d", maxActive.Load())
	}
}

func TestRevisionMetadataStoreCancelledLoaderDoesNotCancelOtherReader(t *testing.T) {
	var store revisionMetadataStore
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	owner := make(chan error, 1)
	go func() {
		_, err := store.load(ctx, "shared", func() (proton.RevisionMetadata, error) {
			close(started)
			<-ctx.Done()
			return proton.RevisionMetadata{}, ctx.Err()
		})
		owner <- err
	}()
	<-started
	other := make(chan error, 1)
	go func() {
		v, err := store.load(context.Background(), "shared", func() (proton.RevisionMetadata, error) {
			return proton.RevisionMetadata{ID: "revision", XAttr: "signed metadata"}, nil
		})
		if err == nil && v.ID != "revision" {
			err = fmt.Errorf("wrong revision: %s", v.ID)
		}
		other <- err
	}()
	cancel()
	if err := <-owner; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	select {
	case err := <-other:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("live reader did not recover from cancelled owner")
	}
}

func TestRevisionMetadataStoreIsBoundedAndKeysCannotAlias(t *testing.T) {
	var store revisionMetadataStore
	for i := range 3000 {
		store.put(fmt.Sprint(i), proton.RevisionMetadata{ID: fmt.Sprint(i), XAttr: strings.Repeat("x", 4096)})
	}
	if len(store.entries) > revisionMetadataEntries || store.bytes > revisionMetadataBytes {
		t.Fatal("metadata store exceeded its bounds")
	}
	store.put("oversized", proton.RevisionMetadata{ID: "huge", XAttr: strings.Repeat("x", revisionMetadataBytes+1)})
	if _, ok := store.get("oversized"); ok {
		t.Fatal("oversized metadata retained")
	}
	if revisionMetadataKey("a:b", "c", "d") == revisionMetadataKey("a", "b:c", "d") {
		t.Fatal("different revision identities alias")
	}
	store.clear()
	if len(store.entries) != 0 || store.bytes != 0 {
		t.Fatal("metadata was not cleared")
	}
}
