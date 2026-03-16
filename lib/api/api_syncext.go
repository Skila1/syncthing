// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/syncext"
)

func (s *service) registerSyncExtEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/sync/bandwidth", s.getBandwidth)
	mux.HandlerFunc(http.MethodPut, "/rest/sync/bandwidth", s.putBandwidth)
	mux.HandlerFunc(http.MethodGet, "/rest/admin/sync/bandwidth/:userId", requireAdmin(s.getAdminBandwidth))
	mux.HandlerFunc(http.MethodPut, "/rest/admin/sync/bandwidth/:userId", requireAdmin(s.putAdminBandwidth))

	mux.HandlerFunc(http.MethodGet, "/rest/sync/schedules", s.getSchedules)
	mux.HandlerFunc(http.MethodPost, "/rest/sync/schedules", s.postSchedule)
	mux.HandlerFunc(http.MethodDelete, "/rest/sync/schedules/:id", s.deleteSchedule)

	mux.HandlerFunc(http.MethodGet, "/rest/sync/conflicts", s.getConflicts)
	mux.HandlerFunc(http.MethodPost, "/rest/sync/conflicts/resolve", s.postResolveConflict)

	mux.HandlerFunc(http.MethodPost, "/rest/sync/pause/folder", s.postPauseFolder)
	mux.HandlerFunc(http.MethodPost, "/rest/sync/resume/folder", s.postResumeFolder)
}

// --- Bandwidth ---

func (s *service) getBandwidth(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	bw, err := s.syncExtStore.GetUserBandwidth(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, bw)
}

func (s *service) putBandwidth(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var bw syncext.UserBandwidth
	if err := json.NewDecoder(r.Body).Decode(&bw); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	bw.UserID = user.ID

	if s.syncExtAdminCeiling != nil {
		if s.syncExtAdminCeiling.MaxSendKbps > 0 && (bw.MaxSendKbps <= 0 || bw.MaxSendKbps > s.syncExtAdminCeiling.MaxSendKbps) {
			bw.MaxSendKbps = s.syncExtAdminCeiling.MaxSendKbps
		}
		if s.syncExtAdminCeiling.MaxRecvKbps > 0 && (bw.MaxRecvKbps <= 0 || bw.MaxRecvKbps > s.syncExtAdminCeiling.MaxRecvKbps) {
			bw.MaxRecvKbps = s.syncExtAdminCeiling.MaxRecvKbps
		}
	}

	if err := s.syncExtStore.SetUserBandwidth(&bw); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, bw)
}

func (s *service) getAdminBandwidth(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	uid, err := strconv.ParseInt(params.ByName("userId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	bw, err := s.syncExtStore.GetUserBandwidth(uid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, bw)
}

func (s *service) putAdminBandwidth(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	uid, err := strconv.ParseInt(params.ByName("userId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	var bw syncext.UserBandwidth
	if err := json.NewDecoder(r.Body).Decode(&bw); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	bw.UserID = uid
	if err := s.syncExtStore.SetUserBandwidth(&bw); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, bw)
}

// --- Schedules ---

func (s *service) getSchedules(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	schedules, err := s.syncExtStore.ListSchedules(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if schedules == nil {
		schedules = []syncext.SyncSchedule{}
	}
	sendJSON(w, schedules)
}

func (s *service) postSchedule(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var sched syncext.SyncSchedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	sched.UserID = user.ID
	if err := s.syncExtStore.SetSchedule(&sched); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, sched)
}

func (s *service) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}
	if err := s.syncExtStore.DeleteSchedule(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Conflicts ---

func (s *service) getConflicts(w http.ResponseWriter, r *http.Request) {
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

	conflicts, err := syncext.ListConflicts(folder.Path, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if conflicts == nil {
		conflicts = []syncext.ConflictFile{}
	}
	sendJSON(w, conflicts)
}

func (s *service) postResolveConflict(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FolderID     string `json:"folderId"`
		ConflictPath string `json:"conflictPath"`
		Action       string `json:"action"`
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

	if err := syncext.ResolveConflict(folder.Path, req.ConflictPath, req.Action); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// --- Folder pause/resume ---

func (s *service) postPauseFolder(w http.ResponseWriter, r *http.Request) {
	s.setFolderPaused(w, r, true)
}

func (s *service) postResumeFolder(w http.ResponseWriter, r *http.Request) {
	s.setFolderPaused(w, r, false)
}

func (s *service) setFolderPaused(w http.ResponseWriter, r *http.Request, paused bool) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		FolderID string `json:"folderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if s.permManager != nil && !s.permManager.CanWriteFolder(req.FolderID, user.ID, user.IsAdmin()) {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	_, err := s.cfg.Modify(func(cfg *config.Configuration) {
		for i := range cfg.Folders {
			if cfg.Folders[i].ID == req.FolderID {
				cfg.Folders[i].Paused = paused
				return
			}
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, map[string]bool{"paused": paused})
}
