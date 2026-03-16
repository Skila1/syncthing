// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"github.com/syncthing/syncthing/lib/filebrowser"
	"github.com/syncthing/syncthing/lib/trash"
)

func (s *service) registerFileBrowserEndpoints(mux *httprouter.Router) {
	mux.HandlerFunc(http.MethodGet, "/rest/files/browse", s.getFileBrowse)
	mux.HandlerFunc(http.MethodGet, "/rest/files/info", s.getFileInfo)
	mux.HandlerFunc(http.MethodGet, "/rest/files/download", s.getFileDownload)
	mux.HandlerFunc(http.MethodGet, "/rest/files/preview", s.getFilePreview)
	mux.HandlerFunc(http.MethodPost, "/rest/files/upload", s.postFileUpload)
	mux.HandlerFunc(http.MethodPost, "/rest/files/mkdir", s.postFileMkdir)
	mux.HandlerFunc(http.MethodDelete, "/rest/files/delete", s.deleteFile)
	mux.HandlerFunc(http.MethodGet, "/rest/files/search", s.getFileSearch)
}

// folderRoot resolves the filesystem root for a folder, checking permissions.
// Returns the folder root path or writes an error and returns "".
func (s *service) folderRoot(w http.ResponseWriter, r *http.Request, folderID string, needWrite bool) string {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return ""
	}

	if s.permManager != nil {
		if needWrite {
			if !s.permManager.CanWriteFolder(folderID, user.ID, user.IsAdmin()) {
				http.Error(w, "No write permission for this folder", http.StatusForbidden)
				return ""
			}
		} else {
			if !s.permManager.CanReadFolder(folderID, user.ID, user.IsAdmin()) {
				http.Error(w, "No read permission for this folder", http.StatusForbidden)
				return ""
			}
		}
	}

	folders := s.cfg.Folders()
	folder, ok := folders[folderID]
	if !ok {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return ""
	}
	return folder.Path
}

func (s *service) getFileBrowse(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relPath := r.URL.Query().Get("path")

	root := s.folderRoot(w, r, folderID, false)
	if root == "" {
		return
	}

	listing, err := filebrowser.ListDirectory(root, relPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Directory not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, listing)
}

func (s *service) getFileInfo(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relPath := r.URL.Query().Get("path")

	root := s.folderRoot(w, r, folderID, false)
	if root == "" {
		return
	}

	info, err := filebrowser.FileInfo(root, relPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSON(w, info)
}

func (s *service) getFileDownload(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relPath := r.URL.Query().Get("path")

	root := s.folderRoot(w, r, folderID, false)
	if root == "" {
		return
	}

	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(root, filepath.FromSlash(clean))
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(root)) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		http.Error(w, "Cannot download directory; use share link for folders", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(fullPath)+"\"")
	http.ServeFile(w, r, fullPath)
}

func (s *service) getFilePreview(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relPath := r.URL.Query().Get("path")

	root := s.folderRoot(w, r, folderID, false)
	if root == "" {
		return
	}

	clean := filepath.Clean(relPath)
	if strings.Contains(clean, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(root, filepath.FromSlash(clean))
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(root)) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if !filebrowser.IsPreviewable(fullPath) {
		http.Error(w, "File type not previewable", http.StatusBadRequest)
		return
	}

	http.ServeFile(w, r, fullPath)
}

