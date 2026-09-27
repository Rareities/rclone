// Copyright (c) 2026 The rclone authors

package bisync

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/rclone/rclone/fs/accounting"
)

type previewSummary struct {
	Version                 int    `json:"version"`
	Status                  string `json:"status"`
	PlannedTransfers        int64  `json:"plannedTransfers"`
	PlannedBytes            int64  `json:"plannedBytes"`
	PlannedFileDeletes      int64  `json:"plannedFileDeletes"`
	PlannedDirectoryDeletes int64  `json:"plannedDirectoryDeletes"`
	ErrorCount              int64  `json:"errorCount"`
	ConflictsKnown          bool   `json:"conflictsKnown"`
}

func validatePreviewJSON(previewJSON, dryRun, inspectState bool) error {
	if !previewJSON {
		return nil
	}
	if inspectState {
		return errors.New("--preview-json cannot be combined with --inspect-state")
	}
	if !dryRun {
		return errors.New("--preview-json requires --dry-run")
	}
	return nil
}

func previewSummaryFromStats(stats *accounting.StatsInfo) previewSummary {
	status := "COMPLETE"
	if stats.GetErrors() != 0 || stats.HadFatalError() || stats.HadRetryError() {
		status = "INCOMPLETE"
	}
	return previewSummary{
		Version:                 1,
		Status:                  status,
		PlannedTransfers:        stats.GetTransfers(),
		PlannedBytes:            stats.GetBytes(),
		PlannedFileDeletes:      stats.GetDeletes(),
		PlannedDirectoryDeletes: stats.GetDeletedDirs(),
		ErrorCount:              stats.GetErrors(),
		ConflictsKnown:          false,
	}
}

func writePreviewJSON(writer io.Writer, stats *accounting.StatsInfo) error {
	return json.NewEncoder(writer).Encode(previewSummaryFromStats(stats))
}
