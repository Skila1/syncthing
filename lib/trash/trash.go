// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package trash

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	TrashDirName       = ".sttrash"
	MetaSuffix         = ".trashmeta"
	DefaultRetention   = 30 * 24 * time.Hour
	VersionsDirName    = ".stversions"
	MaxVersionsPerFile = 20
)

var (
	ErrTrashItemNotFound = errors.New("trash item not found")
)

// TrashEntry represents a trashed file.
type TrashEntry struct {
	ID           string `json:"id"`
	OriginalPath string `json:"originalPath"`
	TrashPath    string `json:"trashPath"`
	DeletedAt    int64  `json:"deletedAt"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"isDir"`
}

// VersionEntry represents a historical version of a file.
type VersionEntry struct {
	Path      string `json:"path"`
	Version   string `json:"version"`
	ModTime   int64  `json:"modTime"`
	Size      int64  `json:"size"`
	StorePath string `json:"-"`
}

// CleanupPolicy defines auto-cleanup rules.
type CleanupPolicy struct {
	ID           int64  `json:"id" db:"id"`
	UserID       int64  `json:"userId" db:"user_id"`
	MaxAgeDays   int    `json:"maxAgeDays" db:"max_age_days"`
	MaxSizeBytes int64  `json:"maxSizeBytes" db:"max_size_bytes"`
	Pattern      string `json:"pattern" db:"pattern"`
	Enabled      bool   `json:"enabled" db:"enabled"`
}

// CleanupStore defines persistence for cleanup policies.
type CleanupStore interface {
	GetCleanupPolicies(userID int64) ([]CleanupPolicy, error)
	GetAllCleanupPolicies() ([]CleanupPolicy, error)
	SetCleanupPolicy(policy *CleanupPolicy) error
	DeleteCleanupPolicy(id int64) error
}

// trashDir returns the .sttrash directory path for a folder root.
func trashDir(folderRoot string) string {
	return filepath.Join(folderRoot, TrashDirName)
}

// versionsDir returns the .stversions directory path for a folder root.
func versionsDir(folderRoot string) string {
	return filepath.Join(folderRoot, VersionsDirName)
}

// MoveToTrash moves a file/directory to the trash instead of deleting it.
// Returns the trash entry for the moved item.
func MoveToTrash(folderRoot, relPath string) (*TrashEntry, error) {
	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") || clean == "." || clean == "" {
		return nil, errors.New("invalid path")
	}

	srcPath := filepath.Join(folderRoot, filepath.FromSlash(clean))
	info, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}

	td := trashDir(folderRoot)
	if err := os.MkdirAll(td, 0o755); err != nil {
		return nil, err
	}

	now := time.Now()
	id := now.Format("20060102-150405.000") + "-" + filepath.Base(clean)
	destPath := filepath.Join(td, id)

	if err := os.Rename(srcPath, destPath); err != nil {
		return nil, err
	}

	entry := &TrashEntry{
		ID:           id,
		OriginalPath: filepath.ToSlash(clean),
		TrashPath:    destPath,
		DeletedAt:    now.UnixMilli(),
		Size:         info.Size(),
		IsDir:        info.IsDir(),
	}

	metaPath := destPath + MetaSuffix
	metaData, _ := json.Marshal(entry)
	if err := os.WriteFile(metaPath, metaData, 0o644); err != nil {
		slog.Error("Failed to write trash metadata", "error", err)
	}

	return entry, nil
}

// ListTrash lists all items in the trash folder.
func ListTrash(folderRoot string) ([]TrashEntry, error) {
	td := trashDir(folderRoot)
	dirEntries, err := os.ReadDir(td)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entries []TrashEntry
	for _, de := range dirEntries {
		if strings.HasSuffix(de.Name(), MetaSuffix) {
			metaPath := filepath.Join(td, de.Name())
			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}
			var entry TrashEntry
			if err := json.Unmarshal(data, &entry); err != nil {
				continue
			}
			entry.TrashPath = filepath.Join(td, entry.ID)
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].DeletedAt > entries[j].DeletedAt
	})
	return entries, nil
}

// RestoreFromTrash moves a trashed item back to its original location.
func RestoreFromTrash(folderRoot, trashID string) error {
	td := trashDir(folderRoot)
	metaPath := filepath.Join(td, trashID+MetaSuffix)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ErrTrashItemNotFound
	}

	var entry TrashEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}

	srcPath := filepath.Join(td, trashID)
	destPath := filepath.Join(folderRoot, filepath.FromSlash(entry.OriginalPath))

	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		return err
	}

	os.Remove(metaPath)
	return nil
}

