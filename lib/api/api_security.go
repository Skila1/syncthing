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

	"github.com/syncthing/syncthing/lib/audit"
	"github.com/syncthing/syncthing/lib/encryption"
)

func (s *service) registerSecurityEndpoints(mux *httprouter.Router) {
	// Audit log (admin only)
	mux.HandlerFunc(http.MethodGet, "/rest/audit", requireAdmin(s.getAuditLog))
	mux.HandlerFunc(http.MethodGet, "/rest/audit/export", requireAdmin(s.exportAuditLog))

	// IP restrictions (admin only)
	mux.HandlerFunc(http.MethodGet, "/rest/ip-restrictions", requireAdmin(s.getIPRestrictions))
	mux.HandlerFunc(http.MethodPost, "/rest/ip-restrictions", requireAdmin(s.postIPRestriction))
	mux.HandlerFunc(http.MethodDelete, "/rest/ip-restrictions/:id", requireAdmin(s.deleteIPRestriction))

	// Encryption key management
	mux.HandlerFunc(http.MethodGet, "/rest/encryption/status", s.getEncryptionStatus)
	mux.HandlerFunc(http.MethodPost, "/rest/encryption/enroll", s.postEncryptionEnroll)
	mux.HandlerFunc(http.MethodPost, "/rest/admin/encryption/recover/:userId", requireAdmin(s.postEncryptionRecover))
}

// --- Audit Log ---

func (s *service) getAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := audit.Filter{
		Limit:  100,
		Offset: 0,
	}

	if v := q.Get("userId"); v != "" {
		if uid, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.UserID = uid
		}
	}
	if v := q.Get("action"); v != "" {
		filter.Action = v
	}
	if v := q.Get("targetType"); v != "" {
		filter.TargetType = v
	}
	if v := q.Get("since"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.Since = ts
		}
	}
	if v := q.Get("before"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.Before = ts
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	entries, err := s.auditStore.ListAudit(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	count, _ := s.auditStore.CountAudit(filter)

	if entries == nil {
		entries = []audit.Entry{}
	}

	sendJSON(w, map[string]interface{}{
		"entries": entries,
		"total":   count,
	})
}

func (s *service) exportAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := audit.Filter{Limit: 10000}

	if v := q.Get("since"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.Since = ts
		}
	}
	if v := q.Get("before"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.Before = ts
		}
	}

	entries, err := s.auditStore.ListAudit(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}

	format := q.Get("format")
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=audit_log.csv")
		w.Write([]byte("id,user_id,action,target_type,target_id,ip_address,details,created_at\n"))
		for _, e := range entries {
			line := strconv.FormatInt(e.ID, 10) + "," +
				strconv.FormatInt(e.UserID, 10) + "," +
				e.Action + "," +
				e.TargetType + "," +
				e.TargetID + "," +
				e.IPAddress + "," +
				"\"" + e.DetailsJSON + "\"," +
				strconv.FormatInt(e.CreatedAt, 10) + "\n"
			w.Write([]byte(line))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_log.json")
	json.NewEncoder(w).Encode(entries)
}

// --- IP Restrictions ---

func (s *service) getIPRestrictions(w http.ResponseWriter, r *http.Request) {
	rules, err := s.ipRestrictionStore.ListAllRestrictions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []audit.IPRestriction{}
	}
	sendJSON(w, rules)
}

func (s *service) postIPRestriction(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	var rule audit.IPRestriction
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if rule.CIDR == "" {
		http.Error(w, "cidr is required", http.StatusBadRequest)
		return
	}
	if rule.Action != "allow" && rule.Action != "deny" {
		rule.Action = "deny"
	}

	if err := s.ipRestrictionStore.CreateRestriction(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.auditLogger != nil && user != nil {
		s.auditLogger.LogRequest(user.ID, audit.ActionIPRuleCreate, audit.TargetIPRule,
			strconv.FormatInt(rule.ID, 10), r, map[string]interface{}{
				"cidr": rule.CIDR, "action": rule.Action,
			})
	}

	sendJSON(w, rule)
}

func (s *service) deleteIPRestriction(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := s.ipRestrictionStore.DeleteRestriction(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.auditLogger != nil && user != nil {
		s.auditLogger.LogRequest(user.ID, audit.ActionIPRuleDelete, audit.TargetIPRule,
			strconv.FormatInt(id, 10), r, nil)
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Encryption ---

func (s *service) getEncryptionStatus(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rec, err := s.encryptionStore.GetEncryptionKey(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, map[string]interface{}{
		"enrolled":  rec != nil,
		"hasEscrow": rec != nil && len(rec.EscrowKey) > 0,
	})
}

func (s *service) postEncryptionEnroll(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	existing, err := s.encryptionStore.GetEncryptionKey(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "Already enrolled", http.StatusConflict)
		return
	}

	var req struct {
		Password    string `json:"password"`
		AdminSecret string `json:"adminSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}

	rec, err := encryption.EnrollUser(user.ID, req.Password, req.AdminSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.encryptionStore.SetEncryptionKey(rec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.auditLogger != nil {
		s.auditLogger.LogRequest(user.ID, "encryption_enroll", audit.TargetUser,
			strconv.FormatInt(user.ID, 10), r, nil)
	}

	sendJSON(w, map[string]interface{}{
		"enrolled":  true,
		"hasEscrow": len(rec.EscrowKey) > 0,
	})
}

func (s *service) postEncryptionRecover(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	uid, err := strconv.ParseInt(params.ByName("userId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	rec, err := s.encryptionStore.GetEncryptionKey(uid)
	if err != nil || rec == nil {
		http.Error(w, "No encryption key found", http.StatusNotFound)
		return
	}
	if len(rec.EscrowKey) == 0 {
		http.Error(w, "No escrow key available", http.StatusNotFound)
		return
	}

	var req struct {
		AdminSecret string `json:"adminSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err = encryption.RecoverFromEscrow(rec.EscrowKey, req.AdminSecret)
	if err != nil {
		http.Error(w, "Recovery failed -- wrong admin secret", http.StatusForbidden)
		return
	}

	sendJSON(w, map[string]interface{}{"recovered": true})
}
