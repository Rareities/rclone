//go:build !plan9

package logger_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/rclone/rclone/fs/logger"
	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	// This enables the testscript package. See:
	// https://bitfieldconsulting.com/golang/cli-testing
	// https://pkg.go.dev/github.com/rogpeppe/go-internal@v1.11.0/testscript
	testscript.Main(m, map[string]func(){
		"rclone": logger.Main,
	})
}

func TestLogger(t *testing.T) {
	// Usage: https://bitfieldconsulting.com/golang/cli-testing

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"diffdirs":  diffDirs,
			"sortfiles": sortFiles,
		},
		Setup: func(env *testscript.Env) error {
			src := filepath.Join(env.WorkDir, "src")
			dst := filepath.Join(env.WorkDir, "dst")
			// Fill src and dst with two overlapping trees of files so the
			// scripts have a realistic mix of matching, differing and
			// missing files to compare. This used to download two old
			// rclone source archives from GitHub which made the tests fail
			// whenever the network or GitHub was flaky.
			if err := makeTestTrees(src, dst); err != nil {
				return err
			}
			env.Setenv("SRC", src)
			env.Setenv("DST", dst)
			return nil
		},
	})
}

type treeEntry struct {
	kind string
	data []byte
}

func diffDirs(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("diffdirs does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("diffdirs <left> <right>")
	}
	left, err := readTree(ts.MkAbs(args[0]))
	if err != nil {
		ts.Fatalf("read %q: %v", args[0], err)
	}
	right, err := readTree(ts.MkAbs(args[1]))
	if err != nil {
		ts.Fatalf("read %q: %v", args[1], err)
	}
	keys := make([]string, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		seen[key] = struct{}{}
	}
	for key := range right {
		seen[key] = struct{}{}
	}
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		l, lok := left[key]
		r, rok := right[key]
		if !lok || !rok || l.kind != r.kind || !bytes.Equal(l.data, r.data) {
			ts.Fatalf("directory trees differ at %q", key)
		}
	}
}

func readTree(root string) (map[string]treeEntry, error) {
	entries := make(map[string]treeEntry)
	err := filepath.WalkDir(root, func(path string, dirEntry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		key := filepath.ToSlash(relative)
		if dirEntry.IsDir() {
			entries[key] = treeEntry{kind: "directory"}
			return nil
		}
		if dirEntry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			entries[key] = treeEntry{kind: "symlink", data: []byte(link)}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[key] = treeEntry{kind: "file", data: data}
		return nil
	})
	return entries, err
}

// sortFiles provides the small sorting primitive used by the scripts without
// depending on a Unix sort executable. Windows' built-in sort has different
// flags and rejects in-place input/output, while the test only needs stable
// line ordering for byte-for-byte comparisons.
func sortFiles(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("sortfiles does not support negation")
	}
	if len(args) != 1 {
		ts.Fatalf("sortfiles <file>")
	}
	path := ts.MkAbs(args[0])
	contents, err := os.ReadFile(path)
	if err != nil {
		ts.Fatalf("read %q: %v", args[0], err)
	}
	trailingNewline := bytes.HasSuffix(contents, []byte{'\n'})
	lines := bytes.Split(contents, []byte{'\n'})
	if trailingNewline {
		lines = lines[:len(lines)-1]
	}
	sort.SliceStable(lines, func(i, j int) bool {
		return bytes.Compare(lines[i], lines[j]) < 0
	})
	sorted := bytes.Join(lines, []byte{'\n'})
	if trailingNewline {
		sorted = append(sorted, '\n')
	}
	if err := os.WriteFile(path, sorted, 0666); err != nil {
		ts.Fatalf("write %q: %v", args[0], err)
	}
}

// makeTestTrees populates src and dst with a deterministic set of files
// covering every comparison category the scripts exercise:
//
//   - identical files present in both (match)
//   - same-named files with different content (differ)
//   - files only in src (missing on dst)
//   - files only in dst (missing on src)
//
// The files are spread across the root and a shared subdirectory so the
// listings need sorting and the directory survives a sync (which deletes the
// dst-only files but keeps the matching ones).
func makeTestTrees(src, dst string) error {
	const perCategory = 25
	dirs := []string{".", "sub"}

	write := func(root, dir, name, content string) error {
		path := filepath.Join(root, dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(content), 0666)
	}

	for _, dir := range dirs {
		for i := range perCategory {
			match := fmt.Sprintf("the same content for match%02d\n", i)
			if err := write(src, dir, fmt.Sprintf("match%02d.txt", i), match); err != nil {
				return err
			}
			if err := write(dst, dir, fmt.Sprintf("match%02d.txt", i), match); err != nil {
				return err
			}

			if err := write(src, dir, fmt.Sprintf("differ%02d.txt", i), fmt.Sprintf("src content %02d\n", i)); err != nil {
				return err
			}
			if err := write(dst, dir, fmt.Sprintf("differ%02d.txt", i), fmt.Sprintf("dst content %02d differs\n", i)); err != nil {
				return err
			}

			if err := write(src, dir, fmt.Sprintf("srconly%02d.txt", i), fmt.Sprintf("only in src %02d\n", i)); err != nil {
				return err
			}
			if err := write(dst, dir, fmt.Sprintf("dstonly%02d.txt", i), fmt.Sprintf("only in dst %02d\n", i)); err != nil {
				return err
			}
		}
	}
	return nil
}
