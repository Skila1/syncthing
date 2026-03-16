// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package syncext

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// UserBandwidth holds per-user bandwidth limits.
type UserBandwidth struct {
	UserID      int64 `json:"userId" db:"user_id"`
	MaxSendKbps int   `json:"maxSendKbps" db:"max_send_kbps"`
	MaxRecvKbps int   `json:"maxRecvKbps" db:"max_recv_kbps"`
}

// SyncSchedule defines a time window for syncing.
type SyncSchedule struct {
	ID         int64  `json:"id" db:"id"`
	UserID     int64  `json:"userId" db:"user_id"`
	FolderID   string `json:"folderId" db:"folder_id"`
	StartTime  string `json:"startTime" db:"start_time"`
	EndTime    string `json:"endTime" db:"end_time"`
	DaysOfWeek string `json:"daysOfWeek" db:"days_of_week"`
	Enabled    bool   `json:"enabled" db:"enabled"`
}

// ConflictFile represents a detected sync conflict.
type ConflictFile struct {
	Path         string `json:"path"`
	OriginalPath string `json:"originalPath"`
	ModTime      int64  `json:"modTime"`
	Size         int64  `json:"size"`
	DeviceShort  string `json:"deviceShort"`
}

// Store defines persistence for sync enhancements.
type Store interface {
	GetUserBandwidth(userID int64) (*UserBandwidth, error)
	SetUserBandwidth(bw *UserBandwidth) error

	ListSchedules(userID int64) ([]SyncSchedule, error)
	SetSchedule(s *SyncSchedule) error
	DeleteSchedule(id int64) error
}

// IsWithinSchedule checks if the current time falls within any active
// schedule for the user. Returns true if syncing is allowed.
// If no schedules exist, syncing is always allowed.
func IsWithinSchedule(schedules []SyncSchedule, folderID string) bool {
	if len(schedules) == 0 {
		return true
	}

	now := time.Now()
	currentDay := byte('0' + byte(now.Weekday()))
	currentTime := now.Format("15:04")

	for _, s := range schedules {
		if !s.Enabled {
			continue
		}
		if s.FolderID != "" && s.FolderID != folderID {
			continue
		}
		if !strings.ContainsRune(s.DaysOfWeek, rune(currentDay)) {
			continue
		}
		if isTimeInRange(currentTime, s.StartTime, s.EndTime) {
			return true
		}
	}

	hasActiveSchedules := false
	for _, s := range schedules {
		if s.Enabled && (s.FolderID == "" || s.FolderID == folderID) {
			hasActiveSchedules = true
			break
		}
	}
	if !hasActiveSchedules {
		return true
	}

	return false
}

func isTimeInRange(current, start, end string) bool {
	if start <= end {
		return current >= start && current <= end
	}
	return current >= start || current <= end
}

// ListConflicts scans a folder for .sync-conflict files.
func ListConflicts(folderRoot string, maxResults int) ([]ConflictFile, error) {
	if maxResults <= 0 {
		maxResults = 100
	}

	var conflicts []ConflictFile
	err := filepath.WalkDir(folderRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(conflicts) >= maxResults {
			return filepath.SkipAll
		}

		name := d.Name()
		if !strings.Contains(name, ".sync-conflict-") {
			return nil
		}

		rel, _ := filepath.Rel(folderRoot, path)
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		original := resolveOriginalPath(name)

		deviceShort := extractDeviceShort(name)

		conflicts = append(conflicts, ConflictFile{
			Path:         filepath.ToSlash(rel),
			OriginalPath: original,
			ModTime:      info.ModTime().UnixMilli(),
			Size:         info.Size(),
			DeviceShort:  deviceShort,
		})
		return nil
	})

	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].ModTime > conflicts[j].ModTime
	})

	return conflicts, err
}

// resolveOriginalPath strips the .sync-conflict-* portion from the filename.
func resolveOriginalPath(name string) string {
	idx := strings.Index(name, ".sync-conflict-")
	if idx < 0 {
		return name
	}
	ext := filepath.Ext(name)
	base := name[:idx]
	return base + ext
}

// extractDeviceShort pulls the device short ID from the conflict filename.
func extractDeviceShort(name string) string {
	idx := strings.Index(name, ".sync-conflict-")
	if idx < 0 {
		return ""
	}
	rest := name[idx+len(".sync-conflict-"):]
	ext := filepath.Ext(name)
	rest = strings.TrimSuffix(rest, ext)
	parts := strings.Split(rest, "-")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// ResolveConflict handles a conflict by keeping one version and removing the other.
// action: "keep-original" removes the conflict file,
//
//	"keep-conflict" replaces original with conflict file,
//	"keep-both" does nothing (both files remain).
func ResolveConflict(folderRoot, conflictRelPath, action string) error {
	conflictAbs := filepath.Join(folderRoot, filepath.FromSlash(conflictRelPath))
	originalName := resolveOriginalPath(filepath.Base(conflictRelPath))
	originalAbs := filepath.Join(filepath.Dir(conflictAbs), originalName)

	switch action {
	case "keep-original":
		return os.Remove(conflictAbs)
	case "keep-conflict":
		if err := os.Remove(originalAbs); err != nil && !os.IsNotExist(err) {
			return err
		}
		return os.Rename(conflictAbs, originalAbs)
	case "keep-both":
		return nil
	default:
		return nil
	}
}
