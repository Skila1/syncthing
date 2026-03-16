// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package audit

import (
	"net"
)

// IPRestriction represents a stored IP allow/deny rule.
type IPRestriction struct {
	ID          int64  `json:"id" db:"id"`
	UserID      int64  `json:"userId" db:"user_id"`
	CIDR        string `json:"cidr" db:"cidr"`
	Action      string `json:"action" db:"action"`
	Description string `json:"description" db:"description"`
	CreatedAt   int64  `json:"createdAt" db:"created_at"`
}

// IPRestrictionStore defines persistence for IP restrictions.
type IPRestrictionStore interface {
	ListRestrictions(userID int64) ([]IPRestriction, error)
	ListAllRestrictions() ([]IPRestriction, error)
	CreateRestriction(r *IPRestriction) error
	DeleteRestriction(id int64) error
}

// IPRule represents a single allow/deny CIDR rule for evaluation.
type IPRule struct {
	CIDR   string
	Action string // "allow" or "deny"
}

// CheckIPAccess evaluates a list of IP rules against a client IP.
// Rules are evaluated in order: first match wins.
// If rules contain any "allow" rules, the default is deny (whitelist mode).
// If rules contain only "deny" rules, the default is allow (blacklist mode).
// If no rules exist, access is always allowed.
func CheckIPAccess(rules []IPRule, clientIP string) bool {
	if len(rules) == 0 {
		return true
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	hasAllowRules := false
	for _, rule := range rules {
		if rule.Action == "allow" {
			hasAllowRules = true
			break
		}
	}

	for _, rule := range rules {
		_, cidrNet, err := net.ParseCIDR(rule.CIDR)
		if err != nil {
			if net.ParseIP(rule.CIDR) != nil && rule.CIDR == clientIP {
				return rule.Action == "allow"
			}
			continue
		}
		if cidrNet.Contains(ip) {
			return rule.Action == "allow"
		}
	}

	if hasAllowRules {
		return false
	}
	return true
}
