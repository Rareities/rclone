package internxt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTOTPPlaintextObscuredAndAdjacentWindows(t *testing.T) {
	seed := "JBSWY3DPEHPK3PXP"
	assert.Equal(t, seed, revealTOTPSecret(seed))
	encoded, err := obscure.Obscure(seed)
	require.NoError(t, err)
	assert.Equal(t, seed, revealTOTPSecret(encoded))
	now := time.Unix(1700000010, 0)
	for offset, expected := range map[int64]string{0: "367665", -1: "324550", 1: "870960"} {
		actual, err := generateTOTPCodeAt(seed, now, offset)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	_, err = generateTOTPCodeAt("", now, 0)
	assert.Error(t, err)
}

func TestSoftAuthBreakerRetriesAndResets(t *testing.T) {
	f := &Fs{}
	now := time.Unix(1700000000, 0)
	calls := 0
	failure := func(context.Context) error { calls++; return errors.New("temporary unavailable") }
	require.Error(t, f.reAuthorizeWith(context.Background(), failure, now))
	assert.Equal(t, 1, calls)
	require.ErrorContains(t, f.reAuthorizeWith(context.Background(), failure, now.Add(time.Second)), "blocked")
	assert.Equal(t, 1, calls)
	success := func(context.Context) error { calls++; return nil }
	require.NoError(t, f.reAuthorizeWith(context.Background(), success, f.nextAuthAllowed))
	assert.Zero(t, f.authFailCount)
	assert.True(t, f.nextAuthAllowed.IsZero())
}

func TestSoftAuthBreakerBoundsAttempts(t *testing.T) {
	f := &Fs{}
	now := time.Unix(1700000000, 0)
	calls := 0
	failure := func(context.Context) error { calls++; return errors.New("rejected") }
	for attempt := 0; attempt < 5; attempt++ {
		require.Error(t, f.reAuthorizeWith(context.Background(), failure, now))
		now = f.nextAuthAllowed
	}
	require.ErrorContains(t, f.reAuthorizeWith(context.Background(), failure, now.Add(time.Hour)), "manual re-auth")
	assert.Equal(t, 5, calls)
	for attempt, base := range map[int]time.Duration{1: time.Minute, 2: 5 * time.Minute, 3: 15 * time.Minute, 4: time.Hour, 5: time.Hour} {
		actual := getBackoffDuration(attempt)
		assert.GreaterOrEqual(t, actual, base*9/10)
		assert.LessOrEqual(t, actual, base)
	}
}

func TestResponseTokenPreference(t *testing.T) {
	assert.Equal(t, "new", responseToken("new", "old"))
	assert.Equal(t, "old", responseToken("", "old"))
	assert.Empty(t, responseToken("", ""))
}
