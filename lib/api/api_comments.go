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

	"github.com/syncthing/syncthing/lib/comments"
)

func (s *service) registerCommentEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/comments", s.getComments)
	mux.HandlerFunc(http.MethodPost, "/rest/comments", s.postComment)
	mux.HandlerFunc(http.MethodDelete, "/rest/comments/:id", s.deleteComment)
}

func (s *service) getComments(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	folderID := q.Get("folder")
	filePath := q.Get("path")
	if folderID == "" {
		http.Error(w, "folder parameter required", http.StatusBadRequest)
		return
	}

	if s.permManager != nil && !s.permManager.CanReadFolder(folderID, user.ID, user.IsAdmin()) {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	limit := 50
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	list, err := s.commentStore.ListComments(folderID, filePath, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	count, _ := s.commentStore.CountComments(folderID, filePath)

	if list == nil {
		list = []comments.Comment{}
	}

	sendJSON(w, map[string]interface{}{
		"comments": list,
		"total":    count,
	})
}

func (s *service) postComment(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var c comments.Comment
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if c.FolderID == "" || c.Content == "" {
		http.Error(w, "folder and content are required", http.StatusBadRequest)
		return
	}

	if s.permManager != nil && !s.permManager.CanReadFolder(c.FolderID, user.ID, user.IsAdmin()) {
		http.Error(w, "No permission", http.StatusForbidden)
		return
	}

	c.UserID = user.ID
	c.Username = user.Username

	if err := s.commentStore.CreateComment(&c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.notifManager != nil {
		s.notifManager.RecordActivity(user.ID, c.FolderID, "comment", c.FilePath, c.Content)
	}

	sendJSON(w, c)
}

func (s *service) deleteComment(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	existing, err := s.commentStore.GetComment(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "Comment not found", http.StatusNotFound)
		return
	}

	if existing.UserID != user.ID && !user.IsAdmin() {
		http.Error(w, "Can only delete your own comments", http.StatusForbidden)
		return
	}

	if err := s.commentStore.DeleteComment(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
