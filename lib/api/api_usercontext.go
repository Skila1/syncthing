// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"context"
	"net/http"

	"github.com/syncthing/syncthing/lib/users"
)

type contextKey int

const userContextKey contextKey = iota

func withUser(r *http.Request, user *users.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

func userFromRequest(r *http.Request) *users.User {
	if u, ok := r.Context().Value(userContextKey).(*users.User); ok {
		return u
	}
	return nil
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := userFromRequest(r)
		if user == nil || !user.IsAdmin() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
