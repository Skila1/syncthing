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

	"github.com/syncthing/syncthing/lib/storage"
)

func (s *service) registerStoragePoolEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/storage/pools", requireAdmin(s.getStoragePools))
	mux.HandlerFunc(http.MethodPost, "/rest/storage/pools", requireAdmin(s.postStoragePool))
	mux.HandlerFunc(http.MethodPut, "/rest/storage/pools/:id", requireAdmin(s.putStoragePool))
	mux.HandlerFunc(http.MethodDelete, "/rest/storage/pools/:id", requireAdmin(s.deleteStoragePool))
	mux.HandlerFunc(http.MethodPost, "/rest/storage/pools/:id/refresh", requireAdmin(s.postRefreshPool))

	mux.HandlerFunc(http.MethodGet, "/rest/storage/pools/:id/users", requireAdmin(s.getPoolUsers))
	mux.HandlerFunc(http.MethodPost, "/rest/storage/pool-assign", requireAdmin(s.postPoolAssign))
	mux.HandlerFunc(http.MethodDelete, "/rest/storage/pool-assign/:userId", requireAdmin(s.deletePoolAssign))
}

func (s *service) getStoragePools(w http.ResponseWriter, r *http.Request) {
	pools, err := s.storagePoolStore.ListPools()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if pools == nil {
		pools = []storage.Pool{}
	}
	sendJSON(w, pools)
}

func (s *service) postStoragePool(w http.ResponseWriter, r *http.Request) {
	var p storage.Pool
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if p.Name == "" || p.Path == "" {
		http.Error(w, "name and path are required", http.StatusBadRequest)
		return
	}

	p.Status = storage.CheckHealth(&p)

	if err := s.storagePoolStore.CreatePool(&p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, p)
}

func (s *service) putStoragePool(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid pool ID", http.StatusBadRequest)
		return
	}
	var p storage.Pool
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	p.ID = id
	if err := s.storagePoolStore.UpdatePool(&p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, p)
}

func (s *service) deleteStoragePool(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid pool ID", http.StatusBadRequest)
		return
	}
	if err := s.storagePoolStore.DeletePool(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *service) postRefreshPool(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid pool ID", http.StatusBadRequest)
		return
	}
	pool, err := s.storagePoolStore.GetPool(id)
	if err != nil || pool == nil {
		http.Error(w, "Pool not found", http.StatusNotFound)
		return
	}

	pool.Status = storage.CheckHealth(pool)
	used, _ := storage.CalculateUsage(pool.Path)
	pool.UsedBytes = used

	s.storagePoolStore.UpdatePoolUsage(pool.ID, used)
	s.storagePoolStore.UpdatePool(pool)

	sendJSON(w, pool)
}

func (s *service) getPoolUsers(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid pool ID", http.StatusBadRequest)
		return
	}
	assignments, err := s.storagePoolStore.ListPoolUsers(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if assignments == nil {
		assignments = []storage.UserPoolAssignment{}
	}
	sendJSON(w, assignments)
}

func (s *service) postPoolAssign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int64 `json:"userId"`
		PoolID int64 `json:"poolId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.storagePoolStore.SetUserPool(req.UserID, req.PoolID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *service) deletePoolAssign(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	uid, err := strconv.ParseInt(params.ByName("userId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	if err := s.storagePoolStore.RemoveUserPool(uid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
