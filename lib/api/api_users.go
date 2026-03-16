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
	"github.com/syncthing/syncthing/lib/users"
)

func (s *service) registerUserEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/users", requireAdmin(s.getUsers))
	mux.HandlerFunc(http.MethodPost, "/rest/users", requireAdmin(s.postUser))
	mux.HandlerFunc(http.MethodGet, "/rest/system/user", s.getCurrentUser)
	mux.HandlerFunc(http.MethodGet, "/rest/storage/usage", s.getStorageUsage)
	mux.HandlerFunc(http.MethodPost, "/rest/storage/recalculate", s.postStorageRecalculate)
	mux.HandlerFunc(http.MethodGet, "/rest/users/:id", s.getUserByID)
	mux.HandlerFunc(http.MethodPut, "/rest/users/:id", s.putUser)
	mux.HandlerFunc(http.MethodDelete, "/rest/users/:id", requireAdmin(s.deleteUser))
	mux.HandlerFunc(http.MethodPost, "/rest/users/:id/password", s.postUserPassword)
}

func (s *service) getUsers(w http.ResponseWriter, _ *http.Request) {
	userList, err := s.userManager.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, userList)
}

func (s *service) postUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = users.RoleUser
	}

	user, err := s.userManager.CreateUser(req.Username, req.Email, req.Password, req.Role)
	if err != nil {
		if err == users.ErrUserExists {
			http.Error(w, "Username already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	sendJSON(w, user)
}

func (s *service) getUserByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	caller := userFromRequest(r)
	if caller == nil {
		forbidden(w)
		return
	}

	// Non-admins can only view their own profile
	if !caller.IsAdmin() && caller.ID != id {
		forbidden(w)
		return
	}

	user, err := s.userManager.GetUser(id)
	if err != nil {
		if err == users.ErrUserNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, user)
}

func (s *service) putUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	caller := userFromRequest(r)
	if caller == nil {
		forbidden(w)
		return
	}

	// Non-admins can only update their own profile (and cannot change role)
	if !caller.IsAdmin() && caller.ID != id {
		forbidden(w)
		return
	}

	user, err := s.userManager.GetUser(id)
	if err != nil {
		if err == users.ErrUserNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var req struct {
		Email      *string `json:"email"`
		Role       *string `json:"role"`
		Status     *string `json:"status"`
		QuotaBytes *int64  `json:"quotaBytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Role != nil {
		if !caller.IsAdmin() {
			http.Error(w, "Only admins can change roles", http.StatusForbidden)
			return
		}
		user.Role = *req.Role
	}
	if req.Status != nil {
		if !caller.IsAdmin() {
			http.Error(w, "Only admins can change status", http.StatusForbidden)
			return
		}
		user.Status = *req.Status
	}
	if req.QuotaBytes != nil {
		if !caller.IsAdmin() {
			http.Error(w, "Only admins can change quotas", http.StatusForbidden)
			return
		}
		user.QuotaBytes = *req.QuotaBytes
	}

	if err := s.userManager.UpdateUser(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, user)
}

func (s *service) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	caller := userFromRequest(r)
	if caller != nil && caller.ID == id {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	if err := s.userManager.DeleteUser(id); err != nil {
		if err == users.ErrUserNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *service) postUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	caller := userFromRequest(r)
	if caller == nil {
		forbidden(w)
		return
	}

	// Non-admins can only change their own password
	if !caller.IsAdmin() && caller.ID != id {
		forbidden(w)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}

	if err := s.userManager.SetPassword(id, req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *service) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		forbidden(w)
		return
	}
	sendJSON(w, user)
}

func (s *service) getStorageUsage(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		forbidden(w)
		return
	}

	status, err := s.userManager.GetStorageStatus(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, status)
}

func (s *service) postStorageRecalculate(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		forbidden(w)
		return
	}

	if user.IsAdmin() {
		if err := s.userManager.RecalculateAllUsage(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if _, err := s.userManager.RecalculateUsage(user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	status, err := s.userManager.GetStorageStatus(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, status)
}

func parseUserID(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())
	idStr := params.ByName("id")
	return strconv.ParseInt(idStr, 10, 64)
}
