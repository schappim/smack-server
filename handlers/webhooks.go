package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"time"
)

type WebhookHandler struct {
	store *store.Store
	hub   *Hub
}

func NewWebhookHandler(s *store.Store, h *Hub) *WebhookHandler {
	return &WebhookHandler{store: s, hub: h}
}

// Create creates a new webhook for a channel
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, `{"error":"Name is required"}`, http.StatusBadRequest)
		return
	}

	if req.ChannelID == "" {
		http.Error(w, `{"error":"Channel ID is required"}`, http.StatusBadRequest)
		return
	}

	// Verify channel exists
	channel, err := h.store.GetChannel(req.ChannelID)
	if err != nil {
		http.Error(w, `{"error":"Channel not found"}`, http.StatusNotFound)
		return
	}

	// Don't allow webhooks on DM channels
	if channel.IsDirect {
		http.Error(w, `{"error":"Cannot create webhooks for direct message channels"}`, http.StatusBadRequest)
		return
	}

	webhook, err := h.store.CreateWebhook(req.Name, req.ChannelID, userID)
	if err != nil {
		http.Error(w, `{"error":"Failed to create webhook"}`, http.StatusInternalServerError)
		return
	}

	response := models.WebhookResponse{
		ID:        webhook.ID,
		Name:      webhook.Name,
		ChannelID: webhook.ChannelID,
		Token:     webhook.Token,
		CreatedBy: webhook.CreatedBy,
		CreatedAt: webhook.CreatedAt,
		URL:       fmt.Sprintf("/api/webhooks/incoming/%s/%s", webhook.ID, webhook.Token),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// List returns webhooks, optionally filtered by channel
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	channelID := r.URL.Query().Get("channel_id")

	var webhooks []models.Webhook
	var err error

	if channelID != "" {
		webhooks, err = h.store.GetWebhooksForChannel(channelID)
	} else {
		webhooks, err = h.store.GetWebhooksByUser(userID)
	}

	if err != nil {
		http.Error(w, `{"error":"Failed to fetch webhooks"}`, http.StatusInternalServerError)
		return
	}

	if webhooks == nil {
		webhooks = []models.Webhook{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhooks)
}

// Get returns a single webhook
func (h *WebhookHandler) Get(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("id")

	if webhookID == "" {
		http.Error(w, `{"error":"Webhook ID required"}`, http.StatusBadRequest)
		return
	}

	webhook, err := h.store.GetWebhook(webhookID)
	if err != nil {
		http.Error(w, `{"error":"Webhook not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhook)
}

// Delete removes a webhook
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	webhookID := r.PathValue("id")

	if webhookID == "" {
		http.Error(w, `{"error":"Webhook ID required"}`, http.StatusBadRequest)
		return
	}

	err := h.store.DeleteWebhook(webhookID, userID)
	if err != nil {
		http.Error(w, `{"error":"Failed to delete webhook"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "webhook deleted"})
}

// Incoming handles incoming webhook payloads (public endpoint)
func (h *WebhookHandler) Incoming(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("id")
	token := r.PathValue("token")

	if webhookID == "" || token == "" {
		http.Error(w, `{"error":"Invalid webhook URL"}`, http.StatusBadRequest)
		return
	}

	// Validate webhook and token
	webhook, err := h.store.GetWebhookByToken(webhookID, token)
	if err != nil {
		http.Error(w, `{"error":"Webhook not found or invalid token"}`, http.StatusNotFound)
		return
	}

	var req models.IncomingWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Content == "" && req.HTML == "" {
		http.Error(w, `{"error":"Content or HTML is required"}`, http.StatusBadRequest)
		return
	}

	// Get or create a webhook user for posting messages
	webhookUserID := "webhook-" + webhook.ID
	username := req.Username
	if username == "" {
		username = webhook.Name
	}

	// Ensure webhook user exists
	h.store.EnsureBotUser(webhookUserID, "webhook-"+webhook.Name, username, req.AvatarURL)

	// Create the message with optional HTML content
	var htmlContent *string
	if req.HTML != "" {
		htmlContent = &req.HTML
	}
	var widgetSize *string
	if req.WidgetSize != "" {
		widgetSize = &req.WidgetSize
	}
	content := req.Content
	if content == "" {
		content = "[HTML Widget]"
	}
	msg, err := h.store.CreateMessageWithHTML(webhook.ChannelID, webhookUserID, content, htmlContent, widgetSize, nil)
	if err != nil {
		http.Error(w, `{"error":"Failed to create message"}`, http.StatusInternalServerError)
		return
	}

	// Get the user for response
	user, _ := h.store.GetUserByID(webhookUserID)
	var userResponse models.UserResponse
	if user != nil {
		userResponse = user.ToResponse()
	} else {
		userResponse = models.UserResponse{
			ID:          webhookUserID,
			Username:    "webhook-" + webhook.Name,
			DisplayName: username,
		}
	}

	msgWithUser := models.MessageWithUser{
		Message: *msg,
		User:    userResponse,
	}

	// Broadcast the message via WebSocket
	h.hub.BroadcastToChannel(webhook.ChannelID, models.WSMessage{
		Type:    models.WSTypeNewMessage,
		Payload: msgWithUser,
	})

	// Return simplified response
	response := map[string]interface{}{
		"id":         msg.ID,
		"channel_id": msg.ChannelID,
		"content":    msg.Content,
		"created_at": msg.CreatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
