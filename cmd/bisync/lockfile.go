package bisync

import (
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
	guard       *flock.Flock
	ownedFile   os.FileInfo
	cleanupMu   sync.Mutex
	stopRenewal func()
	data        struct {
		Session     string
		PID         string
		TimeRenewed time.Time
		TimeExpires time.Time
	}
}

func (b *bisyncRun) setLockFile() (err error) {
	b.lockFile = ""
	// The guard inode is deliberately retained. Removing an OS lock file can
	// let separate processes lock different inodes under the same pathname.
	guard := flock.New(b.basePath + ".lck-guard")
	locked, err := guard.TryLock()
	if err != nil {
		return fmt.Errorf("cannot acquire bisync guard: %w", err)
	}
	if !locked {
		return fmt.Errorf("another bisync run owns the state for %s", b.basePath)
	}
	b.lockFileOpt.guard = guard
	defer func() {
		if err != nil {
			_ = guard.Unlock()
			b.lockFileOpt.guard = nil
		}
	}()
	b.setLockFileExpiration()
	if !b.opt.DryRun {
		b.lockFile = b.basePath + ".lck"
		if bilib.FileExists(b.lockFile) {
			if !b.lockFileIsExpired() {
				errTip := Color(terminal.MagentaFg, "Tip: this indicates that another bisync run (of these same paths) either is still running or was interrupted before completion. \n")
				errTip += Color(terminal.MagentaFg, "If you're SURE you want to override this safety feature, you can delete the lock file with the following command, then run bisync again: \n")
				errTip += fmt.Sprintf(Color(terminal.HiRedFg, "rclone deletefile \"%s\""), b.lockFile)
				return fmt.Errorf(Color(terminal.RedFg, "prior lock file found: %s \n")+errTip, Color(terminal.HiYellowFg, b.lockFile))
			}
			// Expiration cannot evict a live updated process: it still owns guard.
			if err = os.Remove(b.lockFile); err != nil {
				return fmt.Errorf("cannot remove expired lock file: %w", err)
			}
		}

		pidStr := []byte(strconv.Itoa(os.Getpid()))
		file, openErr := os.OpenFile(b.lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, bilib.PermSecure)
		if openErr != nil {
			return fmt.Errorf("cannot exclusively create lock file: %w", openErr)
		}
		b.lockFileOpt.ownedFile, err = file.Stat()
		if err == nil {
			_, err = file.Write(pidStr)
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf(Color(terminal.RedFg, "cannot create lock file: %s: %w"), b.lockFile, err)
		}
		fs.Debugf(nil, "Lock file created: %s", b.lockFile)
		b.renewLockFile()
		b.lockFileOpt.stopRenewal = b.startLockRenewal()
	}
	return nil
}

func (b *bisyncRun) removeLockFile() (err error) {
	b.lockFileOpt.cleanupMu.Lock()
	defer b.lockFileOpt.cleanupMu.Unlock()
	defer func() {
		if guard := b.lockFileOpt.guard; guard != nil {
			if unlockErr := guard.Unlock(); err == nil {
				err = unlockErr
			}
			b.lockFileOpt.guard = nil
		}
	}()
	if b.lockFile != "" {
		if b.lockFileOpt.stopRenewal != nil {
			b.lockFileOpt.stopRenewal()
		}
		current, statErr := os.Stat(b.lockFile)
		if statErr == nil && b.lockFileOpt.ownedFile != nil && os.SameFile(current, b.lockFileOpt.ownedFile) {
			err = os.Remove(b.lockFile)
		} else if !os.IsNotExist(statErr) {
			err = fmt.Errorf("refusing to remove a bisync lock no longer owned by this run: %s", b.lockFile)
		}
		if err == nil {
			fs.Debugf(nil, "Lock file removed: %s", b.lockFile)
		} else {
			fs.Errorf(nil, "cannot remove lockfile %s: %v", b.lockFile, err)
		}
		b.lockFile = "" // block removing it again
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

func (b *bisyncRun) renewLockFile() {
	if b.lockFile != "" && bilib.FileExists(b.lockFile) {

		b.lockFileOpt.data.Session = b.basePath
		b.lockFileOpt.data.PID = strconv.Itoa(os.Getpid())
		b.lockFileOpt.data.TimeRenewed = time.Now()
		b.lockFileOpt.data.TimeExpires = time.Now().Add(time.Duration(b.opt.MaxLock))

		// Open without truncating, then prove ownership before touching contents.
		df, err := os.OpenFile(b.lockFile, os.O_WRONLY, bilib.PermSecure)
		if err != nil {
			b.handleErr(b.lockFile, "error renewing lock file", err, true, true)
			return
		}
		defer func() { _ = df.Close() }()
		info, err := df.Stat()
		if err != nil || b.lockFileOpt.ownedFile == nil || !os.SameFile(info, b.lockFileOpt.ownedFile) {
			b.handleErr(b.lockFile, "lock ownership changed", fmt.Errorf("refusing to renew a different lock file"), true, true)
			return
		}
		data, err := json.Marshal(b.lockFileOpt.data)
		if err == nil {
			_, err = df.Write(data)
		}
		if err == nil {
			err = df.Truncate(int64(len(data)))
		}
		if err == nil {
			err = df.Sync()
		}
		if err != nil {
			b.handleErr(b.lockFile, "error renewing lock data", err, true, true)
			return
		}
		if b.opt.MaxLock < basicallyforever {
			fs.Infof(nil, Color(terminal.HiBlueFg, "lock file renewed for %v. New expiration: %v"), b.opt.MaxLock, b.lockFileOpt.data.TimeExpires)
		}
	}
}

func (b *bisyncRun) lockFileIsExpired() bool {
	if b.lockFile != "" && bilib.FileExists(b.lockFile) {
		rdf, err := os.Open(b.lockFile)
		if err != nil {
			b.handleErr(b.lockFile, "error reading lock file", err, true, true)
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
	wg.Go(func() {
		ticker := time.NewTicker(time.Duration(b.opt.MaxLock) - time.Minute)
		for {
			select {
			case <-ticker.C:
				b.renewLockFile()
			case <-stopLockRenewal:
				ticker.Stop()
				return
			}
		}
	})
	var stopOnce sync.Once
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
