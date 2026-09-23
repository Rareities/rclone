package bisync

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/terminal"
)

const (
	basicallyforever = fs.Duration(200 * 365 * 24 * time.Hour)
	lockFileVersion  = 2
)

type bisyncLockMetadata struct {
	Version     int       `json:"version"`
	State       string    `json:"state"`
	OwnerToken  string    `json:"ownerToken"`
	PID         string    `json:"pid"`
	TimeRenewed time.Time `json:"timeRenewed"`
	// Active version-2 locks deliberately have no expiry. The OS lock held on
	// the companion .guard file is authoritative; a stale heartbeat must never
	// let another process steal ownership from a paused but live process.
	TimeExpires time.Time `json:"timeExpires,omitempty"`
}

type lockFileOpt struct {
	stopRenewal func()
	guard       *flock.Flock
	ownerToken  string
	data        bisyncLockMetadata
	mu          sync.Mutex
}

// setLockFile acquires a persistent OS lock on a companion file before reading
// or updating metadata. The metadata path itself is never deleted: unlinking a
// locked file would let a concurrent process lock a replacement inode.
func (b *bisyncRun) setLockFile() (err error) {
	b.setLockFileExpiration()
	if b.opt.DryRun {
		b.lockFile = ""
		return nil
	}
	lockPath := b.basePath + ".lck"
	guardPath := lockPath + ".guard"
	guard := flock.New(guardPath,
		flock.SetFlag(os.O_CREATE|os.O_RDWR),
		flock.SetPermissions(bilib.PermSecure),
	)
	acquired, err := guard.TryLock()
	if err != nil {
		return fmt.Errorf("cannot acquire Bisync process lock %s: %w", guardPath, err)
	}
	if !acquired {
		return fmt.Errorf("prior Bisync run is active for this profile (process lock: %s)", guardPath)
	}
	keepGuard := false
	defer func() {
		if !keepGuard {
			_ = guard.Unlock()
		}
	}()

	ownerToken, err := newLockOwnerToken()
	if err != nil {
		return fmt.Errorf("cannot create Bisync lock owner token: %w", err)
	}
	metadata := bisyncLockMetadata{
		Version:     lockFileVersion,
		State:       "active",
		OwnerToken:  ownerToken,
		PID:         strconv.Itoa(os.Getpid()),
		TimeRenewed: time.Now().UTC(),
	}

	previous, readErr := readLockMetadata(lockPath)
	switch {
	case errors.Is(readErr, os.ErrNotExist):
		// Hard-link publication gives older rclone versions either no lock file
		// or the complete non-expiring owner record. They must never observe an
		// empty placeholder and mistake it for an expired lock.
		if err = writeLockMetadata(lockPath, metadata, true); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("Bisync lock appeared during acquisition; retry after verifying its owner: %s", lockPath)
			}
			return fmt.Errorf("cannot publish Bisync lock metadata %s: %w", lockPath, err)
		}
	case readErr != nil:
		return fmt.Errorf("existing Bisync lock metadata is unreadable; verify its owner before recovery (%s): %w", lockPath, readErr)
	case previous.Version != lockFileVersion:
		return fmt.Errorf("legacy or unsupported Bisync lock metadata found; verify the older process has stopped, then recover it explicitly: %s", lockPath)
	case previous.OwnerToken == "" || (previous.State != "active" && previous.State != "released"):
		return fmt.Errorf("existing Bisync lock metadata has no valid owner state; verify before recovery: %s", lockPath)
	default:
		// A version-2 record can be reclaimed only after the OS lock was
		// acquired. That proves its former process released the handle or died.
		// Expiration timestamps are informational and never authorize takeover.
		if err = writeLockMetadata(lockPath, metadata, false); err != nil {
			return fmt.Errorf("cannot replace Bisync lock metadata %s: %w", lockPath, err)
		}
	}

	b.lockFile = lockPath
	b.lockFileOpt.guard = guard
	b.lockFileOpt.ownerToken = ownerToken
	b.lockFileOpt.data = metadata
	b.lockFileOpt.stopRenewal = b.startLockRenewal()
	keepGuard = true
	fs.Debugf(nil, "Bisync process lock acquired: %s", guardPath)
	return nil
}

func newLockOwnerToken() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func readLockMetadata(path string) (bisyncLockMetadata, error) {
	var metadata bisyncLockMetadata
	data, err := os.ReadFile(path)
	if err != nil {
		return metadata, err
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return metadata, err
	}
	return metadata, nil
}

