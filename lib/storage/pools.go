// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package storage

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	StatusOnline   = "online"
	StatusDegraded = "degraded"
	StatusOffline  = "offline"

	StrategyManual     = "manual"
	StrategyRoundRobin = "round-robin"
	StrategyFillFirst  = "fill-first"
)

// Pool represents a storage location that can hold user data.
type Pool struct {
	ID         int64  `json:"id" db:"id"`
	Name       string `json:"name" db:"name"`
	Path       string `json:"path" db:"path"`
	TotalBytes int64  `json:"totalBytes" db:"total_bytes"`
	UsedBytes  int64  `json:"usedBytes" db:"used_bytes"`
	Status     string `json:"status" db:"status"`
	Strategy   string `json:"strategy" db:"strategy"`
	CreatedAt  int64  `json:"createdAt" db:"created_at"`
}

// UserPoolAssignment maps a user to a specific storage pool.
type UserPoolAssignment struct {
	UserID int64 `json:"userId" db:"user_id"`
	PoolID int64 `json:"poolId" db:"pool_id"`
}

// Store defines persistence for storage pool management.
type Store interface {
	ListPools() ([]Pool, error)
	GetPool(id int64) (*Pool, error)
	CreatePool(p *Pool) error
	UpdatePool(p *Pool) error
	DeletePool(id int64) error
	UpdatePoolUsage(id int64, usedBytes int64) error

	GetUserPool(userID int64) (*UserPoolAssignment, error)
	SetUserPool(userID, poolID int64) error
	RemoveUserPool(userID int64) error
	ListPoolUsers(poolID int64) ([]UserPoolAssignment, error)
}

// CheckHealth probes a pool's path to determine its status.
func CheckHealth(p *Pool) string {
	info, err := os.Stat(p.Path)
	if err != nil {
		return StatusOffline
	}
	if !info.IsDir() {
		return StatusOffline
	}

	testFile := filepath.Join(p.Path, ".pool_health_check")
	f, err := os.Create(testFile)
	if err != nil {
		return StatusDegraded
	}
	f.Close()
	os.Remove(testFile)

	return StatusOnline
}

// CalculateUsage walks a pool path and returns total bytes used.
func CalculateUsage(poolPath string) (int64, error) {
	var total int64
	err := filepath.WalkDir(poolPath, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// SelectPool picks a pool for a new user based on the given strategy.
func SelectPool(pools []Pool, strategy string) *Pool {
	available := make([]Pool, 0)
	for _, p := range pools {
		if p.Status == StatusOnline {
			available = append(available, p)
		}
	}
	if len(available) == 0 {
		return nil
	}

	switch strategy {
	case StrategyFillFirst:
		sort.Slice(available, func(i, j int) bool {
			return available[i].UsedBytes < available[j].UsedBytes
		})
		return &available[0]

	case StrategyRoundRobin:
		sort.Slice(available, func(i, j int) bool {
			return available[i].UsedBytes < available[j].UsedBytes
		})
		return &available[0]

	default:
		return &available[0]
	}
}

// InitPoolsFromEnv reads ST_STORAGE_POOLS env var and returns pool paths.
func InitPoolsFromEnv() []string {
	val := os.Getenv("ST_STORAGE_POOLS")
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	var paths []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}
