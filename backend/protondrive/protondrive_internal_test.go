package protondrive

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	protonDriveAPI "github.com/rclone/Proton-API-Bridge"
	protonCommon "github.com/rclone/Proton-API-Bridge/common"
	"github.com/rclone/go-proton-api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/lib/dircache"
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

func TestProtonAuthHandlersStayBoundToTheirConfigMap(t *testing.T) {
	firstMapper := configmap.Simple{}
	secondMapper := configmap.Simple{}

	// Each Proton client retains its callback receiver. A later filesystem must
	// not redirect an earlier client's refresh or deauth callback.
	firstAuthState := &protonAuthState{mapper: firstMapper, saltedKeyPass: "salt-first"}
	firstAuthState.set("old-uid-first", "old-access-first", "old-refresh-first", "salt-first")
	firstAuthHandler, firstDeAuthHandler := firstAuthState.authHandler, firstAuthState.deAuthHandler
	secondAuthState := &protonAuthState{mapper: secondMapper, saltedKeyPass: "salt-second"}
	secondAuthState.set("old-uid-second", "old-access-second", "old-refresh-second", "salt-second")

	firstAuthHandler(proton.Auth{
		UID:          "new-uid-first",
		AccessToken:  "new-access-first",
		RefreshToken: "new-refresh-first",
	})

	assert.Equal(t, "new-uid-first", firstMapper[clientUIDKey])
	assert.Equal(t, "new-access-first", firstMapper[clientAccessTokenKey])
	assert.Equal(t, "new-refresh-first", firstMapper[clientRefreshTokenKey])
	assert.Equal(t, "salt-first", firstMapper[clientSaltedKeyPassKey])
	assert.Equal(t, "old-uid-second", secondMapper[clientUIDKey])
	assert.Equal(t, "old-access-second", secondMapper[clientAccessTokenKey])
	assert.Equal(t, "old-refresh-second", secondMapper[clientRefreshTokenKey])
	assert.Equal(t, "salt-second", secondMapper[clientSaltedKeyPassKey])

	firstDeAuthHandler()
	assert.Equal(t, "", firstMapper[clientUIDKey])
	assert.Equal(t, "", firstMapper[clientAccessTokenKey])
	assert.Equal(t, "", firstMapper[clientRefreshTokenKey])
	assert.Equal(t, "", firstMapper[clientSaltedKeyPassKey])
	assert.Equal(t, "old-uid-second", secondMapper[clientUIDKey])
	assert.Equal(t, "old-access-second", secondMapper[clientAccessTokenKey])
	assert.Equal(t, "old-refresh-second", secondMapper[clientRefreshTokenKey])
	assert.Equal(t, "salt-second", secondMapper[clientSaltedKeyPassKey])
}

func TestSearchByNameWithFallbackFindsNewStandardVaultName(t *testing.T) {
	ctx := context.Background()
	want := &proton.Link{LinkID: "standard-vault-link"}
	listCalls := 0
	got, err := searchByNameWithFallback(
		ctx, "folder-id", "notes.md", true, false,
		func(context.Context, string, string, bool, bool) (*proton.Link, error) {
			return nil, nil // The server-side legacy name-hash lookup misses.
		},
		func(_ context.Context, folderID string) ([]*protonDriveAPI.ProtonDirectoryData, error) {
			listCalls++
			assert.Equal(t, "folder-id", folderID)
			return []*protonDriveAPI.ProtonDirectoryData{
				{Name: "notes.md", Link: want, IsFolder: false},
				{Name: "archive", Link: &proton.Link{LinkID: "folder-link"}, IsFolder: true},
			}, nil
		},
	)

	assert.NoError(t, err)
	assert.Same(t, want, got)
	assert.Equal(t, 1, listCalls)
}

func TestSearchByNameWithFallbackDoesNotListOnHashHit(t *testing.T) {
	want := &proton.Link{LinkID: "hash-hit"}
	listCalls := 0
	got, err := searchByNameWithFallback(
		context.Background(), "folder-id", "notes.md", true, false,
		func(context.Context, string, string, bool, bool) (*proton.Link, error) {
			return want, nil
		},
		func(context.Context, string) ([]*protonDriveAPI.ProtonDirectoryData, error) {
			listCalls++
			return nil, nil
		},
	)
	assert.NoError(t, err)
	assert.Same(t, want, got)
	assert.Zero(t, listCalls)
}

