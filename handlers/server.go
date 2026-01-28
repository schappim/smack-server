package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"smack-server/middleware"
	"smack-server/store"
)

type ServerHandler struct {
	store     *store.Store
	uploadDir string
}

type ServerInfo struct {
	Name      string `json:"name"`
	IconURL   string `json:"icon_url,omitempty"`
}

func NewServerHandler(s *store.Store, uploadDir string) *ServerHandler {
	return &ServerHandler{store: s, uploadDir: uploadDir}
}

func (h *ServerHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	_ = middleware.GetUserID(r) // auth required

	info := ServerInfo{}

	if name, err := h.store.GetServerSetting("name"); err == nil {
		info.Name = name
	} else {
		info.Name = "Smack Server"
	}

	if iconURL, err := h.store.GetServerSetting("icon_url"); err == nil {
		info.IconURL = iconURL
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (h *ServerHandler) UpdateInfo(w http.ResponseWriter, r *http.Request) {
	_ = middleware.GetUserID(r)

	var req struct {
		Name *string `json:"name,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		if err := h.store.SetServerSetting("name", *req.Name); err != nil {
			http.Error(w, "Failed to update", http.StatusInternalServerError)
			return
		}
	}

	h.GetInfo(w, r)
}

func (h *ServerHandler) UploadIcon(w http.ResponseWriter, r *http.Request) {
	_ = middleware.GetUserID(r)

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, "File too large (max 5MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("icon")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !isAllowedType(contentType) {
		http.Error(w, "File type not allowed", http.StatusBadRequest)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = getExtensionFromMime(contentType)
	}

	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	filename := "server-icon-" + hex.EncodeToString(randBytes) + ext

	destPath := filepath.Join(h.uploadDir, filename)
	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(destPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Remove old icon file if it exists
	if oldURL, err := h.store.GetServerSetting("icon_url"); err == nil {
		oldFilename := filepath.Base(oldURL)
		os.Remove(filepath.Join(h.uploadDir, oldFilename))
	}

	iconURL := "/api/files/" + filename
	if err := h.store.SetServerSetting("icon_url", iconURL); err != nil {
		http.Error(w, "Failed to save setting", http.StatusInternalServerError)
		return
	}

	info := ServerInfo{}
	if name, err := h.store.GetServerSetting("name"); err == nil {
		info.Name = name
	} else {
		info.Name = "Smack Server"
	}
	info.IconURL = iconURL

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (h *ServerHandler) DeleteIcon(w http.ResponseWriter, r *http.Request) {
	_ = middleware.GetUserID(r)

	if oldURL, err := h.store.GetServerSetting("icon_url"); err == nil {
		oldFilename := filepath.Base(oldURL)
		os.Remove(filepath.Join(h.uploadDir, oldFilename))
	}

	h.store.SetServerSetting("icon_url", "")

	info := ServerInfo{}
	if name, err := h.store.GetServerSetting("name"); err == nil {
		info.Name = name
	} else {
		info.Name = "Smack Server"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// Convenience helper for reading icon_url, exported for potential use by other handlers
func (h *ServerHandler) GetIconURL() string {
	if url, err := h.store.GetServerSetting("icon_url"); err == nil {
		return url
	}
	return ""
}

// GetName returns current server name
func (h *ServerHandler) GetName() string {
	if name, err := h.store.GetServerSetting("name"); err == nil && name != "" {
		return name
	}
	return "Smack Server"
}

// Minimal public info endpoint (no auth required) — just the server name and icon
func (h *ServerHandler) GetPublicInfo(w http.ResponseWriter, r *http.Request) {
	info := ServerInfo{}
	if name, err := h.store.GetServerSetting("name"); err == nil {
		info.Name = name
	} else {
		info.Name = "Smack Server"
	}
	if iconURL, err := h.store.GetServerSetting("icon_url"); err == nil {
		info.IconURL = iconURL
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
