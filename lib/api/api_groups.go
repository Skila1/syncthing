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

	"github.com/syncthing/syncthing/lib/groups"
)

func (s *service) registerGroupEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/groups", requireAdmin(s.getGroups))
	mux.HandlerFunc(http.MethodPost, "/rest/groups", requireAdmin(s.postGroup))
	mux.HandlerFunc(http.MethodPut, "/rest/groups/:id", requireAdmin(s.putGroup))
	mux.HandlerFunc(http.MethodDelete, "/rest/groups/:id", requireAdmin(s.deleteGroup))

	mux.HandlerFunc(http.MethodGet, "/rest/groups/:id/members", requireAdmin(s.getGroupMembers))
	mux.HandlerFunc(http.MethodPost, "/rest/groups/:id/members", requireAdmin(s.postGroupMember))
	mux.HandlerFunc(http.MethodDelete, "/rest/groups/:id/members/:userId", requireAdmin(s.deleteGroupMember))

	mux.HandlerFunc(http.MethodGet, "/rest/groups/:id/folders", requireAdmin(s.getGroupFolders))
	mux.HandlerFunc(http.MethodPost, "/rest/groups/:id/folders", requireAdmin(s.postGroupFolder))
	mux.HandlerFunc(http.MethodDelete, "/rest/groups/:id/folders/:folderId", requireAdmin(s.deleteGroupFolder))

	mux.HandlerFunc(http.MethodGet, "/rest/user/groups", s.getUserGroups)
}

// --- Group CRUD ---

func (s *service) getGroups(w http.ResponseWriter, r *http.Request) {
	list, err := s.groupManager.ListGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []groups.Group{}
	}
	sendJSON(w, list)
}

func (s *service) postGroup(w http.ResponseWriter, r *http.Request) {
	var g groups.Group
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if g.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if err := s.groupManager.CreateGroup(&g); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, g)
}

func (s *service) putGroup(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	var g groups.Group
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	g.ID = id
	if err := s.groupManager.UpdateGroup(&g); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, g)
}

func (s *service) deleteGroup(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	if err := s.groupManager.DeleteGroup(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Members ---

func (s *service) getGroupMembers(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	members, err := s.groupManager.ListMembers(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if members == nil {
		members = []groups.GroupMember{}
	}
	sendJSON(w, members)
}

func (s *service) postGroupMember(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	groupID, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	var req struct {
		UserID int64 `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.groupManager.AddMember(groupID, req.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *service) deleteGroupMember(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	groupID, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseInt(params.ByName("userId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	if err := s.groupManager.RemoveMember(groupID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Group Folders ---

func (s *service) getGroupFolders(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	folders, err := s.groupManager.ListGroupFolders(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if folders == nil {
		folders = []groups.GroupFolder{}
	}
	sendJSON(w, folders)
}

func (s *service) postGroupFolder(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	groupID, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	var gf groups.GroupFolder
	if err := json.NewDecoder(r.Body).Decode(&gf); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	gf.GroupID = groupID
	if gf.Permission == "" {
		gf.Permission = "read"
	}
	if err := s.groupManager.SetGroupFolder(&gf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, gf)
}

func (s *service) deleteGroupFolder(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	groupID, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}
	folderID := params.ByName("folderId")
	if err := s.groupManager.RemoveGroupFolder(groupID, folderID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- User's own groups ---

func (s *service) getUserGroups(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	list, err := s.groupManager.ListUserGroups(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []groups.Group{}
	}
	sendJSON(w, list)
}
