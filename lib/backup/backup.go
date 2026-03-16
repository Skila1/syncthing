// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	TypeFull        = "full"
	TypeIncremental = "incremental"

	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Backup represents a backup job record.
type Backup struct {
	ID          int64  `json:"id" db:"id"`
	UserID      int64  `json:"userId" db:"user_id"`
	FolderID    string `json:"folderId" db:"folder_id"`
	Type        string `json:"type" db:"type"`
	Status      string `json:"status" db:"status"`
	StartedAt   int64  `json:"startedAt" db:"started_at"`
	CompletedAt int64  `json:"completedAt" db:"completed_at"`
	SizeBytes   int64  `json:"sizeBytes" db:"size_bytes"`
	BackupPath  string `json:"backupPath" db:"backup_path"`
	CreatedAt   int64  `json:"createdAt" db:"created_at"`
}

// Snapshot represents a point-in-time folder snapshot.
type Snapshot struct {
	ID           int64  `json:"id" db:"id"`
	UserID       int64  `json:"userId" db:"user_id"`
	FolderID     string `json:"folderId" db:"folder_id"`
	Name         string `json:"name" db:"name"`
	SnapshotPath string `json:"snapshotPath" db:"snapshot_path"`
	SizeBytes    int64  `json:"sizeBytes" db:"size_bytes"`
	MetadataJSON string `json:"metadata" db:"metadata_json"`
	CreatedAt    int64  `json:"createdAt" db:"created_at"`
}

// Store defines persistence for backups and snapshots.
type Store interface {
	ListBackups(userID int64) ([]Backup, error)
	GetBackup(id int64) (*Backup, error)
	CreateBackup(b *Backup) error
	UpdateBackup(b *Backup) error
	DeleteBackup(id int64) error

	ListSnapshots(userID int64, folderID string) ([]Snapshot, error)
	GetSnapshot(id int64) (*Snapshot, error)
	CreateSnapshot(s *Snapshot) error
	DeleteSnapshot(id int64) error
}

// RunFullBackup copies a source folder to a backup destination.
func RunFullBackup(srcPath, destPath string) (int64, error) {
	if err := os.MkdirAll(destPath, 0700); err != nil {
		return 0, err
	}

	var totalBytes int64
	err := filepath.WalkDir(srcPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(srcPath, path)
		target := filepath.Join(destPath, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}

		size, copyErr := copyFile(path, target)
		if copyErr != nil {
			return nil
		}
		totalBytes += size
		return nil
	})
	return totalBytes, err
}

// CreateFolderSnapshot creates a snapshot by copying the folder tree and
// recording file metadata.
func CreateFolderSnapshot(folderPath, snapshotDir, name string) (*Snapshot, error) {
	ts := time.Now().Format("20060102-150405")
	snapName := fmt.Sprintf("%s-%s", name, ts)
	snapPath := filepath.Join(snapshotDir, snapName)

	size, err := RunFullBackup(folderPath, snapPath)
	if err != nil {
		return nil, err
	}

	meta := map[string]interface{}{
		"source":    folderPath,
		"timestamp": ts,
	}
	metaBytes, _ := json.Marshal(meta)

	return &Snapshot{
		Name:         snapName,
		SnapshotPath: snapPath,
		SizeBytes:    size,
		MetadataJSON: string(metaBytes),
		CreatedAt:    time.Now().UnixMilli(),
	}, nil
}

// RestoreSnapshot copies a snapshot back to the target folder.
func RestoreSnapshot(snapshotPath, targetPath string) error {
	_, err := RunFullBackup(snapshotPath, targetPath)
	return err
}

func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	n, err := io.Copy(out, in)
	return n, err
}
