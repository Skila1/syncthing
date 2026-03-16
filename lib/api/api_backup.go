// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/syncthing/syncthing/lib/backup"
	"github.com/syncthing/syncthing/lib/dedup"
)

func (s *service) registerBackupEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/backups", s.getBackups)
	mux.HandlerFunc(http.MethodPost, "/rest/backups", s.postBackup)
	mux.HandlerFunc(http.MethodDelete, "/rest/backups/:id", s.deleteBackup)
	mux.HandlerFunc(http.MethodPost, "/rest/backups/:id/restore", s.postRestoreBackup)

	mux.HandlerFunc(http.MethodGet, "/rest/snapshots", s.getSnapshots)
	mux.HandlerFunc(http.MethodPost, "/rest/snapshots", s.postSnapshot)
	mux.HandlerFunc(http.MethodDelete, "/rest/snapshots/:id", s.deleteSnapshot)
	mux.HandlerFunc(http.MethodPost, "/rest/snapshots/:id/restore", s.postRestoreSnapshot)

	mux.HandlerFunc(http.MethodGet, "/rest/duplicates", s.getDuplicates)
	mux.HandlerFunc(http.MethodPost, "/rest/duplicates/resolve", s.postResolveDuplicates)
}

// --- Backups ---

func (s *service) getBackups(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	list, err := s.backupStore.ListBackups(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []backup.Backup{}
	}
	sendJSON(w, list)
}

func (s *service) postBackup(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		FolderID string `json:"folderId"`
		Type     string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.FolderID == "" {
		http.Error(w, "folderId is required", http.StatusBadRequest)
		return
	}

	folders := s.cfg.Folders()
	folder, ok := folders[req.FolderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	if s.permManager != nil && !s.permManager.CanReadFolder(req.FolderID, user.ID, user.IsAdmin()) {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	bType := req.Type
	if bType == "" {
		bType = backup.TypeFull
	}

	b := &backup.Backup{
		UserID:    user.ID,
		FolderID:  req.FolderID,
		Type:      bType,
		Status:    backup.StatusRunning,
		StartedAt: time.Now().UnixMilli(),
	}
	if err := s.backupStore.CreateBackup(b); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		destPath := filepath.Join(folder.Path, ".stbackups", fmt.Sprintf("backup-%d-%d", b.ID, b.StartedAt))
		size, err := backup.RunFullBackup(folder.Path, destPath)
		if err != nil {
			b.Status = backup.StatusFailed
		} else {
			b.Status = backup.StatusCompleted
			b.SizeBytes = size
			b.BackupPath = destPath
		}
		b.CompletedAt = time.Now().UnixMilli()
		s.backupStore.UpdateBackup(b)
	}()

	sendJSON(w, b)
}

func (s *service) deleteBackup(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	b, err := s.backupStore.GetBackup(id)
	if err != nil || b == nil {
		http.Error(w, "Backup not found", http.StatusNotFound)
		return
	}
	if b.UserID != user.ID && !user.IsAdmin() {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	if b.BackupPath != "" {
		os.RemoveAll(b.BackupPath)
	}
	s.backupStore.DeleteBackup(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *service) postRestoreBackup(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	b, err := s.backupStore.GetBackup(id)
	if err != nil || b == nil {
		http.Error(w, "Backup not found", http.StatusNotFound)
		return
	}
	if b.UserID != user.ID && !user.IsAdmin() {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	folders := s.cfg.Folders()
	folder, ok := folders[b.FolderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	if err := backup.RestoreSnapshot(b.BackupPath, folder.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, map[string]string{"status": "restored"})
}

// --- Snapshots ---

func (s *service) getSnapshots(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	folderID := r.URL.Query().Get("folder")
	list, err := s.backupStore.ListSnapshots(user.ID, folderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []backup.Snapshot{}
	}
	sendJSON(w, list)
}

func (s *service) postSnapshot(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		FolderID string `json:"folderId"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.FolderID == "" {
		http.Error(w, "folderId is required", http.StatusBadRequest)
		return
	}

	folders := s.cfg.Folders()
	folder, ok := folders[req.FolderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	if s.permManager != nil && !s.permManager.CanReadFolder(req.FolderID, user.ID, user.IsAdmin()) {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	name := req.Name
	if name == "" {
		name = "snapshot"
	}

	snapshotDir := filepath.Join(folder.Path, ".stsnapshots")
	snap, err := backup.CreateFolderSnapshot(folder.Path, snapshotDir, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	snap.UserID = user.ID
	snap.FolderID = req.FolderID
	if err := s.backupStore.CreateSnapshot(snap); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, snap)
}

func (s *service) deleteSnapshot(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	snap, err := s.backupStore.GetSnapshot(id)
	if err != nil || snap == nil {
		http.Error(w, "Snapshot not found", http.StatusNotFound)
		return
	}
	if snap.UserID != user.ID && !user.IsAdmin() {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	if snap.SnapshotPath != "" {
		os.RemoveAll(snap.SnapshotPath)
	}
	s.backupStore.DeleteSnapshot(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *service) postRestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	snap, err := s.backupStore.GetSnapshot(id)
	if err != nil || snap == nil {
		http.Error(w, "Snapshot not found", http.StatusNotFound)
		return
	}
	if snap.UserID != user.ID && !user.IsAdmin() {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	folders := s.cfg.Folders()
	folder, ok := folders[snap.FolderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	if err := backup.RestoreSnapshot(snap.SnapshotPath, folder.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, map[string]string{"status": "restored"})
}

// --- Duplicates ---

func (s *service) getDuplicates(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	folderID := r.URL.Query().Get("folder")
	if folderID == "" {
		http.Error(w, "folder parameter required", http.StatusBadRequest)
		return
	}

	folders := s.cfg.Folders()
	folder, ok := folders[folderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	if s.permManager != nil && !s.permManager.CanReadFolder(folderID, user.ID, user.IsAdmin()) {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	result, err := dedup.ScanForDuplicates(folder.Path, 1024)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, result)
}

func (s *service) postResolveDuplicates(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		FolderID string               `json:"folderId"`
		Group    dedup.DuplicateGroup `json:"group"`
		Action   string               `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	folders := s.cfg.Folders()
	folder, ok := folders[req.FolderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	if s.permManager != nil && !s.permManager.CanWriteFolder(req.FolderID, user.ID, user.IsAdmin()) {
		http.Error(w, "No write permission", http.StatusForbidden)
		return
	}

	if err := dedup.ResolveDuplicates(folder.Path, req.Group, req.Action); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, map[string]string{"status": "resolved"})
}
