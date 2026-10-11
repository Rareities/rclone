package protondrive

import (
	"crypto/sha256"
	"strings"
	"sync"

	"github.com/rclone/go-proton-api"
	"github.com/rclone/rclone/fs/config/configmap"
)

// This mutex protects mapper transactions, never stores an account's credentials.
// It also makes the in-memory Simple mapper safe for concurrent callback tests.
var credentialMu sync.Mutex

type credentials struct {
	uid, access, refresh, salt string
}

func (c credentials) complete() bool {
	return c.uid != "" && c.access != "" && c.refresh != "" && c.salt != ""
}

func readCredentials(m configmap.Mapper) credentials {
	uid, _ := m.Get(clientUIDKey)
	access, _ := m.Get(clientAccessTokenKey)
	refresh, _ := m.Get(clientRefreshTokenKey)
	salt, _ := m.Get(clientSaltedKeyPassKey)
	return credentials{uid, access, refresh, salt}
}

func writeCredentials(m configmap.Mapper, value credentials) {
	m.Set(clientUIDKey, value.uid)
	m.Set(clientAccessTokenKey, value.access)
	m.Set(clientRefreshTokenKey, value.refresh)
	m.Set(clientSaltedKeyPassKey, value.salt)
}

func mapperOwner(m configmap.Mapper) [32]byte {
	values := make([]string, 0, 3)
	for _, name := range []string{"username", "password", "mailbox_password"} {
		value, _ := m.Get(name)
		values = append(values, name+"\x00"+value)
	}
	return sha256.Sum256([]byte(strings.Join(values, "\x00")))
}

// sessionAuth owns callbacks for one filesystem. UID and salted key bind a
// callback to its original login generation, not merely to a remote's name.
type sessionAuth struct {
	m       configmap.Mapper
	owner   [32]byte
	initial credentials
	current credentials
	retired map[[32]byte]struct{}
	pending proton.Auth
}

func newSessionAuth(m configmap.Mapper) *sessionAuth {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	value := readCredentials(m)
	current := value
	if !value.complete() {
		current = credentials{}
	}
	return &sessionAuth{m: m, owner: mapperOwner(m), initial: value, current: current, retired: make(map[[32]byte]struct{})}
}

func (s *sessionAuth) sameOwner() bool { return mapperOwner(s.m) == s.owner }

func (s *sessionAuth) rememberOld(refresh string) {
	if refresh != "" {
		s.retired[sha256.Sum256([]byte(refresh))] = struct{}{}
	}
}

func (s *sessionAuth) onAuth(auth proton.Auth) {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	if !s.sameOwner() || auth.UID == "" || auth.AccessToken == "" || auth.RefreshToken == "" {
		return
	}
	if s.current.salt == "" {
		// A refresh during password login cannot be persisted until the bridge
		// returns the verified salted key. Keep it scoped to this constructor.
		s.pending = auth
		return
	}
	stored := readCredentials(s.m)
	if stored.uid != s.current.uid || stored.salt != s.current.salt || auth.UID != s.current.uid {
		return
	}
	if _, old := s.retired[sha256.Sum256([]byte(auth.RefreshToken))]; old {
		return
	}
	if stored.refresh != s.current.refresh && stored.refresh != auth.RefreshToken {
		return // A newer callback or another client has already committed a rotation.
	}
	if s.current.refresh != auth.RefreshToken {
		s.rememberOld(s.current.refresh)
	}
	s.current = credentials{auth.UID, auth.AccessToken, auth.RefreshToken, stored.salt}
	if stored != s.current {
		writeCredentials(s.m, s.current)
	}
}

func (s *sessionAuth) onDeauth() {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	stored := readCredentials(s.m)
	if s.sameOwner() && s.current.complete() && stored.uid == s.current.uid &&
		stored.refresh == s.current.refresh && stored.salt == s.current.salt {
		writeCredentials(s.m, credentials{})
	}
}

func (s *sessionAuth) refresh() (uid, access, refresh string, ok bool) {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	stored := readCredentials(s.m)
	if !s.sameOwner() || !stored.complete() || stored.uid != s.current.uid || stored.salt != s.current.salt {
		return "", "", "", false
	}
	if _, old := s.retired[sha256.Sum256([]byte(stored.refresh))]; old {
		return "", "", "", false
	}
	if stored.refresh != s.current.refresh {
		s.rememberOld(s.current.refresh)
		s.current = stored
	}
	return stored.uid, stored.access, stored.refresh, true
}

func (s *sessionAuth) commitLogin(uid, access, refresh, salt string) bool {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	value := credentials{uid, access, refresh, salt}
	if !s.sameOwner() || !value.complete() || readCredentials(s.m) != s.initial {
		return false
	}
	if s.pending.UID == uid && s.pending.AccessToken != "" && s.pending.RefreshToken != "" {
		value.access, value.refresh = s.pending.AccessToken, s.pending.RefreshToken
	}
	s.current = value
	s.pending = proton.Auth{}
	writeCredentials(s.m, value)
	return true
}
