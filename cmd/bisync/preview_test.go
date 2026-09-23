// Copyright (c) 2026 The rclone authors

package bisync

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs/accounting"
	"github.com/stretchr/testify/require"
)

func TestValidatePreviewJSON(t *testing.T) {
	require.NoError(t, validatePreviewJSON(false, false, false))
	require.NoError(t, validatePreviewJSON(true, true, false))
	require.EqualError(t, validatePreviewJSON(true, false, false), "--preview-json requires --dry-run")
	require.EqualError(t, validatePreviewJSON(true, true, true), "--preview-json cannot be combined with --inspect-state")
}

func TestPreviewJSONIsPathFreeAndDisclosesUnknownConflicts(t *testing.T) {
	stats := accounting.NewStats(context.Background())
	stats.DeletedDirs(3)
	var output bytes.Buffer
	require.NoError(t, writePreviewJSON(&output, stats))

	var got map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &got))
	require.EqualValues(t, 1, got["version"])
	require.Equal(t, "COMPLETE", got["status"])
	require.EqualValues(t, 3, got["plannedDirectoryDeletes"])
	require.Equal(t, false, got["conflictsKnown"])
	require.NotContains(t, strings.ToLower(output.String()), "path")
	require.NotContains(t, strings.ToLower(output.String()), "object")
}

func TestPreviewJSONMarksNativeErrorsIncomplete(t *testing.T) {
	stats := accounting.NewStats(context.Background())
	stats.Errors(1)
	var output bytes.Buffer
	require.NoError(t, writePreviewJSON(&output, stats))

	var got map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &got))
	require.Equal(t, "INCOMPLETE", got["status"])
	require.EqualValues(t, 1, got["errorCount"])
}
