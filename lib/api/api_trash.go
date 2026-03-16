// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/syncthing/syncthing/lib/trash"
)

func (s *service) registerTrashEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/trash", s.getTrash)
	mux.HandlerFunc(http.MethodPost, "/rest/trash/restore", s.postTrashRestore)
	mux.HandlerFunc(http.MethodDelete, "/rest/trash/item", s.deleteTrashItem)
	mux.HandlerFunc(http.MethodPost, "/rest/trash/empty", s.postTrashEmpty)

	mux.HandlerFunc(http.MethodGet, "/rest/versions", s.getVersions)
	mux.HandlerFunc(http.MethodPost, "/rest/versions/restore", s.postVersionRestore)
	mux.HandlerFunc(http.MethodGet, "/rest/versions/download", s.getVersionDownload)

	mux.HandlerFunc(http.MethodGet, "/rest/cleanup-policies", s.getCleanupPolicies)
	mux.HandlerFunc(http.MethodPost, "/rest/cleanup-policies", s.postCleanupPolicy)
	mux.HandlerFunc(http.MethodDelete, "/rest/cleanup-policies/:id", s.deleteCleanupPolicy)
	mux.HandlerFunc(http.MethodGet, "/rest/admin/cleanup-policies", requireAdmin(s.getAdminCleanupPolicies))
}

// --- Trash endpoints ---

func (s *service) getTrash(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	root := s.folderRoot(w, r, folderID, false)
	if root == "" {
		return
	}

	entries, err := trash.ListTrash(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []trash.TrashEntry{}
	}
	sendJSON(w, entries)
}

func (s *service) postTrashRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FolderID string `json:"folderId"`
		TrashID  string `json:"trashId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	root := s.folderRoot(w, r, req.FolderID, true)
	if root == "" {
		return
	}

	if err := trash.RestoreFromTrash(root, req.TrashID); err != nil {
		if err == trash.ErrTrashItemNotFound {
			http.Error(w, "Trash item not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := userFromRequest(r)
	if s.userManager != nil && user != nil {
		if _, err := s.userManager.RecalculateUsage(user.ID); err != nil {
			slog.Error("Failed to recalculate usage after restore", "error", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *service) deleteTrashItem(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	trashID := r.URL.Query().Get("id")

	root := s.folderRoot(w, r, folderID, true)
	if root == "" {
		return
	}

	if err := trash.PermanentDelete(root, trashID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := userFromRequest(r)
	if s.userManager != nil && user != nil {
		if _, err := s.userManager.RecalculateUsage(user.ID); err != nil {
			slog.Error("Failed to recalculate usage after permanent delete", "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *service) postTrashEmpty(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FolderID string `json:"folderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	root := s.folderRoot(w, r, req.FolderID, true)
	if root == "" {
		return
	}

	if err := trash.EmptyTrash(root); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := userFromRequest(r)
	if s.userManager != nil && user != nil {
		if _, err := s.userManager.RecalculateUsage(user.ID); err != nil {
			slog.Error("Failed to recalculate usage after empty trash", "error", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// --- Version endpoints ---

func (s *service) getVersions(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relPath := r.URL.Query().Get("path")

	root := s.folderRoot(w, r, folderID, false)
	if root == "" {
		return
	}

	versions, err := trash.ListVersions(root, relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if versions == nil {
		versions = []trash.VersionEntry{}
	}
	sendJSON(w, versions)
}

func (s *service) postVersionRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FolderID string `json:"folderId"`
		Path     string `json:"path"`
		Version  string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	root := s.folderRoot(w, r, req.FolderID, true)
	if root == "" {
		return
	}

	if err := trash.RestoreVersion(root, req.Path, req.Version); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *service) getVersionDownload(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relPath := r.URL.Query().Get("path")
	version := r.URL.Query().Get("version")

	root := s.folderRoot(w, r, folderID, false)
	if root == "" {
		return
	}

	versions, err := trash.ListVersions(root, relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, v := range versions {
		if v.Version == version {
			w.Header().Set("Content-Disposition", "attachment; filename=\""+v.Version+"-"+r.URL.Query().Get("path")+"\"")
			http.ServeFile(w, r, v.StorePath)
			return
		}
	}

	http.Error(w, "Version not found", http.StatusNotFound)
}

// --- Cleanup policy endpoints ---

func (s *service) getCleanupPolicies(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if s.cleanupStore == nil {
		sendJSON(w, []trash.CleanupPolicy{})
		return
	}

	policies, err := s.cleanupStore.GetCleanupPolicies(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if policies == nil {
		policies = []trash.CleanupPolicy{}
	}
	sendJSON(w, policies)
}

func (s *service) postCleanupPolicy(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var policy trash.CleanupPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !user.IsAdmin() {
		policy.UserID = user.ID
	}

	if s.cleanupStore == nil {
		http.Error(w, "Cleanup not configured", http.StatusInternalServerError)
		return
	}

	if err := s.cleanupStore.SetCleanupPolicy(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, policy)
}

func (s *service) deleteCleanupPolicy(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	idStr := params.ByName("id")

	var id int64
	if _, err := json.Number(idStr).Int64(); err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}
	id, _ = json.Number(idStr).Int64()

	if s.cleanupStore == nil {
		http.Error(w, "Cleanup not configured", http.StatusInternalServerError)
		return
	}

	if err := s.cleanupStore.DeleteCleanupPolicy(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *service) getAdminCleanupPolicies(w http.ResponseWriter, r *http.Request) {
	if s.cleanupStore == nil {
		sendJSON(w, []trash.CleanupPolicy{})
		return
	}

	policies, err := s.cleanupStore.GetAllCleanupPolicies()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if policies == nil {
		policies = []trash.CleanupPolicy{}
	}
	sendJSON(w, policies)
}