func TestSearchByNameWithFallbackHonorsFileAndFolderKinds(t *testing.T) {
	fileLink := &proton.Link{LinkID: "file-link"}
	folderLink := &proton.Link{LinkID: "folder-link"}
	entries := []*protonDriveAPI.ProtonDirectoryData{
		{Name: "item", Link: folderLink, IsFolder: true},
		{Name: "item", Link: fileLink, IsFolder: false},
	}
	search := func(searchFile, searchFolder bool) *proton.Link {
		got, err := searchByNameWithFallback(
			context.Background(), "folder-id", "item", searchFile, searchFolder,
			func(context.Context, string, string, bool, bool) (*proton.Link, error) { return nil, nil },
			func(context.Context, string) ([]*protonDriveAPI.ProtonDirectoryData, error) { return entries, nil },
		)
		assert.NoError(t, err)
		return got
	}
	assert.Same(t, fileLink, search(true, false))
	assert.Same(t, folderLink, search(false, true))
	assert.Same(t, folderLink, search(true, true))
}

func TestSearchByNameWithFallbackPropagatesListingFailure(t *testing.T) {
	wantErr := errors.New("synthetic listing failure")
	_, err := searchByNameWithFallback(
		context.Background(), "folder-id", "notes.md", true, false,
		func(context.Context, string, string, bool, bool) (*proton.Link, error) { return nil, nil },
		func(context.Context, string) ([]*protonDriveAPI.ProtonDirectoryData, error) { return nil, wantErr },
	)
	assert.ErrorIs(t, err, wantErr)
}

func TestSearchByNameWithFallbackDoesNotTreatMalformedMatchAsMissing(t *testing.T) {
	_, err := searchByNameWithFallback(
		context.Background(), "folder-id", "notes.md", true, false,
		func(context.Context, string, string, bool, bool) (*proton.Link, error) { return nil, nil },
		func(context.Context, string) ([]*protonDriveAPI.ProtonDirectoryData, error) {
			return []*protonDriveAPI.ProtonDirectoryData{{Name: "notes.md", IsFolder: false}}, nil
		},
	)
	assert.Error(t, err, "a matching but incomplete provider entry must not look like a missing object")
}

func TestFlushMoveCachesInvalidatesSourceAndDestinationSubtrees(t *testing.T) {
	sourceCache := dircache.New("", "source-root", nil)
	destinationCache := dircache.New("", "destination-root", nil)
	sourceCache.Put("old/note", "source-note")
	sourceCache.Put("old/note/child", "source-child")
	destinationCache.Put("new/note", "destination-note")
	destinationCache.Put("new/note/child", "destination-child")

	flushMoveCaches(sourceCache, destinationCache, "old/note", "new/note")

	_, sourceFound := sourceCache.Get("old/note")
	_, sourceChildFound := sourceCache.Get("old/note/child")
	_, destinationFound := destinationCache.Get("new/note")
	_, destinationChildFound := destinationCache.Get("new/note/child")
	assert.False(t, sourceFound)
	assert.False(t, sourceChildFound)
	assert.False(t, destinationFound)
	assert.False(t, destinationChildFound)
}

func TestReusableLoginFailurePreservesSavedCredentials(t *testing.T) {
	mapper := configmap.Simple{}
	setConfigMap(mapper, "saved-uid", "saved-access", "saved-refresh", "saved-salt")
	factoryCalls := 0
	_, err := newProtonDriveWithConstructor(
		context.Background(),
		&Fs{ci: &fs.ConfigInfo{}},
		&Options{Username: "user@example.test", Password: "synthetic-password"},
		mapper,
		func(_ context.Context, config *protonCommon.Config, _ proton.AuthHandler, _ proton.Handler) (*protonDriveAPI.ProtonDrive, *protonCommon.ProtonDriveCredential, error) {
			factoryCalls++
			if factoryCalls == 1 {
				assert.True(t, config.UseReusableLogin)
			}
			return nil, nil, errors.New("synthetic transient network failure")
		},
	)

	assert.Error(t, err)
	assert.Equal(t, 2, factoryCalls, "cached login should retain the existing password-login fallback")
	uid, access, refresh, salt, ok := getConfigMap(mapper)
	assert.True(t, ok)
	assert.Equal(t, "saved-uid", uid)
	assert.Equal(t, "saved-access", access)
	assert.Equal(t, "saved-refresh", refresh)
	assert.Equal(t, "saved-salt", salt)
}
