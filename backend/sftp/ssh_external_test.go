//go:build !plan9

package sftp

import (
	"runtime"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
)

func externalShell() fs.SpaceSepList {
	if runtime.GOOS == "windows" {
		return fs.SpaceSepList{"cmd", "/d", "/c"}
	}
	return fs.SpaceSepList{"echo", "test"}
}

// TestSSHExternalWaitMultipleCalls verifies that calling Wait() multiple times
// doesn't cause zombie processes
func TestSSHExternalWaitMultipleCalls(t *testing.T) {
	// Create a minimal Fs object for testing
	opt := &Options{
		SSH: externalShell(),
	}

	f := &Fs{
		opt: *opt,
	}

	// Create a new SSH session
	session := f.newSSHSessionExternal()

	// Start a simple command that exits quickly
	err := session.Start("exit 0")
	assert.NoError(t, err)

	// Give the command time to complete
	time.Sleep(100 * time.Millisecond)

	// Call Wait() multiple times - this should not cause issues
	err1 := session.Wait()
	err2 := session.Wait()
	err3 := session.Wait()

	// All calls should return the same result (no error in this case)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)

	// Verify the process has exited
	assert.True(t, session.exited())
}

// TestSSHExternalCloseMultipleCalls verifies that calling Close() multiple times
// followed by Wait() calls doesn't cause zombie processes
func TestSSHExternalCloseMultipleCalls(t *testing.T) {
	// Create a minimal Fs object for testing
	opt := &Options{
		SSH: externalShell(),
	}

	f := &Fs{
		opt: *opt,
	}

	// Create a new SSH session
	session := f.newSSHSessionExternal()

	// Start a long-running command
	command := "sleep 10"
	if runtime.GOOS == "windows" {
		command = "timeout /t 10 /nobreak >NUL"
	}
	err := session.Start(command)
	if err != nil {
		t.Skip("Cannot start sleep command:", err)
	}

	// Close should cancel and wait for the process
	_ = session.Close()

	// Additional Wait() calls should return the same error
	err2 := session.Wait()
	err3 := session.Wait()

	// All should complete without panicking
	// err1 could be nil or an error depending on how the process was killed
	// err2 and err3 should be the same
	assert.Equal(t, err2, err3, "Subsequent Wait() calls should return same result")

	// Verify the process has exited
	assert.True(t, session.exited())
}
