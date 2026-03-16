// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/syncthing/syncthing/lib/sharing"
)

func (s *service) registerShareEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/shares", s.getShareLinks)
	mux.HandlerFunc(http.MethodPost, "/rest/shares", s.postShareLink)
	mux.HandlerFunc(http.MethodDelete, "/rest/shares/:id", s.deleteShareLink)
	mux.HandlerFunc(http.MethodGet, "/rest/admin/shares", requireAdmin(s.getAllShareLinks))
	mux.HandlerFunc(http.MethodDelete, "/rest/admin/shares/:id", requireAdmin(s.adminDeleteShareLink))
}

func (s *service) getShareLinks(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	links, err := s.shareManager.ListLinks(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, links)
}

func (s *service) postShareLink(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		FolderID     string `json:"folderId"`
		FilePath     string `json:"filePath"`
		Password     string `json:"password"`
		ExpiryHours  int    `json:"expiryHours"`
		MaxDownloads int    `json:"maxDownloads"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FolderID == "" {
		http.Error(w, "folderId is required", http.StatusBadRequest)
		return
	}

	if _, ok := s.cfg.Folders()[req.FolderID]; !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	cleanPath := filepath.Clean(req.FilePath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	var expiry time.Duration
	if req.ExpiryHours > 0 {
		expiry = time.Duration(req.ExpiryHours) * time.Hour
		if expiry > time.Duration(sharing.MaxExpiryDays)*24*time.Hour {
			expiry = time.Duration(sharing.MaxExpiryDays) * 24 * time.Hour
		}
	} else {
		expiry = sharing.DefaultExpiry
	}

	link, err := s.shareManager.CreateLink(user.ID, req.FolderID, cleanPath, req.Password, expiry, req.MaxDownloads)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSON(w, link)
}

func (s *service) deleteShareLink(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	idStr := params.ByName("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid share link ID", http.StatusBadRequest)
		return
	}

	if err := s.shareManager.RevokeLink(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *service) getAllShareLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.shareManager.ListAllLinks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, links)
}

func (s *service) adminDeleteShareLink(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	idStr := params.ByName("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid share link ID", http.StatusBadRequest)
		return
	}

	if err := s.shareManager.RevokeLink(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getShareDownload serves the info/password prompt page for a share link (GET).
func (s *service) getShareDownload(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	token := params.ByName("token")

	link, err := s.shareManager.ValidateLink(token, "")
	if err != nil {
		if err == sharing.ErrInvalidPassword {
			sendJSON(w, map[string]interface{}{
				"passwordRequired": true,
				"fileName":         filepath.Base(link.FilePath),
				"folderId":         link.FolderID,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.serveSharedFile(w, r, link)
}

// postShareDownload handles password-protected link downloads (POST).
func (s *service) postShareDownload(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	token := params.ByName("token")

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	link, err := s.shareManager.ValidateLink(token, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	s.serveSharedFile(w, r, link)
}

func (s *service) serveSharedFile(w http.ResponseWriter, r *http.Request, link *sharing.ShareLink) {
	folders := s.cfg.Folders()
	folder, ok := folders[link.FolderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}

	fullPath := filepath.Join(folder.Path, filepath.FromSlash(link.FilePath))

	cleanedBase := filepath.Clean(folder.Path)
	cleanedFull := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanedFull, cleanedBase) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, "File access error", http.StatusInternalServerError)
		return
	}

	if err := s.shareManager.RecordDownload(link.Token); err != nil {
		slog.Error("Failed to record download", "error", err)
	}

	if info.IsDir() {
		s.serveDirectoryAsZip(w, fullPath, filepath.Base(fullPath))
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(fullPath)+"\"")
	http.ServeFile(w, r, fullPath)
}

func (s *service) serveDirectoryAsZip(w http.ResponseWriter, dirPath, dirName string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+dirName+".zip\"")

	zw := zip.NewWriter(w)
	defer zw.Close()

	baseDir := filepath.Clean(dirPath)
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(writer, f)
		return err
	})

	if err != nil {
		slog.Error("Failed to create zip archive", "error", err)
	}
}
