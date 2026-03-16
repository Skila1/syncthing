// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/syncthing/syncthing/lib/provisioning"
	"github.com/syncthing/syncthing/lib/rand"
	"github.com/syncthing/syncthing/lib/syncext"
	"github.com/syncthing/syncthing/lib/ur"
)

func (s *service) registerAdminPlatformEndpoints(mux *httprouter.Router) {
	// Analytics
	mux.HandlerFunc(http.MethodGet, "/rest/admin/analytics", requireAdmin(s.getAdminAnalytics))

	// Health
	mux.HandlerFunc(http.MethodGet, "/rest/admin/health", requireAdmin(s.getAdminHealth))

	// Cross-user devices
	mux.HandlerFunc(http.MethodGet, "/rest/admin/devices", requireAdmin(s.getAdminDevices))

	// Invite links
	mux.HandlerFunc(http.MethodGet, "/rest/admin/invites", requireAdmin(s.getInvites))
	mux.HandlerFunc(http.MethodPost, "/rest/admin/invites", requireAdmin(s.postInvite))
	mux.HandlerFunc(http.MethodDelete, "/rest/admin/invites/:id", requireAdmin(s.deleteInvite))

	// CSV bulk import
	mux.HandlerFunc(http.MethodPost, "/rest/admin/users/import", requireAdmin(s.postBulkImport))

	// Alert rules
	mux.HandlerFunc(http.MethodGet, "/rest/admin/alerts", requireAdmin(s.getAlertRules))
	mux.HandlerFunc(http.MethodPost, "/rest/admin/alerts", requireAdmin(s.postAlertRule))
	mux.HandlerFunc(http.MethodDelete, "/rest/admin/alerts/:id", requireAdmin(s.deleteAlertRule))

	// Global bandwidth ceiling
	mux.HandlerFunc(http.MethodGet, "/rest/admin/bandwidth-ceiling", requireAdmin(s.getBandwidthCeiling))
	mux.HandlerFunc(http.MethodPut, "/rest/admin/bandwidth-ceiling", requireAdmin(s.putBandwidthCeiling))
}

// --- Analytics ---

func (s *service) getAdminAnalytics(w http.ResponseWriter, r *http.Request) {
	users, err := s.userManager.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var totalUsed, totalQuota int64
	type userStat struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		UsedBytes int64  `json:"usedBytes"`
		Quota     int64  `json:"quotaBytes"`
	}
	perUser := make([]userStat, 0, len(users))
	for _, u := range users {
		totalUsed += u.UsedBytes
		totalQuota += u.QuotaBytes
		perUser = append(perUser, userStat{
			ID:        u.ID,
			Username:  u.Username,
			UsedBytes: u.UsedBytes,
			Quota:     u.QuotaBytes,
		})
	}

	sendJSON(w, map[string]interface{}{
		"totalUsers":   len(users),
		"totalUsed":    totalUsed,
		"totalQuota":   totalQuota,
		"perUser":      perUser,
		"totalFolders": len(s.cfg.Folders()),
		"totalDevices": len(s.cfg.Devices()),
		"generatedAt":  time.Now().UnixMilli(),
	})
}

// --- Health ---

func (s *service) getAdminHealth(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	sendJSON(w, map[string]interface{}{
		"goroutines":   runtime.NumGoroutine(),
		"heapAlloc":    mem.HeapAlloc,
		"heapSys":      mem.HeapSys,
		"heapInuse":    mem.HeapInuse,
		"stackInuse":   mem.StackInuse,
		"numGC":        mem.NumGC,
		"goVersion":    runtime.Version(),
		"numCPU":       runtime.NumCPU(),
		"totalFolders": len(s.cfg.Folders()),
		"totalDevices": len(s.cfg.Devices()),
		"uptime":       time.Since(ur.StartTime).Milliseconds(),
		"generatedAt":  time.Now().UnixMilli(),
	})
}

// --- Cross-user devices ---

func (s *service) getAdminDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.cfg.Devices()
	type deviceInfo struct {
		DeviceID string `json:"deviceId"`
		Name     string `json:"name"`
		Paused   bool   `json:"paused"`
	}
	result := make([]deviceInfo, 0, len(devices))
	for _, d := range devices {
		result = append(result, deviceInfo{
			DeviceID: d.DeviceID.String(),
			Name:     d.Name,
			Paused:   d.Paused,
		})
	}
	sendJSON(w, result)
}

// --- Invites ---

func (s *service) getInvites(w http.ResponseWriter, r *http.Request) {
	list, err := s.provisioningStore.ListInvites()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []provisioning.InviteLink{}
	}
	sendJSON(w, list)
}

func (s *service) postInvite(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	var inv provisioning.InviteLink
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	inv.Token = rand.String(32)
	if inv.Role == "" {
		inv.Role = "user"
	}
	if user != nil {
		inv.CreatedBy = user.ID
	}
	if err := s.provisioningStore.CreateInvite(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, inv)
}

func (s *service) deleteInvite(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := s.provisioningStore.DeleteInvite(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- CSV bulk import ---

func (s *service) postBulkImport(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	csvUsers, err := provisioning.ParseCSV(file)
	if err != nil {
		http.Error(w, "CSV parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	var created, failed int
	for _, cu := range csvUsers {
		if cu.Username == "" || cu.Password == "" {
			failed++
			continue
		}
		_, err := s.userManager.CreateUser(cu.Username, "", cu.Password, cu.Role)
		if err != nil {
			failed++
			continue
		}
		created++
	}

	sendJSON(w, map[string]interface{}{
		"created": created,
		"failed":  failed,
		"total":   len(csvUsers),
	})
}

// --- Alert rules ---

func (s *service) getAlertRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.provisioningStore.ListAlertRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []provisioning.AlertRule{}
	}
	sendJSON(w, rules)
}

func (s *service) postAlertRule(w http.ResponseWriter, r *http.Request) {
	var ar provisioning.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&ar); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if ar.Metric == "" {
		http.Error(w, "metric is required", http.StatusBadRequest)
		return
	}
	if err := s.provisioningStore.CreateAlertRule(&ar); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, ar)
}

func (s *service) deleteAlertRule(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := s.provisioningStore.DeleteAlertRule(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Global bandwidth ceiling ---

func (s *service) getBandwidthCeiling(w http.ResponseWriter, r *http.Request) {
	if s.syncExtAdminCeiling != nil {
		sendJSON(w, s.syncExtAdminCeiling)
	} else {
		sendJSON(w, map[string]int{"maxSendKbps": 0, "maxRecvKbps": 0})
	}
}

func (s *service) putBandwidthCeiling(w http.ResponseWriter, r *http.Request) {
	var ceiling struct {
		MaxSendKbps int `json:"maxSendKbps"`
		MaxRecvKbps int `json:"maxRecvKbps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&ceiling); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if s.syncExtAdminCeiling == nil {
		s.syncExtAdminCeiling = &syncext.UserBandwidth{}
	}
	s.syncExtAdminCeiling.MaxSendKbps = ceiling.MaxSendKbps
	s.syncExtAdminCeiling.MaxRecvKbps = ceiling.MaxRecvKbps
	sendJSON(w, ceiling)
}
