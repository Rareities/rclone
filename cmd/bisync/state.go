// Package bisync implements bisync
// Copyright (c) 2017-2020 Chris Nelson
package bisync

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
)

const (
	stateInspectionVersion = 1
	maxListingFileBytes    = 64 << 20
	maxListingEntries      = 200_000
	maxListingLineBytes    = 1 << 20
)

// StateStatus describes the safety state of a native Bisync work directory.
type StateStatus string

const (
	StateAbsent       StateStatus = "ABSENT"
	StateCompatible   StateStatus = "COMPATIBLE"
	StateInterrupted  StateStatus = "INTERRUPTED"
	StateIncompatible StateStatus = "INCOMPATIBLE"
	StateUnknown      StateStatus = "UNKNOWN"
)

// StateInspection reports native state without migrating or changing listings.
type StateInspection struct {
	Version               int         `json:"version"`
	Status                StateStatus `json:"status"`
	Reason                string      `json:"reason"`
	RecoveryListingsValid bool        `json:"recoveryListingsValid"`
}

func stateResult(status StateStatus, reason string, recoveryValid bool) StateInspection {
	return StateInspection{
		Version:               stateInspectionVersion,
		Status:                status,
		Reason:                reason,
		RecoveryListingsValid: recoveryValid,
	}
}

