package protondrive

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	proton "github.com/rclone/go-proton-api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
)

func TestPersistentAuthProcessHelper(t *testing.T) {
	path := os.Getenv("CLOUDBRIDGE_AUTH_TEST_CONFIG")
	if path == "" {
		return
	}
	if err := config.SetConfigPath(path); err != nil {
		t.Fatal(err)
	}
	configfile.Install()
	m := fs.ConfigMap("protondrive", nil, "test", nil)
	hooks := newConfigAuthHooks(m, "test")
	if hooks.persistErr != nil {
		t.Fatal(hooks.persistErr)
	}
	token := os.Getenv("CLOUDBRIDGE_AUTH_TEST_TOKEN")
	if err := os.WriteFile(path+".ready-"+token, []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path + ".go"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("barrier timed out")
		}
		time.Sleep(time.Millisecond)
	}
	hooks.auth(proton.Auth{UID: "session", AccessToken: "access-" + token, RefreshToken: "refresh-" + token})
	if hooks.persistErr != nil {
		t.Fatal(hooks.persistErr)
	}
}

func TestPersistentTokenTupleRotationAcrossProcesses(t *testing.T) {
	oldPath, oldData := config.GetConfigPath(), config.Data()
	t.Cleanup(func() { _ = config.SetConfigPath(oldPath); config.SetData(oldData) })
	path := filepath.Join(t.TempDir(), "rclone.conf")
	initial := "[test]\ntype = protondrive\nusername = fixture\npassword = fixture\nclient_uid = session\nclient_access_token = access-old\nclient_refresh_token = refresh-old\nclient_salted_key_pass = salted\nclient_token_generation = 0\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	if err := config.SetConfigPath(path); err != nil {
		t.Fatal(err)
	}
	configfile.Install()
	m := fs.ConfigMap("protondrive", nil, "test", nil)
	reader := newConfigAuthHooks(m, "test")
	children := make([]*exec.Cmd, 0, 2)
	for _, token := range []string{"one", "two"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPersistentAuthProcessHelper$")
		cmd.Env = append(os.Environ(), "CLOUDBRIDGE_AUTH_TEST_CONFIG="+path, "CLOUDBRIDGE_AUTH_TEST_TOKEN="+token)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, cmd)
		t.Cleanup(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, one := os.Stat(path + ".ready-one")
		_, two := os.Stat(path + ".ready-two")
		if one == nil && two == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("children did not reach barrier")
		}
		time.Sleep(time.Millisecond)
	}
	if err := os.WriteFile(path+".go", nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range children {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	uid, access, refresh, ok := reader.refresh()
	if !ok || uid != "session" {
		t.Fatal("latest persisted session was not adopted")
	}
	if !((access == "access-one" && refresh == "refresh-one") || (access == "access-two" && refresh == "refresh-two")) {
		t.Fatal("credential tuple was split between concurrent writers")
	}
	if generation := tokenGeneration(m); generation != 1 {
		t.Fatal(fmt.Sprintf("stale writer replaced winner: generation %d", generation))
	}
}

func TestAuthGuardCanonicalizesConfigAliases(t *testing.T) {
	oldPath, oldData := config.GetConfigPath(), config.Data()
	t.Cleanup(func() { _ = config.SetConfigPath(oldPath); config.SetData(oldData) })
	dir := t.TempDir()
	path := filepath.Join(dir, "real.conf")
	if err := os.WriteFile(path, []byte("[test]\ntype=protondrive\n"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.conf")
	if err := os.Symlink(path, alias); err != nil {
		t.Skip("symlinks unavailable")
	}
	makeHooks := func(path string) *configAuthHooks {
		if err := config.SetConfigPath(path); err != nil {
			t.Fatal(err)
		}
		configfile.Install()
		return newConfigAuthHooks(fs.ConfigMap("protondrive", nil, "test", nil), "test")
	}
	first, second := makeHooks(path), makeHooks(alias)
	if first.persistErr != nil || second.persistErr != nil || first.guardPath != second.guardPath || first.guardPath == "" {
		t.Fatal("config aliases did not share one credential guard")
	}
}

func TestRejectedRefreshCannotEraseALateWinner(t *testing.T) {
	m := configmap.Simple{clientUIDKey: "session", clientAccessTokenKey: "old-access", clientRefreshTokenKey: "old-refresh", clientSaltedKeyPassKey: "salted"}
	winner, loser := newConfigAuthHooks(m), newConfigAuthHooks(m)
	loser.refreshRejected()
	winner.auth(proton.Auth{UID: "session", AccessToken: "new-access", RefreshToken: "new-refresh"})
	_, access, refresh, ok := loser.refresh()
	if !ok || access != "new-access" || refresh != "new-refresh" {
		t.Fatal("a rejected refresh erased or blocked the late winner")
	}
}
