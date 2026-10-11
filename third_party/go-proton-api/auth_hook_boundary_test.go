package proton

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAuthHookRejectsCrossSessionAndSpentTokens(t *testing.T) {
	c := newClient(nil, "session").withAuth("initial-access", "initial-refresh")
	uid, access, refresh := "other-session", "new-access", "new-refresh"
	c.AddAuthRefreshHook(func() (string, string, string, bool) { return uid, access, refresh, true })
	require.False(t, c.adoptNewerAuth())
	require.Equal(t, "session", c.uid)
	uid = "session"
	access = ""
	require.False(t, c.adoptNewerAuth())
	access = "new-access"
	require.True(t, c.adoptNewerAuth())
	require.Equal(t, "new-refresh", c.ref)
	refresh = "initial-refresh"
	require.False(t, c.adoptNewerAuth(), "must not return to an already spent token")
	require.Equal(t, "new-refresh", c.ref)
}

func TestAuthPersistenceFailureStopsRefreshedOperation(t *testing.T) {
	c := newClient(nil, "session").withAuth("old-access", "old-refresh")
	c.AddAuthRefreshHook(func() (string, string, string, bool) { return "session", "new-access", "new-refresh", true })
	failure := errors.New("fixture persistence failure")
	var persisted error
	c.AddAuthHandler(func(Auth) { persisted = failure })
	c.AddAuthPersistenceHook(func() error { return persisted })
	require.ErrorIs(t, c.authRefresh(context.Background()), failure)
	require.Equal(t, "new-refresh", c.ref, "a spent token must not be restored after persistence failure")
}
