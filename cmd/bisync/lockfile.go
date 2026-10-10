package bisync

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofrs/flock"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/terminal"
)

const basicallyforever = fs.Duration(200 * 365 * 24 * time.Hour)

type lockFileOpt struct {
	mu          sync.Mutex
	stopRenewal func()
	file        *os.File
	owner       string
	data        struct {
		Owner       string `json:",omitempty"`
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}
}

// lockGuard serializes creation, expiration, renewal, and removal across processes.
// The sidecar is retained so all contenders lock the same inode.
func lockGuard(path string) (*flock.Flock, error) {
	guard := flock.New(path+".guard", flock.SetPermissions(bilib.PermSecure))
	if err := guard.Lock(); err != nil {
		_ = guard.Close()
		return nil, err
	}
	return guard, nil
}

func (b *bisyncRun) setLockFile() (err error) {
	b.lockFile = ""
	b.setLockFileExpiration()
	if b.opt.DryRun {
		return nil
	}
	path := b.basePath + ".lck"
	guard, err := lockGuard(path)
	if err != nil {
		return fmt.Errorf("cannot guard lock file %s: %w", path, err)
	}
	defer func() { _ = guard.Close() }()
	b.lockFile = path
	defer func() {
		if err != nil {
			b.lockFile = ""
		}
	}()
	if _, statErr := os.Stat(path); statErr == nil {
		if !b.lockFileIsExpired() {
			errTip := Color(terminal.MagentaFg, "Tip: this indicates that another bisync run (of these same paths) either is still running or was interrupted before completion. \n")
			errTip += Color(terminal.MagentaFg, "If you're SURE you want to override this safety feature, you can delete the lock file with the following command, then run bisync again: \n")
			errTip += fmt.Sprintf(Color(terminal.HiRedFg, "rclone deletefile \"%s\""), path)
			return fmt.Errorf(Color(terminal.RedFg, "prior lock file found: %s \n")+errTip, Color(terminal.HiYellowFg, path))
		}
		if err = os.Remove(path); err != nil {
			return fmt.Errorf("cannot remove expired lock file: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, bilib.PermSecure)
	if err != nil {
		return fmt.Errorf("cannot create lock file %s: %w", path, err)
	}
	b.lockFileOpt.file = file
	b.lockFileOpt.owner = rand.Text()
	if err = b.writeLockFile(); err != nil {
		_ = file.Close()
		b.lockFileOpt.file = nil
		_ = os.Remove(path)
		return err
	}
	fs.Debugf(nil, "Lock file created: %s", path)
	b.lockFileOpt.stopRenewal = b.startLockRenewal()
	return nil
}

func (b *bisyncRun) ownsLockFile() bool {
	if b.lockFileOpt.file == nil {
		return false
	}
	owned, err := b.lockFileOpt.file.Stat()
	if err != nil {
		return false
	}
	current, err := os.Stat(b.lockFile)
	if err != nil || !os.SameFile(owned, current) {
		return false
	}
	data, err := os.ReadFile(b.lockFile)
	if err != nil {
		return false
	}
	var owner struct{ Owner string }
	return json.Unmarshal(data, &owner) == nil && owner.Owner == b.lockFileOpt.owner
}

func (b *bisyncRun) removeLockFile() (err error) {
	b.lockFileOpt.mu.Lock()
	if b.lockFile == "" {
		b.lockFileOpt.mu.Unlock()
		return nil
	}
	stopRenewal := b.lockFileOpt.stopRenewal
	b.lockFileOpt.mu.Unlock()
	// Renewal needs the same mutex, so join it before taking removal ownership.
	if stopRenewal != nil {
		stopRenewal()
	}
	b.lockFileOpt.mu.Lock()
	defer b.lockFileOpt.mu.Unlock()
	if b.lockFile == "" {
		return nil
	}
	defer func() {
		if b.lockFileOpt.file != nil {
			_ = b.lockFileOpt.file.Close()
			b.lockFileOpt.file = nil
		}
		b.lockFile = ""
	}()
	guard, err := lockGuard(b.lockFile)
	if err != nil {
		return err
	}
	defer func() { _ = guard.Close() }()
	if !b.ownsLockFile() {
		return fmt.Errorf("lock file ownership lost: %s", b.lockFile)
	}
	// Close before unlinking for platforms which disallow removing an open file.
	if err = b.lockFileOpt.file.Close(); err != nil {
		return err
	}
	b.lockFileOpt.file = nil
	err = os.Remove(b.lockFile)
	if err == nil {
		fs.Debugf(nil, "Lock file removed: %s", b.lockFile)
	} else {
		fs.Errorf(nil, "cannot remove lockfile %s: %v", b.lockFile, err)
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

func (b *bisyncRun) writeLockFile() error {
	b.lockFileOpt.data.Owner = b.lockFileOpt.owner
	b.lockFileOpt.data.Session = b.basePath
	b.lockFileOpt.data.PID = strconv.Itoa(os.Getpid())
	b.lockFileOpt.data.TimeRenewed = time.Now()
	b.lockFileOpt.data.TimeExpires = b.lockFileOpt.data.TimeRenewed.Add(time.Duration(b.opt.MaxLock))
	data, err := json.Marshal(b.lockFileOpt.data)
	if err != nil {
		return err
	}
	file := b.lockFileOpt.file
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err = file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err = file.Truncate(int64(len(data) + 1)); err != nil {
		return err
	}
	return file.Sync()
}

func (b *bisyncRun) renewLockFile() {
	b.lockFileOpt.mu.Lock()
	defer b.lockFileOpt.mu.Unlock()
	if b.lockFile == "" {
		return
	}
	guard, err := lockGuard(b.lockFile)
	if err != nil {
		b.handleErr(b.lockFile, "error guarding lock file", err, true, true)
		return
	}
	defer func() { _ = guard.Close() }()
	if !b.ownsLockFile() {
		b.handleErr(b.lockFile, "error renewing lock file", fmt.Errorf("lock file ownership lost"), true, true)
		return
	}
	if err = b.writeLockFile(); err != nil {
		b.handleErr(b.lockFile, "error renewing lock file", err, true, true)
		return
	}
	if b.opt.MaxLock < basicallyforever {
		fs.Infof(nil, Color(terminal.HiBlueFg, "lock file renewed for %v. New expiration: %v"), b.opt.MaxLock, b.lockFileOpt.data.TimeExpires)
	}
}

func (b *bisyncRun) lockFileIsExpired() bool {
	if b.lockFile != "" && bilib.FileExists(b.lockFile) {
		rdf, err := os.Open(b.lockFile)
		b.handleErr(b.lockFile, "error reading lock file", err, true, true)
		if err != nil {
			return false
		}
		dec := json.NewDecoder(rdf)
		var decodeErr error
		for {
			if err := dec.Decode(&b.lockFileOpt.data); err != nil {
				if err != io.EOF {
					decodeErr = err
				}
				break
			}
		}
		b.handleErr(b.lockFile, "error closing file", rdf.Close(), true, true)
		if decodeErr != nil {
			if b.opt.MaxLock < basicallyforever {
				fs.Infof(b.lockFile, Color(terminal.YellowFg, "Lock file is unreadable (decode error: %v) and --max-lock is set. Treating as expired."), decodeErr)
				markFailed(b.listing1)
				markFailed(b.listing2)
				return true
			}
			fs.Errorf(b.lockFile, Color(terminal.RedFg, "Lock file exists, but contents are unreadable. (decode error: %v)"), decodeErr)
			return false
		}
		if !b.lockFileOpt.data.TimeExpires.IsZero() && b.lockFileOpt.data.TimeExpires.Before(time.Now()) {
			fs.Infof(b.lockFile, Color(terminal.GreenFg, "Lock file found, but it expired at %v. Will delete it and proceed."), b.lockFileOpt.data.TimeExpires)
			markFailed(b.listing1) // listing is untrusted so force revert to prior (if --recover) or create new ones (if --resync)
			markFailed(b.listing2)
			return true
		}
		fs.Infof(b.lockFile, Color(terminal.RedFg, "Valid lock file found. Expires at %v. (%v from now)"), b.lockFileOpt.data.TimeExpires, time.Since(b.lockFileOpt.data.TimeExpires).Abs().Round(time.Second))
		prettyprint(b.lockFileOpt.data, "Lockfile info", fs.LogLevelInfo)
	}
	return false
}

// StartLockRenewal renews the lockfile every --max-lock minus one minute.
//
// It returns a func which should be called to stop the renewal.
func (b *bisyncRun) startLockRenewal() func() {
	if b.opt.MaxLock <= 0 || b.opt.MaxLock >= basicallyforever || b.lockFile == "" {
		return func() {}
	}
	stopLockRenewal := make(chan struct{})
	var wg sync.WaitGroup
	var stopOnce sync.Once
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
		stopOnce.Do(func() { close(stopLockRenewal) })
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
