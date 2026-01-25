package handlers

import (
	"encoding/json"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
)

type ReactionHandler struct {
	store *store.Store
	hub   *Hub
}

func NewReactionHandler(s *store.Store, hub *Hub) *ReactionHandler {
	return &ReactionHandler{store: s, hub: hub}
}

func (h *ReactionHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.AddReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.MessageID == "" || req.Emoji == "" {
		http.Error(w, "Message ID and emoji are required", http.StatusBadRequest)
		return
	}

	// Get the message to find the channel
	msg, err := h.store.GetMessage(req.MessageID)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Add the reaction
	reaction, err := h.store.AddReaction(req.MessageID, userID, req.Emoji)
	if err != nil {
		http.Error(w, "Failed to add reaction", http.StatusInternalServerError)
		return
	}

	// Get updated reactions for the message
	reactions, err := h.store.GetReactionsForMessage(req.MessageID)
	if err != nil {
		reactions = []models.ReactionGroup{}
	}

	// Broadcast to WebSocket clients
	if h.hub != nil {
		h.hub.BroadcastToChannel(msg.ChannelID, models.WSMessage{
			Type: models.WSTypeReactionUpdate,
			Payload: map[string]interface{}{
				"message_id": req.MessageID,
				"reactions":  reactions,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reaction)
}

func (h *ReactionHandler) Remove(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.RemoveReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.MessageID == "" || req.Emoji == "" {
		http.Error(w, "Message ID and emoji are required", http.StatusBadRequest)
		return
	}

	// Get the message to find the channel
	msg, err := h.store.GetMessage(req.MessageID)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Remove the reaction
	err = h.store.RemoveReaction(req.MessageID, userID, req.Emoji)
	if err != nil {
		http.Error(w, "Failed to remove reaction", http.StatusInternalServerError)
		return
	}

	// Get updated reactions for the message
	reactions, err := h.store.GetReactionsForMessage(req.MessageID)
	if err != nil {
		reactions = []models.ReactionGroup{}
	}

	// Broadcast to WebSocket clients
	if h.hub != nil {
		h.hub.BroadcastToChannel(msg.ChannelID, models.WSMessage{
			Type: models.WSTypeReactionUpdate,
			Payload: map[string]interface{}{
				"message_id": req.MessageID,
				"reactions":  reactions,
			},
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ReactionHandler) GetForMessage(w http.ResponseWriter, r *http.Request) {
	messageID := r.PathValue("id")
	if messageID == "" {
		http.Error(w, "Message ID required", http.StatusBadRequest)
		return
	}

	reactions, err := h.store.GetReactionsForMessage(messageID)
	if err != nil {
		http.Error(w, "Failed to get reactions", http.StatusInternalServerError)
		return
	}

	if reactions == nil {
		reactions = []models.ReactionGroup{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reactions)
}
