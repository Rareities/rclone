package protondrive

import (
	"context"
	"errors"
	"fmt"
	protonDriveAPI "github.com/rclone/Proton-API-Bridge"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/assert"
)

var protonDriveAppVersionPattern = regexp.MustCompile(`(?i)^external-drive(-[a-z_]+)+@[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?-((stable|beta|RC|alpha)(([.-]?\d+)*)?)?([.-]?dev)?(\+.*)?$`)

func TestProtonDriveAppVersionFromRcloneVersion(t *testing.T) {
	testCases := []struct {
		name          string
		rcloneVersion string
		want          string
	}{
		{
			name:          "release",
			rcloneVersion: "v1.73.5",
			want:          "external-drive-rclone@1.73.5-stable",
		},
		{
			name:          "dev build",
			rcloneVersion: "v1.74.0-DEV",
			want:          "external-drive-rclone@1.74.0-dev",
		},
		{
			name:          "beta build with extra metadata",
			rcloneVersion: "v1.74.0-beta.9519.990f33f2a.fix-protondrive-sdk-2026",
			want:          "external-drive-rclone@1.74.0-beta.9519+990f33f2a.fix-protondrive-sdk-2026",
		},
		{
			name:          "beta build with unsanitized branch name",
			rcloneVersion: "v1.74.0-beta.9519.990f33f2a.fix/protondrive-sdk-2026",
			want:          "external-drive-rclone@1.74.0-beta.9519+990f33f2a.fix-protondrive-sdk-2026",
		},
		{
			name:          "invalid version falls back to stable",
			rcloneVersion: "not-a-version",
			want:          "external-drive-rclone@1.0.0-stable",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := protonDriveAppVersionFromRcloneVersion(testCase.rcloneVersion)

			if got != testCase.want {
				t.Fatalf("unexpected app version: got %q, want %q", got, testCase.want)
			}
			if !protonDriveAppVersionPattern.MatchString(got) {
				t.Fatalf("app version %q does not match Proton pattern", got)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	apiErr := func(status int, code proton.Code) error {
		return &proton.APIError{Status: status, Code: code, Message: "test"}
	}

	for _, tc := range []struct {
		name      string
		ctx       context.Context
		err       error
		wantRetry bool
	}{
		{"nil error", ctx, nil, false},
		{"cancelled context", cancelledCtx, errors.New("some error"), false},
		{"permanent validation error Code=200501 Status=422 (not retried)", ctx, apiErr(422, 200501), false},
		{"transient storage block error Code=200501 Status=500 (retried)", ctx, apiErr(500, 200501), true},
		{"server error Status=500", ctx, apiErr(500, 0), true},
		{"server error Status=502", ctx, apiErr(502, 0), true},
		{"server error Status=504", ctx, apiErr(504, 0), true},
		{"server error Status=503 (handled by SDK, not retried here)", ctx, apiErr(503, 0), false},
		{"rate limit Status=429 (handled by SDK, not retried here)", ctx, apiErr(429, 0), false},
		{"client error Status=400", ctx, apiErr(400, 0), false},
		{"client error Status=404", ctx, apiErr(404, 0), false},
		{"wrapped API error retried via errors.As", ctx, fmt.Errorf("wrapped: %w", apiErr(500, 0)), true},
		{"non-API error falls back to fserrors.ShouldRetry", ctx, errors.New("plain error"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotRetry, _ := shouldRetry(tc.ctx, tc.err)
			assert.Equal(t, tc.wantRetry, gotRetry)
		})
	}
}

func TestAuthHooksIsolationAndStaleSession(t *testing.T) {
	newMapper := func(uid string) configmap.Simple {
		return configmap.Simple{clientUIDKey: uid, clientAccessTokenKey: "access", clientRefreshTokenKey: "refresh", clientSaltedKeyPassKey: "salt-" + uid}
	}
	a, b := newMapper("a"), newMapper("b")
	ha, hb := newConfigAuthHooks(a), newConfigAuthHooks(b)
	stale := newConfigAuthHooks(a)
	var wg sync.WaitGroup
	for _, h := range []*configAuthHooks{ha, hb} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.auth(proton.Auth{UID: h.uid, AccessToken: "rotated", RefreshToken: "rotated-refresh"})
		}()
	}
	wg.Wait()
	stale.deauth()
	stale.auth(proton.Auth{UID: "a", AccessToken: "obsolete", RefreshToken: "obsolete"})
	assert.Equal(t, "rotated", a[clientAccessTokenKey])
	assert.Equal(t, "salt-a", a[clientSaltedKeyPassKey])
	assert.Equal(t, "b", b[clientUIDKey])
	assert.Equal(t, "salt-b", b[clientSaltedKeyPassKey])
	hb.deauth()
	assert.Empty(t, b[clientAccessTokenKey])
	assert.Equal(t, "rotated", a[clientAccessTokenKey])
}

func TestFirstLoginPreservesSaltDuringRefresh(t *testing.T) {
	m := configmap.Simple{}
	h := newConfigAuthHooks(m)
	h.auth(proton.Auth{UID: "a", AccessToken: "early", RefreshToken: "early"})
	assert.Empty(t, m)
	h.firstLogin("a", "access", "refresh", "salt")
	h.auth(proton.Auth{UID: "a", AccessToken: "new-access", RefreshToken: "new-refresh"})
	assert.Equal(t, "salt", m[clientSaltedKeyPassKey])
	assert.Equal(t, "new-access", m[clientAccessTokenKey])
}

func TestCachedLoginRejected(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422, 429, 500, 503} {
		err := fmt.Errorf("wrapped: %w", &proton.APIError{Status: status})
		assert.Equal(t, status == 401, cachedLoginRejected(err), "status %d", status)
	}
	assert.False(t, cachedLoginRejected(context.Canceled))
	assert.False(t, cachedLoginRejected(errors.New("signature verification failed")))
}

