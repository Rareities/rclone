package protondrive

import (
	"sync"
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/require"
)

func storedAuth(uid, access, refresh, salt string) configmap.Simple {
	m := configmap.Simple{"username": uid}
	setConfigMap(m, uid, access, refresh, salt)
	return m
}

func TestSessionCallbacksStayWithTheirOwnRemote(t *testing.T) {
	a, b := storedAuth("a", "a1", "ar1", "salt-a"), storedAuth("b", "b1", "br1", "salt-b")
	ha, hb := newSessionAuth(a), newSessionAuth(b)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); ha.onAuth(proton.Auth{UID: "a", AccessToken: "a2", RefreshToken: "ar2"}) }()
	go func() { defer wg.Done(); hb.onAuth(proton.Auth{UID: "b", AccessToken: "b2", RefreshToken: "br2"}) }()
	wg.Wait()
	uid, access, refresh, salt, ok := getConfigMap(a)
	require.True(t, ok)
	require.Equal(t, []string{"a", "a2", "ar2", "salt-a"}, []string{uid, access, refresh, salt})
	uid, access, refresh, salt, ok = getConfigMap(b)
	require.True(t, ok)
	require.Equal(t, []string{"b", "b2", "br2", "salt-b"}, []string{uid, access, refresh, salt})
}

func TestSessionStaleCallbacksDoNotOverwriteOrClearRotation(t *testing.T) {
	m := storedAuth("uid", "a1", "r1", "salt")
	a, b := newSessionAuth(m), newSessionAuth(m)
	a.onAuth(proton.Auth{UID: "uid", AccessToken: "a2", RefreshToken: "r2"})
	b.onDeauth()
	b.onAuth(proton.Auth{UID: "uid", AccessToken: "stale", RefreshToken: "stale"})
	_, access, refresh, _, ok := getConfigMap(m)
	require.True(t, ok)
	require.Equal(t, "a2", access)
	require.Equal(t, "r2", refresh)
	uid, access, refresh, ok := b.refresh()
	require.True(t, ok)
	require.Equal(t, []string{"uid", "a2", "r2"}, []string{uid, access, refresh})
	b.onAuth(proton.Auth{UID: uid, AccessToken: access, RefreshToken: refresh})
	_, _, _, _, ok = getConfigMap(m)
	require.True(t, ok, "adopting a stored rotation must not clear it")
}

func TestSessionCallbacksRejectReconnectedGeneration(t *testing.T) {
	for _, change := range []string{"uid", "salt", "owner"} {
		t.Run(change, func(t *testing.T) {
			m := storedAuth("uid", "a1", "r1", "salt")
			h := newSessionAuth(m)
			switch change {
			case "uid":
				setConfigMap(m, "new-uid", "new-a", "new-r", "salt")
			case "salt":
				setConfigMap(m, "uid", "new-a", "new-r", "new-salt")
			case "owner":
				m.Set("username", "another-user")
			}
			before := m.String()
			h.onAuth(proton.Auth{UID: "uid", AccessToken: "bad", RefreshToken: "bad"})
			h.onDeauth()
			_, _, _, ok := h.refresh()
			require.False(t, ok)
			require.Equal(t, before, m.String())
		})
	}
}

func TestSessionRejectsRollbackAndIncompleteTokens(t *testing.T) {
	m := storedAuth("uid", "a1", "r1", "salt")
	h := newSessionAuth(m)
	h.onAuth(proton.Auth{UID: "uid", AccessToken: "a2", RefreshToken: "r2"})
	h.onAuth(proton.Auth{UID: "uid", AccessToken: "a1", RefreshToken: "r1"})
	h.onAuth(proton.Auth{UID: "uid", AccessToken: "", RefreshToken: "r3"})
	_, access, refresh, _, _ := getConfigMap(m)
	require.Equal(t, "a2", access)
	require.Equal(t, "r2", refresh)
	setConfigMap(m, "uid", "a1", "r1", "salt")
	_, _, _, ok := h.refresh()
	require.False(t, ok, "a previously seen old token cannot be adopted as newer")
}

func TestSessionCurrentRevocationClearsOnlyCurrentSession(t *testing.T) {
	m := storedAuth("uid", "a1", "r1", "salt")
	h := newSessionAuth(m)
	h.onDeauth()
	_, _, _, _, ok := getConfigMap(m)
	require.False(t, ok)
}

func TestSessionFirstLoginDoesNotLoseEarlyRotation(t *testing.T) {
	m := configmap.Simple{"username": "user"}
	h := newSessionAuth(m)
	h.onAuth(proton.Auth{UID: "uid", AccessToken: "a2", RefreshToken: "r2"})
	_, _, _, _, ok := getConfigMap(m)
	require.False(t, ok, "incomplete first-login credentials must not be saved")
	require.True(t, h.commitLogin("uid", "a1", "r1", "salt"))
	_, access, refresh, salt, ok := getConfigMap(m)
	require.True(t, ok)
	require.Equal(t, []string{"a2", "r2", "salt"}, []string{access, refresh, salt})
	require.False(t, h.commitLogin("uid", "a1", "r1", "salt"), "late first-login completion cannot overwrite current tokens")
}