// PermanentDelete removes an item from trash permanently.
func PermanentDelete(folderRoot, trashID string) error {
	td := trashDir(folderRoot)
	itemPath := filepath.Join(td, trashID)
	metaPath := itemPath + MetaSuffix

	if err := os.RemoveAll(itemPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	os.Remove(metaPath)
	return nil
}

// EmptyTrash permanently deletes all items in the trash.
func EmptyTrash(folderRoot string) error {
	td := trashDir(folderRoot)
	if err := os.RemoveAll(td); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CleanExpiredTrash removes items older than the retention period.
func CleanExpiredTrash(folderRoot string, retention time.Duration) (int, error) {
	entries, err := ListTrash(folderRoot)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-retention).UnixMilli()
	cleaned := 0
	for _, entry := range entries {
		if entry.DeletedAt < cutoff {
			if err := PermanentDelete(folderRoot, entry.ID); err != nil {
				slog.Error("Failed to clean expired trash item", "id", entry.ID, "error", err)
				continue
			}
			cleaned++
		}
	}
	return cleaned, nil
}

// --- File Versioning ---

// SaveVersion copies a file to the versions directory before overwrite.
func SaveVersion(folderRoot, relPath string) error {
	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") || clean == "." {
		return errors.New("invalid path")
	}

	srcPath := filepath.Join(folderRoot, filepath.FromSlash(clean))
	info, err := os.Stat(srcPath)
	if err != nil || info.IsDir() {
		return nil
	}

	vd := versionsDir(folderRoot)
	verDir := filepath.Join(vd, filepath.Dir(filepath.FromSlash(clean)))
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(info.Name())
	base := strings.TrimSuffix(info.Name(), ext)
	versionName := base + "~" + timestamp + ext
	destPath := filepath.Join(verDir, versionName)

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return err
	}

	pruneVersions(folderRoot, clean)
	return nil
}

// ListVersions returns all saved versions for a file.
func ListVersions(folderRoot, relPath string) ([]VersionEntry, error) {
	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") {
		return nil, errors.New("invalid path")
	}

	vd := versionsDir(folderRoot)
	verDir := filepath.Join(vd, filepath.Dir(filepath.FromSlash(clean)))

	dirEntries, err := os.ReadDir(verDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	ext := filepath.Ext(filepath.Base(clean))
	base := strings.TrimSuffix(filepath.Base(clean), ext)
	prefix := base + "~"

	var versions []VersionEntry
	for _, de := range dirEntries {
		name := de.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
			continue
		}

		verPart := strings.TrimPrefix(name, prefix)
		verPart = strings.TrimSuffix(verPart, ext)

		info, err := de.Info()
		if err != nil {
			continue
		}

		storePath := filepath.Join(verDir, name)
		versions = append(versions, VersionEntry{
			Path:      filepath.ToSlash(clean),
			Version:   verPart,
			ModTime:   info.ModTime().UnixMilli(),
			Size:      info.Size(),
			StorePath: storePath,
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})
	return versions, nil
}

// RestoreVersion copies a versioned file back to its original location,
// saving the current version first.
func RestoreVersion(folderRoot, relPath, version string) error {
	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") {
		return errors.New("invalid path")
	}

	ext := filepath.Ext(filepath.Base(clean))
	base := strings.TrimSuffix(filepath.Base(clean), ext)
	versionFileName := base + "~" + version + ext

	vd := versionsDir(folderRoot)
	verDir := filepath.Join(vd, filepath.Dir(filepath.FromSlash(clean)))
	srcPath := filepath.Join(verDir, versionFileName)

	if _, err := os.Stat(srcPath); err != nil {
		return err
	}

	// Save current version before restoring
	if err := SaveVersion(folderRoot, clean); err != nil {
		slog.Error("Failed to save current version before restore", "error", err)
	}

	destPath := filepath.Join(folderRoot, filepath.FromSlash(clean))
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = dst.ReadFrom(src)
	return err
}

// pruneVersions keeps only MaxVersionsPerFile versions, deleting the oldest.
func pruneVersions(folderRoot, relPath string) {
	versions, err := ListVersions(folderRoot, relPath)
	if err != nil || len(versions) <= MaxVersionsPerFile {
		return
	}

	for _, v := range versions[MaxVersionsPerFile:] {
		os.Remove(v.StorePath)
	}
}
