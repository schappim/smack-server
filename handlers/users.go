package handlers

import (
	"encoding/json"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
)

type UserHandler struct {
	store *store.Store
}

func NewUserHandler(s *store.Store) *UserHandler {
	return &UserHandler{store: s}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	responses := make([]models.UserResponse, len(users))
	for i, u := range users {
		responses[i] = u.ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.ToResponse())
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	user, err := h.store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.ToResponse())
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req struct {
		DisplayName *string `json:"display_name,omitempty"`
		AvatarURL   *string `json:"avatar_url,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DisplayName != nil {
		if err := h.store.UpdateUserDisplayName(userID, *req.DisplayName); err != nil {
			http.Error(w, "Failed to update display name", http.StatusInternalServerError)
			return
		}
	}

	if req.AvatarURL != nil {
		if err := h.store.UpdateUserAvatar(userID, *req.AvatarURL); err != nil {
			http.Error(w, "Failed to update avatar", http.StatusInternalServerError)
			return
		}
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.ToResponse())
}

func (h *UserHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	prefs, err := h.store.GetAllUserPreferences(userID)
	if err != nil {
		http.Error(w, "Failed to fetch preferences", http.StatusInternalServerError)
		return
	}
	if prefs == nil {
		prefs = []models.UserPreference{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

func (h *UserHandler) GetPreference(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Preference key required", http.StatusBadRequest)
		return
	}

	value, err := h.store.GetUserPreference(userID, key)
	if err != nil {
		http.Error(w, "Preference not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.UserPreference{Key: key, Value: value})
}

func (h *UserHandler) SetPreference(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.SetPreferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	if err := h.store.SetUserPreference(userID, req.Key, req.Value); err != nil {
		http.Error(w, "Failed to save preference", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.UserPreference{Key: req.Key, Value: req.Value})
}

func (h *UserHandler) DeletePreference(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Preference key required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteUserPreference(userID, key); err != nil {
		http.Error(w, "Failed to delete preference", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate status
	validStatuses := map[string]bool{"online": true, "away": true, "offline": true, "dnd": true}
	if !validStatuses[req.Status] {
		http.Error(w, "Invalid status. Use: online, away, offline, or dnd", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateUserStatus(userID, req.Status); err != nil {
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": req.Status})
}
