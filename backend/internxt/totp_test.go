package internxt

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	internxtauth "github.com/internxt/rclone-adapter/auth"
	sdkerrors "github.com/internxt/rclone-adapter/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/require"
)

const testTOTPSecret = "JBSWY3DPEHPK3PXP"

func TestInternxtTOTPSecretOptionIsSensitivePassword(t *testing.T) {
	regInfo, err := fs.Find("internxt")
	require.NoError(t, err)
	var option *fs.Option
	for i := range regInfo.Options {
		if regInfo.Options[i].Name == "totp_secret" {
			option = &regInfo.Options[i]
			break
		}
	}
	require.NotNil(t, option)
	require.True(t, option.IsPassword)
	require.True(t, option.Sensitive)
	require.True(t, option.Advanced)
}

func TestRevealTOTPSecretSupportsCurrentAndLegacyValues(t *testing.T) {
	require.Equal(t, testTOTPSecret, revealTOTPSecret(testTOTPSecret))
	require.Equal(t, testTOTPSecret, revealTOTPSecret(strings.ToLower(testTOTPSecret)))
	require.Equal(t, testTOTPSecret, revealTOTPSecret(obscure.MustObscure(testTOTPSecret)))
}

func TestLoginWithTOTPTimeWindowsUsesAdjacentWindowsAfterAuthRejection(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	wantCurrent, err := generateTOTPCodeAt(testTOTPSecret, now)
	require.NoError(t, err)
	wantPrevious, err := generateTOTPCodeAt(testTOTPSecret, now.Add(-30*time.Second))
	require.NoError(t, err)

	var attempted []string
	wantResponse := &internxtauth.AccessResponse{}
	response, err := loginWithTOTPTimeWindows(testTOTPSecret, now, func(code string) (*internxtauth.AccessResponse, error) {
		attempted = append(attempted, code)
		if code == wantPrevious {
			return wantResponse, nil
		}
		return nil, &sdkerrors.HTTPError{Response: &http.Response{StatusCode: http.StatusUnauthorized}}
	})
	require.NoError(t, err)
	require.Same(t, wantResponse, response)
	require.Equal(t, []string{wantCurrent, wantPrevious}, attempted)
}

func TestLoginWithTOTPTimeWindowsStopsOnRateLimit(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	wantErr := &sdkerrors.HTTPError{Response: &http.Response{StatusCode: http.StatusTooManyRequests}}
	attempts := 0
	_, err := loginWithTOTPTimeWindows(testTOTPSecret, now, func(string) (*internxtauth.AccessResponse, error) {
		attempts++
		return nil, wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, attempts, "429 must not trigger another authentication request")
}

func TestLoginWithTOTPTimeWindowsHasThreeAttemptCeiling(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	attempts := 0
	_, err := loginWithTOTPTimeWindows(testTOTPSecret, now, func(string) (*internxtauth.AccessResponse, error) {
		attempts++
		return nil, &sdkerrors.HTTPError{Response: &http.Response{StatusCode: http.StatusForbidden}}
	})
	require.ErrorContains(t, err, "current and adjacent time windows")
	require.Equal(t, 3, attempts)
}

func TestLoginWithTOTPTimeWindowsRejectsInvalidSeedBeforeNetwork(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	for _, seed := range []string{"", "not a Base32 seed"} {
		attempts := 0
		_, err := loginWithTOTPTimeWindows(seed, now, func(string) (*internxtauth.AccessResponse, error) {
			attempts++
			return nil, errors.New("must not call authentication")
		})
		require.Error(t, err)
		require.Zero(t, attempts)
	}
}

func TestLoginWithTOTPTimeWindowsRejectsEmptyResponse(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	_, err := loginWithTOTPTimeWindows(testTOTPSecret, now, func(string) (*internxtauth.AccessResponse, error) {
		return nil, nil
	})
	require.ErrorContains(t, err, "empty response")
}