func TestFindNamedLink(t *testing.T) {
	entry := func(id, name string, folder bool, state proton.LinkState) *protonDriveAPI.ProtonDirectoryData {
		return &protonDriveAPI.ProtonDirectoryData{Link: &proton.Link{LinkID: id, State: state}, Name: name, IsFolder: folder}
	}
	file := entry("file", "é.txt", false, proton.LinkStateActive)
	entries := []*protonDriveAPI.ProtonDirectoryData{nil, {}, entry("draft", "é.txt", false, proton.LinkStateDraft), entry("folder", "é.txt", true, proton.LinkStateActive), file}
	got, err := findNamedLink(entries, "é.txt", false)
	assert.NoError(t, err)
	assert.Same(t, file.Link, got)
	got, err = findNamedLink(entries, "É.txt", false)
	assert.NoError(t, err)
	assert.Nil(t, got)
	entries = append(entries, entry("other", "é.txt", false, proton.LinkStateActive))
	_, err = findNamedLink(entries, "é.txt", false)
	assert.ErrorContains(t, err, "ambiguous")
}

func TestObjectPlaintextSizeAndHash(t *testing.T) {
	f := &Fs{opt: Options{ReportOriginalSize: true}}
	o := &Object{fs: f, size: 100}
	assert.Equal(t, int64(-1), o.Size(), "ciphertext size must not be reported as plaintext")
	zero := int64(0)
	o.originalSize = &zero
	assert.Zero(t, o.Size())
	digest := strings.Repeat("ABCDEF01", 5)
	o.digests = &digest
	got, err := o.Hash(context.Background(), hash.SHA1)
	assert.NoError(t, err)
	assert.Equal(t, strings.ToLower(digest), got)
	f.opt.ReportOriginalSize = false
	assert.Equal(t, int64(100), o.Size())
}

func TestMissingHashAndRangeMetadata(t *testing.T) {
	o := &Object{fs: &Fs{opt: Options{ReportOriginalSize: true}}, size: 123}
	_, err := o.Open(context.Background(), &fs.RangeOption{Start: 0, End: 2})
	assert.ErrorContains(t, err, "plaintext size")
	for _, digest := range []string{"", "abc", strings.Repeat("z", 40)} {
		o.digests = &digest
		_, err := o.Hash(context.Background(), hash.SHA1)
		assert.ErrorContains(t, err, "unavailable or invalid")
	}
}

func TestDelayedCachedSessionDeauthPreservesFreshLogin(t *testing.T) {
	m := configmap.Simple{clientUIDKey: "old", clientAccessTokenKey: "old-access", clientRefreshTokenKey: "old-refresh", clientSaltedKeyPassKey: "salt"}
	old := newConfigAuthHooks(m)
	old.deauth()
	fresh := newConfigAuthHooks(m)
	fresh.firstLogin("fresh", "fresh-access", "fresh-refresh", "fresh-salt")
	old.deauth()
	old.auth(proton.Auth{UID: "old", AccessToken: "obsolete", RefreshToken: "obsolete"})
	assert.Equal(t, "fresh", m[clientUIDKey])
	assert.Equal(t, "fresh-access", m[clientAccessTokenKey])
	assert.Equal(t, "fresh-salt", m[clientSaltedKeyPassKey])
}
