// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DuplicateGroup represents a set of files sharing the same content hash.
type DuplicateGroup struct {
	Hash        string   `json:"hash"`
	Size        int64    `json:"size"`
	Files       []string `json:"files"`
	WastedBytes int64    `json:"wastedBytes"`
}

// ScanResult holds the output of a deduplication scan.
type ScanResult struct {
	Groups       []DuplicateGroup `json:"groups"`
	TotalWasted  int64            `json:"totalWasted"`
	FilesScanned int              `json:"filesScanned"`
}

// ScanForDuplicates walks the given root and identifies files with identical
// content hashes. Only files above minSize bytes are considered.
func ScanForDuplicates(root string, minSize int64) (*ScanResult, error) {
	hashMap := make(map[string][]fileEntry)
	scanned := 0

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == ".stversions" || name == ".sttrash" {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() < minSize {
			return nil
		}

		scanned++
		h, err := hashFile(path)
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		hashMap[h] = append(hashMap[h], fileEntry{
			Path: filepath.ToSlash(rel),
			Size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	var groups []DuplicateGroup
	var totalWasted int64

	for hash, entries := range hashMap {
		if len(entries) < 2 {
			continue
		}
		size := entries[0].Size
		wasted := size * int64(len(entries)-1)
		totalWasted += wasted

		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.Path
		}
		groups = append(groups, DuplicateGroup{
			Hash:        hash,
			Size:        size,
			Files:       paths,
			WastedBytes: wasted,
		})
	}

	return &ScanResult{
		Groups:       groups,
		TotalWasted:  totalWasted,
		FilesScanned: scanned,
	}, nil
}

// ResolveDuplicates keeps the first file in a group and replaces the rest
// with hardlinks (or removes them if hardlinking fails).
func ResolveDuplicates(root string, group DuplicateGroup, action string) error {
	if len(group.Files) < 2 {
		return nil
	}

	keepPath := filepath.Join(root, filepath.FromSlash(group.Files[0]))

	for _, rel := range group.Files[1:] {
		absPath := filepath.Join(root, filepath.FromSlash(rel))

		switch action {
		case "hardlink":
			if err := os.Remove(absPath); err != nil {
				continue
			}
			os.Link(keepPath, absPath)

		case "delete":
			os.Remove(absPath)

		default:
			// "keep" -- do nothing
		}
	}
	return nil
}

type fileEntry struct {
	Path string
	Size int64
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