// InspectState checks Bisync listings under the native process guard. It does not create the
// work directory, rename legacy listings, restore backups, or access either endpoint's contents.
func InspectState(ctx context.Context, fs1, fs2 fs.Fs, optArg *Options) StateInspection {
	if fs1 == nil || fs2 == nil || optArg == nil || strings.TrimSpace(optArg.Workdir) == "" {
		return stateResult(StateUnknown, "INVALID_INPUT", false)
	}
	workDir, err := filepath.Abs(optArg.Workdir)
	if err != nil {
		return stateResult(StateUnknown, "WORKDIR_INVALID", false)
	}
	workInfo, err := os.Lstat(workDir)
	if errors.Is(err, os.ErrNotExist) {
		return stateResult(StateAbsent, "WORKDIR_ABSENT", false)
	}
	if err != nil || !workInfo.IsDir() || workInfo.Mode()&os.ModeSymlink != 0 {
		return stateResult(StateUnknown, "WORKDIR_UNSAFE", false)
	}

	session := bilib.SessionName(fs1, fs2)
	basePath := filepath.Join(workDir, session)
	suffixedSession := bilib.CanonicalPath(bilib.FsPath(fs1)) + ".." + bilib.CanonicalPath(bilib.FsPath(fs2))
	if bilib.HasHexString(suffixedSession) {
		legacyBase := filepath.Join(workDir, suffixedSession)
		for _, suffix := range []string{".path1.lst", ".path2.lst", ".path1.lst-old", ".path2.lst-old"} {
			legacyPath := legacyBase + suffix
			exists, unsafe := stateFileExists(legacyPath)
			if unsafe {
				return stateResult(StateUnknown, "STATE_FILE_UNSAFE", false)
			} else if exists {
				return stateResult(StateUnknown, "LEGACY_NAME_REQUIRES_EXPLICIT_MIGRATION", false)
			}
		}
	}

	guardPath := basePath + ".lck.guard"
	if info, statErr := os.Lstat(guardPath); statErr == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return stateResult(StateUnknown, "LOCK_GUARD_UNSAFE", false)
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return stateResult(StateUnknown, "LOCK_GUARD_UNREADABLE", false)
	}
	guard := flock.New(guardPath, flock.SetFlag(os.O_CREATE|os.O_RDWR), flock.SetPermissions(bilib.PermSecure))
	acquired, err := guard.TryLock()
	if err != nil {
		return stateResult(StateUnknown, "LOCK_GUARD_UNAVAILABLE", false)
	}
	if !acquired {
		return stateResult(StateUnknown, "NATIVE_RUN_ACTIVE", false)
	}
	defer func() { _ = guard.Unlock() }()

	lockPath := basePath + ".lck"
	lockExists, unsafe := stateFileExists(lockPath)
	if unsafe {
		return stateResult(StateUnknown, "LOCK_METADATA_UNSAFE", false)
	}
	staleOwner := false
	if lockExists {
		metadata, readErr := readLockMetadata(lockPath)
		if readErr != nil || metadata.Version != lockFileVersion || metadata.OwnerToken == "" ||
			(metadata.State != "active" && metadata.State != "released") {
			return stateResult(StateUnknown, "LOCK_METADATA_UNVERIFIED", false)
		}
		staleOwner = metadata.State == "active"
	}

	current1, unsafe1 := stateFileExists(basePath + ".path1.lst")
	current2, unsafe2 := stateFileExists(basePath + ".path2.lst")
	if unsafe1 || unsafe2 {
		return stateResult(StateUnknown, "LISTING_UNSAFE", false)
	}
	if !lockExists && (current1 || current2) {
		return stateResult(StateUnknown, "LOCK_METADATA_MISSING", false)
	}

	old1Path, old2Path := basePath+".path1.lst-old", basePath+".path2.lst-old"
	old1Exists, old1Unsafe := stateFileExists(old1Path)
	old2Exists, old2Unsafe := stateFileExists(old2Path)
	if old1Unsafe || old2Unsafe {
		return stateResult(StateUnknown, "RECOVERY_LISTING_UNSAFE", false)
	}

	oldValid := false
	var oldList1, oldList2 *fileList
	if old1Exists && old2Exists {
		oldList1, err = loadListingStrict(old1Path)
		if err == nil {
			oldList2, err = loadListingStrict(old2Path)
		}
		if err == nil {
			oldValid, err = listingsMatch(ctx, fs1, fs2, optArg, oldList1, oldList2)
			if err != nil {
				return stateResult(StateUnknown, "RECOVERY_VALIDATION_FAILED", false)
			}
		}
	}

	dirty, dirtyUnsafe := false, false
	for _, side := range []string{".path1.lst", ".path2.lst"} {
		for _, suffix := range []string{"-new", "-err", "-dry", "-dry-new", "-dry-old", "-dry-err"} {
			exists, unsafe := stateFileExists(basePath + side + suffix)
			dirty = dirty || exists
			dirtyUnsafe = dirtyUnsafe || unsafe
		}
	}
	if dirtyUnsafe {
		return stateResult(StateUnknown, "INTERRUPTION_ARTIFACT_UNSAFE", oldValid)
	}
	if staleOwner {
		return stateResult(StateInterrupted, "OWNER_EXITED_BEFORE_CLEAN_RELEASE", oldValid && (!current1 || !current2))
	}
	if dirty {
		return stateResult(StateInterrupted, "UNRESOLVED_RUN_ARTIFACTS", oldValid)
	}
	if !current1 && !current2 {
		if old1Exists || old2Exists {
			return stateResult(StateInterrupted, "CURRENT_LISTINGS_MISSING", oldValid)
		}
		return stateResult(StateAbsent, "LISTINGS_ABSENT", false)
	}
	if !current1 || !current2 {
		return stateResult(StateInterrupted, "CURRENT_LISTINGS_PARTIAL", oldValid)
	}

	list1, err := loadListingStrict(basePath + ".path1.lst")
	if err != nil {
		return stateResult(StateIncompatible, "PATH1_LISTING_INVALID", oldValid)
	}
	list2, err := loadListingStrict(basePath + ".path2.lst")
	if err != nil {
		return stateResult(StateIncompatible, "PATH2_LISTING_INVALID", oldValid)
	}
	match, err := listingsMatch(ctx, fs1, fs2, optArg, list1, list2)
	if err != nil {
		return stateResult(StateUnknown, "LISTING_VALIDATION_FAILED", oldValid)
	}
	if !match {
		return stateResult(StateIncompatible, "LISTINGS_DIVERGED", oldValid)
	}
	return stateResult(StateCompatible, "LISTINGS_COMPATIBLE", oldValid)
}

