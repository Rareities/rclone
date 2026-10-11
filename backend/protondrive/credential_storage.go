package protondrive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
)

const clientTokenGenerationKey = "client_token_generation"

type credentialStorage struct {
	section    string
	path       string
	guardPath  string
	data       config.Storage
	generation uint64
}

func newCredentialStorage(m configmap.Mapper) (*credentialStorage, error) {
	mapped, ok := m.(*configmap.Map)
	if !ok || mapped.ConfigName() == "" || strings.HasPrefix(mapped.ConfigName(), ":") {
		return nil, nil
	}
	path := config.GetConfigPath()
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	data := config.LoadedData()
	if !data.HasSection(mapped.ConfigName()) {
		return nil, nil
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, err
	}
	return &credentialStorage{section: mapped.ConfigName(), path: canonical, guardPath: canonical + ".proton-auth.guard", data: data}, nil
}

// lock serialises a complete read/check/save transaction, including disk reload.
// A failed reload cannot fall back to stale in-memory auth for another mutation.
func (s *sessionAuth) lock() (func(), error) {
	credentialMu.Lock()
	fail := func(err error) (func(), error) { s.persistErr = err; credentialMu.Unlock(); return nil, err }
	if s.persistErr != nil {
		return fail(s.persistErr)
	}
	if s.persistence == nil {
		return credentialMu.Unlock, nil
	}
	p := s.persistence
	guard := flock.New(p.guardPath, flock.SetPermissions(0600))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	locked, err := guard.TryLockContext(ctx, 10*time.Millisecond)
	cancel()
	if err != nil || !locked {
		_ = guard.Close()
		return fail(errors.New("Proton credential coordination is unavailable"))
	}
	failGuard := func(err error) (func(), error) { _ = guard.Unlock(); _ = guard.Close(); return fail(err) }
	unlock := func() { _ = guard.Unlock(); _ = guard.Close(); credentialMu.Unlock() }
	actualPath, pathErr := filepath.EvalSymlinks(config.GetConfigPath())
	if pathErr == nil {
		actualPath, pathErr = filepath.Abs(actualPath)
	}
	if pathErr != nil || actualPath != p.path {
		return failGuard(errors.New("Proton configuration path changed; reopen the remote"))
	}
	if err = p.data.Load(); err != nil {
		return failGuard(errors.New("Cannot reload Proton credential storage safely"))
	}
	if !p.data.HasSection(p.section) {
		return failGuard(errors.New("The Proton configuration section no longer exists"))
	}
	for _, key := range []string{"username", "password", "mailbox_password", clientUIDKey, clientAccessTokenKey, clientRefreshTokenKey, clientSaltedKeyPassKey} {
		actual, _ := s.m.Get(key)
		stored, _ := p.data.GetValue(p.section, key)
		if actual != stored {
			return failGuard(errors.New("Proton credentials are overridden; edit the stored remote before persisting a session"))
		}
	}
	generation, _ := p.data.GetValue(p.section, clientTokenGenerationKey)
	current := uint64(0)
	if generation != "" {
		current, err = strconv.ParseUint(generation, 10, 64)
		if err != nil {
			return failGuard(errors.New("Invalid Proton credential generation"))
		}
	}
	if current < p.generation {
		return failGuard(errors.New("Proton credential storage moved to an older generation; reconnect explicitly"))
	}
	p.generation = current
	return unlock, nil
}

// One Save publishes the entire tuple using rclone's existing encrypted format.
func (s *sessionAuth) store(value credentials) bool {
	if s.persistence == nil {
		writeCredentials(s.m, value)
		return true
	}
	p := s.persistence
	if p.generation == ^uint64(0) {
		s.persistErr = errors.New("Proton credential generation overflow")
		return false
	}
	values := map[string]string{clientUIDKey: value.uid, clientAccessTokenKey: value.access,
		clientRefreshTokenKey: value.refresh, clientSaltedKeyPassKey: value.salt,
		clientTokenGenerationKey: strconv.FormatUint(p.generation+1, 10)}
	for key, item := range values {
		p.data.SetValue(p.section, key, item)
	}
	if err := p.data.Save(); err != nil {
		_ = p.data.Load()
		s.persistErr = fmt.Errorf("Proton credential persistence failed; operation stopped: %w", err)
		return false
	}
	p.generation++
	return true
}

func (s *sessionAuth) persistenceError() error {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	return s.persistErr
}
