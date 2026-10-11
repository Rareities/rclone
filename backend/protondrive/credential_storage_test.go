package protondrive

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	proton "github.com/rclone/go-proton-api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/require"
)

const testSessionConfig = "[test]\ntype = protondrive\nusername = fixture\npassword = fixture\nclient_uid = session\nclient_access_token = access-old\nclient_refresh_token = refresh-old\nclient_salted_key_pass = salted\nclient_token_generation = 0\n[other]\ntype = memory\nvalue = untouched\n"

func installSessionFixture(t *testing.T) (string, *configmap.Map) {
	t.Helper()
	oldPath, oldData := config.GetConfigPath(), config.Data()
	t.Cleanup(func() { _ = config.SetConfigPath(oldPath); config.SetData(oldData) })
	path := filepath.Join(t.TempDir(), "rclone.conf")
	require.NoError(t, os.WriteFile(path, []byte(testSessionConfig), 0600))
	require.NoError(t, config.SetConfigPath(path))
	configfile.Install()
	return path, fs.ConfigMap("protondrive", nil, "test", nil)
}

func TestSessionPersistentProcessHelper(t *testing.T) {
	path := os.Getenv("CLOUDBRIDGE_AUTH_TEST_CONFIG")
	if path == "" {
		return
	}
	require.NoError(t, config.SetConfigPath(path))
	configfile.Install()
	hooks := newSessionAuth(fs.ConfigMap("protondrive", nil, "test", nil))
	require.NoError(t, hooks.persistenceError())
	token := os.Getenv("CLOUDBRIDGE_AUTH_TEST_TOKEN")
	require.True(t, token == "one" || token == "two")
	require.NoError(t, os.WriteFile(path+".ready-"+token, nil, 0600))
	deadline := time.Now().Add(8 * time.Second)
	for {
		if _, err := os.Stat(path + ".go"); err == nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "barrier timed out")
		time.Sleep(time.Millisecond)
	}
	hooks.onAuth(proton.Auth{UID: "session", AccessToken: "access-" + token, RefreshToken: "refresh-" + token})
	require.NoError(t, hooks.persistenceError())
}

func TestSessionPersistentTupleIsAtomicAcrossProcesses(t *testing.T) {
	path, m := installSessionFixture(t)
	reader := newSessionAuth(m)
	require.NoError(t, reader.persistenceError())
	var children []*exec.Cmd
	for _, token := range []string{"one", "two"} {
		child := exec.Command(os.Args[0], "-test.run=^TestSessionPersistentProcessHelper$")
		for _, item := range os.Environ() {
			if !strings.HasPrefix(item, "RCLONE_") && !strings.HasPrefix(item, "CLOUDBRIDGE_AUTH_TEST_") {
				child.Env = append(child.Env, item)
			}
		}
		child.Env = append(child.Env, "CLOUDBRIDGE_AUTH_TEST_CONFIG="+path, "CLOUDBRIDGE_AUTH_TEST_TOKEN="+token)
		require.NoError(t, child.Start())
		children = append(children, child)
		t.Cleanup(func() {
			if child.ProcessState == nil {
				_ = child.Process.Kill()
				_ = child.Wait()
			}
		})
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		_, a := os.Stat(path + ".ready-one")
		_, b := os.Stat(path + ".ready-two")
		if a == nil && b == nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "children did not reach barrier")
		time.Sleep(time.Millisecond)
	}
	require.NoError(t, os.WriteFile(path+".go", nil, 0600))
	for _, child := range children {
		require.NoError(t, child.Wait())
	}
	uid, access, refresh, ok := reader.refresh()
	require.True(t, ok)
	require.Equal(t, "session", uid)
	require.True(t, (access == "access-one" && refresh == "refresh-one") || (access == "access-two" && refresh == "refresh-two"), "tuple split between writers")
	generation, _ := m.Get(clientTokenGenerationKey)
	require.Equal(t, "1", generation, "a stale callback must not replace the winner")
	value, _ := config.LoadedData().GetValue("other", "value")
	require.Equal(t, "untouched", value)
}

type failingSessionSave struct{ config.Storage }

func (f failingSessionSave) Save() error { return errors.New("injected persistence failure") }

func TestSessionPersistentSaveFailureIsFatalAndPreservesDiskTuple(t *testing.T) {
	path, m := installSessionFixture(t)
	hooks := newSessionAuth(m)
	require.NoError(t, hooks.persistenceError())
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	hooks.persistence.data = failingSessionSave{hooks.persistence.data}
	hooks.onAuth(proton.Auth{UID: "session", AccessToken: "new-access", RefreshToken: "new-refresh"})
	require.ErrorContains(t, hooks.persistenceError(), "persistence failed")
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, _, _, ok := hooks.refresh()
	require.False(t, ok)
}

func TestSessionPersistentIdentityUsesOriginalSectionAndRejectsOverrides(t *testing.T) {
	_, m := installSessionFixture(t)
	require.Equal(t, "test", m.ConfigName())
	hooks := newSessionAuth(m)
	require.NoError(t, hooks.persistenceError())
	require.Equal(t, "test", hooks.persistence.section)
	overridden := fs.ConfigMap("protondrive", nil, "test", configmap.Simple{"username": "other-account"})
	require.ErrorContains(t, newSessionAuth(overridden).persistenceError(), "overridden")
}

func TestSessionPersistentMissingConfigCannotReuseCachedCredentials(t *testing.T) {
	path, m := installSessionFixture(t)
	hooks := newSessionAuth(m)
	require.NoError(t, hooks.persistenceError())
	require.NoError(t, os.Remove(path))
	_, _, _, ok := hooks.refresh()
	require.False(t, ok)
	require.Error(t, hooks.persistenceError())
	require.NoFileExists(t, path, "a missing config must not be recreated using stale credentials")
}

func TestSessionRejectedRefreshPreservesLateWinner(t *testing.T) {
	m := storedAuth("session", "old-access", "old-refresh", "salted")
	winner, loser := newSessionAuth(m), newSessionAuth(m)
	loser.onRejectedRefresh()
	winner.onAuth(proton.Auth{UID: "session", AccessToken: "new-access", RefreshToken: "new-refresh"})
	_, access, refresh, ok := loser.refresh()
	require.True(t, ok)
	require.Equal(t, "new-access", access)
	require.Equal(t, "new-refresh", refresh)
}
