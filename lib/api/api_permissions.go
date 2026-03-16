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

	"github.com/julienschmidt/httprouter"
	"github.com/syncthing/syncthing/lib/permissions"
)

func (s *service) registerPermissionEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/folders/permissions", s.getUserFolderPermissions)
	mux.HandlerFunc(http.MethodGet, "/rest/devices/ownership", s.getUserDeviceOwnership)
	mux.HandlerFunc(http.MethodGet, "/rest/folders/:id/permissions", requireAdmin(s.getFolderPermissionsFunc))
	mux.HandlerFunc(http.MethodPut, "/rest/folders/:id/permissions", s.putFolderPermissionFunc)
	mux.HandlerFunc(http.MethodDelete, "/rest/folders/:id/permissions/:userId", s.deleteFolderPermissionFunc)
	mux.HandlerFunc(http.MethodPost, "/rest/devices/:id/claim", s.postClaimDeviceFunc)
	mux.HandlerFunc(http.MethodPost, "/rest/devices/:id/approve", requireAdmin(s.postApproveDeviceFunc))
	mux.HandlerFunc(http.MethodDelete, "/rest/devices/:id/ownership", requireAdmin(s.deleteDeviceOwnershipFunc))
}

// getUserFolderPermissions returns all folders accessible by the current user.
func (s *service) getUserFolderPermissions(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		forbidden(w)
		return
	}

	if user.IsAdmin() {
		// Admins see all folders
		sendJSON(w, s.cfg.FolderList())
		return
	}

	perms, err := s.permManager.GetUserFolders(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, perms)
}

// getFolderPermissionsFunc returns all permissions for a specific folder.
func (s *service) getFolderPermissionsFunc(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	folderID := params.ByName("id")
	perms, err := s.permManager.GetFolderPermissions(folderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, perms)
}

// putFolderPermissionFunc sets a user's permission on a folder.
func (s *service) putFolderPermissionFunc(w http.ResponseWriter, r *http.Request) {
	caller := userFromRequest(r)
	if caller == nil {
		forbidden(w)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	folderID := params.ByName("id")

	if !caller.IsAdmin() && !s.permManager.IsOwner(folderID, caller.ID, false) {
		forbidden(w)
		return
	}

	var req struct {
		UserID     int64  `json:"userId"`
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Permission != permissions.PermOwner && req.Permission != permissions.PermReadWrite && req.Permission != permissions.PermRead {
		http.Error(w, "Invalid permission level (must be owner, readwrite, or read)", http.StatusBadRequest)
		return
	}

	if err := s.permManager.SetPermission(folderID, req.UserID, req.Permission); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteFolderPermissionFunc removes a user's access to a folder.
func (s *service) deleteFolderPermissionFunc(w http.ResponseWriter, r *http.Request) {
	caller := userFromRequest(r)
	if caller == nil {
		forbidden(w)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	folderID := params.ByName("id")

	if !caller.IsAdmin() && !s.permManager.IsOwner(folderID, caller.ID, false) {
		forbidden(w)
		return
	}

	userID, err := parseIDParam(params.ByName("userId"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := s.permManager.RemovePermission(folderID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getUserDeviceOwnership returns all devices owned by the current user.
func (s *service) getUserDeviceOwnership(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		forbidden(w)
		return
	}

	devices, err := s.permManager.GetUserDevices(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, devices)
}

// postClaimDeviceFunc assigns a device to the current user.
func (s *service) postClaimDeviceFunc(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		forbidden(w)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	deviceID := params.ByName("id")
	if err := s.permManager.ClaimDevice(deviceID, user.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// postApproveDeviceFunc marks a device as approved (admin only).
func (s *service) postApproveDeviceFunc(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	deviceID := params.ByName("id")
	if err := s.permManager.ApproveDevice(deviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteDeviceOwnershipFunc releases device ownership (admin only).
func (s *service) deleteDeviceOwnershipFunc(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	deviceID := params.ByName("id")
	if err := s.permManager.ReleaseDevice(deviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseIDParam(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	return id, err
}