func stateFileExists(file string) (exists, unsafe bool) {
	info, err := os.Lstat(file)
	if errors.Is(err, os.ErrNotExist) {
		return false, false
	}
	if err != nil {
		return false, true
	}
	return true, !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0
}

func loadListingStrict(file string) (*fileList, error) {
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > maxListingFileBytes {
		return nil, fmt.Errorf("listing is not a bounded regular file")
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	last := []byte{0}
	if _, err := f.ReadAt(last, info.Size()-1); err != nil || last[0] != '\n' {
		return nil, fmt.Errorf("listing is truncated")
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	reader := bufio.NewScanner(f)
	reader.Buffer(make([]byte, 64*1024), maxListingLineBytes)
	if !reader.Scan() {
		if err := reader.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("listing header is missing")
	}
	header := strings.TrimSuffix(reader.Text(), "\r")
	headerTime := strings.TrimPrefix(header, ListingHeader+" ")
	if headerTime == header {
		return nil, fmt.Errorf("listing header is unsupported")
	}
	if _, err := time.ParseInLocation(timeFormat, headerTime, TZ); err != nil {
		return nil, fmt.Errorf("listing header timestamp is invalid")
	}

	list := newFileList()
	lastHashName := ""
	for reader.Scan() {
		line := strings.TrimSuffix(reader.Text(), "\r")
		match := lineRegex.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("listing contains a malformed entry")
		}
		flags, sizeStr, hashStr := match[1], match[2], match[3]
		id, timeStr, nameStr := match[4], match[5], match[6]
		sizeVal, sizeErr := strconv.ParseInt(sizeStr, 10, 64)
		timeVal, timeErr := time.ParseInLocation(timeFormat, timeStr, TZ)
		nameVal, nameErr := strconv.Unquote(nameStr)
		hashName, hashVal, hashErr := parseHash(hashStr)
		if hashErr == nil && hashName != "" {
			if lastHashName == "" {
				lastHashName = hashName
				hashErr = list.hash.Set(hashName)
			} else if hashName != lastHashName {
				hashErr = fmt.Errorf("listing hash type changed")
			}
		}
		if sizeErr != nil || timeErr != nil || nameErr != nil || hashErr != nil ||
			(flags != "-" && flags != "d") || id != "-" || !validListingPath(nameVal) {
			return nil, fmt.Errorf("listing contains an unsupported entry")
		}
		if list.has(nameVal) {
			return nil, fmt.Errorf("listing contains a duplicate path")
		}
		if len(list.list) >= maxListingEntries {
			return nil, fmt.Errorf("listing exceeds the entry limit")
		}
		list.put(nameVal, sizeVal, timeVal.In(TZ), hashVal, id, flags)
	}
	if err := reader.Err(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return list, nil
}

func validListingPath(name string) bool {
	if name == "" || strings.ContainsRune(name, '\x00') || path.IsAbs(name) || path.Clean(name) != name {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func listingsMatch(ctx context.Context, fs1, fs2 fs.Fs, optArg *Options, list1, list2 *fileList) (bool, error) {
	opt := *optArg
	b := &bisyncRun{fs1: fs1, fs2: fs2, opt: &opt, aliases: bilib.AliasMap{}}
	if err := b.setCompareDefaults(ctx); err != nil {
		return false, err
	}
	b.fctx = ctx
	for _, name := range list1.list {
		other := name
		if !list2.has(other) {
			other = b.aliases.Alias(name)
		}
		leftInfo, rightInfo := list1.get(name), list2.get(other)
		if leftInfo == nil || rightInfo == nil || leftInfo.flags != rightInfo.flags ||
			!b.fileInfoEqual(name, other, list1, list2) {
			return false, nil
		}
	}
	for _, name := range list2.list {
		other := name
		if !list1.has(other) {
			other = b.aliases.Alias(name)
		}
		if !list1.has(other) {
			return false, nil
		}
	}
	return true, nil
}