func (s *service) postFileUpload(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relDir := r.URL.Query().Get("path")

	root := s.folderRoot(w, r, folderID, true)
	if root == "" {
		return
	}

	user := userFromRequest(r)

	// 256 MB max in-memory; rest spills to temp files
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		http.Error(w, "Failed to parse upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	cleanDir := filepath.Clean(relDir)
	if cleanDir == "." {
		cleanDir = ""
	}
	if strings.Contains(cleanDir, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	destDir := filepath.Join(root, filepath.FromSlash(cleanDir))
	if !strings.HasPrefix(filepath.Clean(destDir), filepath.Clean(root)) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var uploaded []string
	for _, fheaders := range r.MultipartForm.File {
		for _, fh := range fheaders {
			// Quota check per file
			if s.userManager != nil && user != nil {
				if err := s.userManager.CheckQuota(user.ID, fh.Size); err != nil {
					http.Error(w, "Quota exceeded", http.StatusForbidden)
					return
				}
			}

			src, err := fh.Open()
			if err != nil {
				http.Error(w, "Failed to read file: "+err.Error(), http.StatusInternalServerError)
				return
			}

			safeName := filepath.Base(fh.Filename)
			destPath := filepath.Join(destDir, safeName)

			dst, err := os.Create(destPath)
			if err != nil {
				src.Close()
				http.Error(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
				return
			}

			written, err := io.Copy(dst, src)
			src.Close()
			dst.Close()

			if err != nil {
				http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Update used bytes after successful write
			if s.userManager != nil && user != nil {
				if err := s.userManager.RecalculateUsage(user.ID, root); err != nil {
					slog.Error("Failed to recalculate usage after upload", "error", err)
				}
			}

			slog.Info("File uploaded", "user", user.Username, "folder", folderID, "file", safeName, "bytes", written)
			uploaded = append(uploaded, safeName)
		}
	}

	sendJSON(w, map[string]interface{}{
		"uploaded": uploaded,
		"count":    len(uploaded),
	})
}

func (s *service) postFileMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FolderID string `json:"folderId"`
		Path     string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	root := s.folderRoot(w, r, req.FolderID, true)
	if root == "" {
		return
	}

	clean := filepath.Clean(req.Path)
	if strings.Contains(clean, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(root, filepath.FromSlash(clean))
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(root)) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(fullPath, 0o755); err != nil {
		http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *service) deleteFile(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	relPath := r.URL.Query().Get("path")

	root := s.folderRoot(w, r, folderID, true)
	if root == "" {
		return
	}

	clean := filepath.Clean(relPath)
	if clean == "." || clean == "" {
		http.Error(w, "Cannot delete folder root", http.StatusBadRequest)
		return
	}
	if strings.Contains(clean, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(root, filepath.FromSlash(clean))
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(root)) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if _, err := trash.MoveToTrash(root, clean); err != nil {
		http.Error(w, "Failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}

	user := userFromRequest(r)
	if s.userManager != nil && user != nil {
		if err := s.userManager.RecalculateUsage(user.ID, root); err != nil {
			slog.Error("Failed to recalculate usage after delete", "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *service) getFileSearch(w http.ResponseWriter, r *http.Request) {
	user := userFromRequest(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}

	folderID := r.URL.Query().Get("folder")
	maxStr := r.URL.Query().Get("max")
	maxResults := 100
	if maxStr != "" {
		if n, err := strconv.Atoi(maxStr); err == nil && n > 0 {
			maxResults = n
		}
	}
	if maxResults > 500 {
		maxResults = 500
	}

	type folderResult struct {
		FolderID    string                `json:"folderId"`
		FolderLabel string                `json:"folderLabel"`
		Results     []filebrowser.FileEntry `json:"results"`
	}

	var allResults []folderResult
	folders := s.cfg.Folders()

	searchInFolder := func(id string) {
		folder, ok := folders[id]
		if !ok {
			return
		}
		if s.permManager != nil && !s.permManager.CanReadFolder(id, user.ID, user.IsAdmin()) {
			return
		}
		results, err := filebrowser.SearchFiles(folder.Path, query, maxResults)
		if err != nil {
			slog.Error("Search error", "folder", id, "error", err)
			return
		}
		if len(results) > 0 {
			allResults = append(allResults, folderResult{
				FolderID:    id,
				FolderLabel: folder.Label,
				Results:     results,
			})
		}
	}

	if folderID != "" {
		searchInFolder(folderID)
	} else {
		for id := range folders {
			searchInFolder(id)
		}
	}

	sendJSON(w, allResults)
}
