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

	"github.com/syncthing/syncthing/lib/notifications"
)

func (s *service) registerNotificationEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/activity", s.getActivity)
	mux.HandlerFunc(http.MethodGet, "/rest/notifications", s.getNotifications)
	mux.HandlerFunc(http.MethodGet, "/rest/notifications/count", s.getNotificationCount)
	mux.HandlerFunc(http.MethodPut, "/rest/notifications/:id/read", s.putNotificationRead)
	mux.HandlerFunc(http.MethodPost, "/rest/notifications/read-all", s.postNotificationsReadAll)
	mux.HandlerFunc(http.MethodGet, "/rest/notification-prefs", s.getNotificationPrefs)
	mux.HandlerFunc(http.MethodPost, "/rest/notification-prefs", s.postNotificationPref)
	mux.HandlerFunc(http.MethodDelete, "/rest/notification-prefs/:id", s.deleteNotificationPref)
}

func (s *service) getActivity(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	folderID := r.URL.Query().Get("folder")
	action := r.URL.Query().Get("action")
	limit := intParam(r, "limit", 50)
	offset := intParam(r, "offset", 0)

	entries, err := s.notifManager.ListActivity(user.ID, folderID, action, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []notifications.ActivityEntry{}
	}
	sendJSON(w, entries)
}

func (s *service) getNotifications(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	unreadOnly := r.URL.Query().Get("unread") == "true"
	limit := intParam(r, "limit", 50)
	offset := intParam(r, "offset", 0)

	items, err := s.notifManager.ListNotifications(user.ID, unreadOnly, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []notifications.Notification{}
	}
	sendJSON(w, items)
}

func (s *service) getNotificationCount(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	count, err := s.notifManager.CountUnread(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, map[string]int{"unread": count})
}

func (s *service) putNotificationRead(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}

	if err := s.notifManager.MarkRead(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *service) postNotificationsReadAll(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := s.notifManager.MarkAllRead(user.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *service) getNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	prefs, err := s.notifManager.GetPrefs(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if prefs == nil {
		prefs = []notifications.NotificationPref{}
	}
	sendJSON(w, prefs)
}

func (s *service) postNotificationPref(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var pref notifications.NotificationPref
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	pref.UserID = user.ID

	if err := s.notifManager.SetPref(&pref); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, pref)
}

func (s *service) deleteNotificationPref(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid preference ID", http.StatusBadRequest)
		return
	}

	if err := s.notifManager.DeletePref(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func intParam(r *http.Request, name string, defaultVal int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
