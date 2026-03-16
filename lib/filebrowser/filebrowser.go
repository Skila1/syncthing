// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package filebrowser

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	Mode    string `json:"mode"`
}

type DirectoryListing struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

// ListDirectory returns the contents of a directory relative to folderRoot.
// relPath must not escape the root (no ".." traversal).
func ListDirectory(folderRoot, relPath string) (*DirectoryListing, error) {
	clean := filepath.Clean(relPath)
	if clean == "." {
		clean = ""
	}
	if strings.Contains(clean, "..") {
		return nil, &os.PathError{Op: "list", Path: relPath, Err: os.ErrPermission}
	}

	absDir := filepath.Join(folderRoot, filepath.FromSlash(clean))
	if !strings.HasPrefix(filepath.Clean(absDir), filepath.Clean(folderRoot)) {
		return nil, &os.PathError{Op: "list", Path: relPath, Err: os.ErrPermission}
	}

	dirEntries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue
		}

		entryRelPath := clean
		if entryRelPath != "" {
			entryRelPath += "/"
		}
		entryRelPath += de.Name()

		entries = append(entries, FileEntry{
			Name:    de.Name(),
			Path:    entryRelPath,
			IsDir:   de.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
			Mode:    info.Mode().String(),
		})
	}

	return &DirectoryListing{
		Path:    clean,
		Entries: entries,
	}, nil
}

// FileInfo returns metadata for a single file or directory.
func FileInfo(folderRoot, relPath string) (*FileEntry, error) {
	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") {
		return nil, &os.PathError{Op: "stat", Path: relPath, Err: os.ErrPermission}
	}

	absPath := filepath.Join(folderRoot, filepath.FromSlash(clean))
	if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(folderRoot)) {
		return nil, &os.PathError{Op: "stat", Path: relPath, Err: os.ErrPermission}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	return &FileEntry{
		Name:    info.Name(),
		Path:    clean,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime().UnixMilli(),
		Mode:    info.Mode().String(),
	}, nil
}

// SearchResult holds a file match.
type SearchResult struct {
	FileEntry
	FolderID string `json:"folderId"`
}

// SearchFiles walks the folder looking for files whose name contains the query.
// maxResults caps the returned results.
func SearchFiles(folderRoot, query string, maxResults int) ([]FileEntry, error) {
	if maxResults <= 0 {
		maxResults = 100
	}
	query = strings.ToLower(query)

	var results []FileEntry
	err := filepath.WalkDir(folderRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(results) >= maxResults {
			return filepath.SkipAll
		}

		if strings.Contains(strings.ToLower(d.Name()), query) {
			rel, relErr := filepath.Rel(folderRoot, path)
			if relErr != nil {
				return nil
			}
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			results = append(results, FileEntry{
				Name:    d.Name(),
				Path:    filepath.ToSlash(rel),
				IsDir:   d.IsDir(),
				Size:    info.Size(),
				ModTime: info.ModTime().UnixMilli(),
				Mode:    info.Mode().String(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// IsPreviewable returns true if the file extension is an image type
// that can be served as a preview.
func IsPreviewable(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return true
	}
	return false
}

// FormatModTime returns a human-readable time string.
func FormatModTime(unixMilli int64) string {
	return time.UnixMilli(unixMilli).Format(time.RFC3339)
}