// writeLockMetadata replaces a metadata file atomically while the companion OS
// lock is held. createOnly uses a same-directory hard link for atomic
// no-overwrite publication against older O_EXCL-based rclone versions.
func writeLockMetadata(path string, metadata bisyncLockMetadata, createOnly bool) (err error) {
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, ".bisync-lock-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()
	if err = temp.Chmod(bilib.PermSecure); err != nil {
		_ = temp.Close()
		return err
	}
	encoder := json.NewEncoder(temp)
	if err = encoder.Encode(metadata); err != nil {
		_ = temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if createOnly {
		return os.Link(tempName, path)
	}
	return os.Rename(tempName, path)
}

func (b *bisyncRun) removeLockFile() (err error) {
	if b.lockFile == "" {
		return nil
	}
	if b.lockFileOpt.stopRenewal != nil {
		b.lockFileOpt.stopRenewal()
		b.lockFileOpt.stopRenewal = nil
	}

	b.lockFileOpt.mu.Lock()
	defer b.lockFileOpt.mu.Unlock()
	guard := b.lockFileOpt.guard
	if guard == nil || !guard.Locked() {
		return fmt.Errorf("Bisync process lock ownership was lost before release: %s", b.lockFile)
	}

	current, readErr := readLockMetadata(b.lockFile)
	if readErr != nil {
		err = fmt.Errorf("cannot verify Bisync lock owner before release: %w", readErr)
	} else if current.Version != lockFileVersion || current.OwnerToken != b.lockFileOpt.ownerToken {
		err = errors.New("Bisync lock owner token changed before release")
	} else {
		b.lockFileOpt.data.State = "released"
		b.lockFileOpt.data.TimeRenewed = time.Now().UTC()
		// A released record remains compatible with older rclone versions,
		// which use an expired timestamp to permit the next run.
		b.lockFileOpt.data.TimeExpires = b.lockFileOpt.data.TimeRenewed.Add(-time.Second)
		if writeErr := writeLockMetadata(b.lockFile, b.lockFileOpt.data, false); writeErr != nil {
			err = fmt.Errorf("cannot persist released Bisync lock state: %w", writeErr)
		}
	}
	if unlockErr := guard.Unlock(); unlockErr != nil {
		err = errors.Join(err, fmt.Errorf("cannot release Bisync OS process lock: %w", unlockErr))
	}
	b.lockFileOpt.guard = nil
	b.lockFileOpt.ownerToken = ""
	b.lockFile = ""
	if err != nil {
		fs.Errorf(nil, "unable to release Bisync lock cleanly: %v", err)
	} else {
		fs.Debugf(nil, "Bisync process lock released: %s", guard.Path())
	}
	return err
}

func (b *bisyncRun) setLockFileExpiration() {
	if b.opt.MaxLock > 0 && b.opt.MaxLock < fs.Duration(2*time.Minute) {
		fs.Logf(nil, Color(terminal.YellowFg, "--max-lock cannot be shorter than 2 minutes (unless 0.) Changing --max-lock from %v to %v"), b.opt.MaxLock, 2*time.Minute)
		b.opt.MaxLock = fs.Duration(2 * time.Minute)
	} else if b.opt.MaxLock <= 0 {
		b.opt.MaxLock = basicallyforever
	}
}

// renewLockFile updates diagnostic heartbeat metadata. The OS lock, not this
// timestamp, is authoritative for ownership and cannot be stolen on expiry.
func (b *bisyncRun) renewLockFile() {
	b.lockFileOpt.mu.Lock()
	defer b.lockFileOpt.mu.Unlock()
	if b.lockFile == "" || b.lockFileOpt.guard == nil || !b.lockFileOpt.guard.Locked() {
		return
	}
	current, err := readLockMetadata(b.lockFile)
	if err != nil {
		b.handleErr(b.lockFile, "error reading Bisync lock metadata", err, true, true)
		return
	}
	if current.Version != lockFileVersion || current.OwnerToken != b.lockFileOpt.ownerToken || current.State != "active" {
		b.handleErr(b.lockFile, "Bisync lock owner verification failed", errors.New("owner token or active state changed"), true, true)
		return
	}
	b.lockFileOpt.data.TimeRenewed = time.Now().UTC()
	b.lockFileOpt.data.TimeExpires = time.Time{}
	if err := writeLockMetadata(b.lockFile, b.lockFileOpt.data, false); err != nil {
		b.handleErr(b.lockFile, "error renewing Bisync lock metadata", err, true, true)
	}
}

// StartLockRenewal renews diagnostic metadata every --max-lock minus one minute.
// It never changes ownership: OS process locking handles crashes and stale state.
func (b *bisyncRun) startLockRenewal() func() {
	if b.opt.MaxLock <= 0 || b.opt.MaxLock >= basicallyforever || b.lockFile == "" {
		return func() {}
	}
	stopLockRenewal := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		ticker := time.NewTicker(time.Duration(b.opt.MaxLock) - time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.renewLockFile()
			case <-stopLockRenewal:
				return
			}
		}
	})
	return func() {
		close(stopLockRenewal)
		wg.Wait()
	}
}

func markFailed(file string) {
	failFile := file + "-err"
	if bilib.FileExists(file) {
		_ = os.Remove(failFile)
		_ = os.Rename(file, failFile)
	}
}
