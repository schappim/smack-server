package handlers

import (
	"encoding/json"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"strings"
)

type ChannelHandler struct {
	store *store.Store
}

func NewChannelHandler(s *store.Store) *ChannelHandler {
	return &ChannelHandler{store: s}
}

func (h *ChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	channels, err := h.store.GetChannelsForUser(userID)
	if err != nil {
		http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
		return
	}

	if channels == nil {
		channels = []models.ChannelWithUnread{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func (h *ChannelHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	channels, err := h.store.GetPublicChannels()
	if err != nil {
		http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
		return
	}

	if channels == nil {
		channels = []models.Channel{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func (h *ChannelHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Channel name is required", http.StatusBadRequest)
		return
	}

	// Sanitize channel name
	req.Name = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))

	channel, err := h.store.CreateChannel(req.Name, req.Description, userID, false)
	if err != nil {
		http.Error(w, "Failed to create channel", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(channel)
}

func (h *ChannelHandler) Get(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	channel, err := h.store.GetChannel(channelID)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channel)
}

func (h *ChannelHandler) Update(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	channel, err := h.store.GetChannel(channelID)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	if channel.IsDirect {
		http.Error(w, "Cannot update direct message channels", http.StatusBadRequest)
		return
	}

	var req models.UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	name := channel.Name
	description := channel.Description
	if req.Name != "" {
		name = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}
	if req.Description != nil {
		description = *req.Description
	}

	err = h.store.UpdateChannel(channelID, name, description)
	if err != nil {
		http.Error(w, "Failed to update channel", http.StatusInternalServerError)
		return
	}

	updatedChannel, _ := h.store.GetChannel(channelID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedChannel)
}

func (h *ChannelHandler) Join(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	channelID := r.PathValue("id")

	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	channel, err := h.store.GetChannel(channelID)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	if channel.IsDirect {
		http.Error(w, "Cannot join direct message channel", http.StatusBadRequest)
		return
	}

	err = h.store.JoinChannel(channelID, userID)
	if err != nil {
		http.Error(w, "Failed to join channel", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "joined"})
}

func (h *ChannelHandler) Members(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	members, err := h.store.GetChannelMembers(channelID)
	if err != nil {
		http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
		return
	}

	responses := make([]models.UserResponse, len(members))
	for i, m := range members {
		responses[i] = m.ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (h *ChannelHandler) CreateDM(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if req.UserID == userID {
		http.Error(w, "Cannot create DM with yourself", http.StatusBadRequest)
		return
	}

	channel, err := h.store.GetOrCreateDMChannel(userID, req.UserID)
	if err != nil {
		http.Error(w, "Failed to create DM channel", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channel)
}

func (h *ChannelHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	channelID := r.PathValue("id")

	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	err := h.store.MarkChannelAsRead(channelID, userID)
	if err != nil {
		http.Error(w, "Failed to mark channel as read", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *ChannelHandler) Mute(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	channelID := r.PathValue("id")

	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	err := h.store.MuteChannel(userID, channelID)
	if err != nil {
		http.Error(w, "Failed to mute channel", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "muted"})
}

func (h *ChannelHandler) Unmute(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	channelID := r.PathValue("id")

	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	err := h.store.UnmuteChannel(userID, channelID)
	if err != nil {
		http.Error(w, "Failed to unmute channel", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "unmuted"})
}

func (h *ChannelHandler) GetMuted(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	channelIDs, err := h.store.GetMutedChannels(userID)
	if err != nil {
		http.Error(w, "Failed to get muted channels", http.StatusInternalServerError)
		return
	}

	if channelIDs == nil {
		channelIDs = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channelIDs)
}

func (h *ChannelHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	channelID := r.PathValue("id")

	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	// Check if it's the general channel (can't leave)
	channel, err := h.store.GetChannel(channelID)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	if channel.Name == "general" {
		http.Error(w, "Cannot leave the general channel", http.StatusBadRequest)
		return
	}

	err = h.store.LeaveChannel(channelID, userID)
	if err != nil {
		http.Error(w, "Failed to leave channel", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "left"})
}

func (h *ChannelHandler) Clear(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")

	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	err := h.store.ClearChannelMessages(channelID)
	if err != nil {
		http.Error(w, "Failed to clear channel messages", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}
