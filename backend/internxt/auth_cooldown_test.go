package internxt

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthCooldownAllowsRecoveryAfterRepeatedFailures(t *testing.T) {
	f := &Fs{}
	transient := errors.New("temporary authentication service outage")
	attempts := 0
	authorize := func(context.Context) error {
		attempts++
		if attempts <= 7 {
			return transient
		}
		return nil
	}
	for i := 0; i < 7; i++ {
		f.nextAuthAllowed = time.Time{} // simulate waiting for the bounded cooldown
		if err := f.reAuthorizeWith(context.Background(), authorize); !errors.Is(err, transient) {
			t.Fatalf("attempt %d: got %v, want transient failure", i+1, err)
		}
		if !f.nextAuthAllowed.After(time.Now()) {
			t.Fatalf("attempt %d did not schedule the next retry", i+1)
		}
	}
	if f.authFailCount != 4 {
		t.Fatalf("retry history should saturate at last cooldown step, got %d", f.authFailCount)
	}
	f.nextAuthAllowed = time.Time{}
	if err := f.reAuthorizeWith(context.Background(), authorize); err != nil {
		t.Fatalf("recovered service was never retried: %v", err)
	}
	if attempts != 8 || f.authFailCount != 0 || !f.nextAuthAllowed.IsZero() {
		t.Fatalf("successful probe did not reset cooldown: attempts=%d failures=%d next=%v", attempts, f.authFailCount, f.nextAuthAllowed)
	}
}

func TestAuthCooldownAvoidsConcurrentRetryStorm(t *testing.T) {
	f := &Fs{}
	failure := errors.New("service unavailable")
	var attempts atomic.Int32
	authorize := func(context.Context) error {
		attempts.Add(1)
		return failure
	}
	const callers = 20
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = f.reAuthorizeWith(context.Background(), authorize)
		}()
	}
	wg.Wait()
	if attempts.Load() != 1 {
		t.Fatalf("concurrent 401s caused %d refresh attempts, want 1", attempts.Load())
	}
}
