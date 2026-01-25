package handlers

import (
	"encoding/json"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
)

type BotHandler struct {
	store *store.Store
}

func NewBotHandler(s *store.Store) *BotHandler {
	return &BotHandler{store: s}
}

func (h *BotHandler) List(w http.ResponseWriter, r *http.Request) {
	bots, err := h.store.GetAllBots()
	if err != nil {
		http.Error(w, "Failed to fetch bots", http.StatusInternalServerError)
		return
	}

	if bots == nil {
		bots = []models.Bot{}
	}

	responses := make([]models.BotResponse, len(bots))
	for i, b := range bots {
		responses[i] = b.ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (h *BotHandler) Get(w http.ResponseWriter, r *http.Request) {
	botID := r.PathValue("id")
	if botID == "" {
		http.Error(w, "Bot ID required", http.StatusBadRequest)
		return
	}

	bot, err := h.store.GetBot(botID)
	if err != nil {
		http.Error(w, "Bot not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot.ToResponse())
}

func (h *BotHandler) CreateBotDM(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.CreateBotDMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.BotID == "" {
		http.Error(w, "Bot ID is required", http.StatusBadRequest)
		return
	}

	// Verify bot exists
	bot, err := h.store.GetBot(req.BotID)
	if err != nil {
		http.Error(w, "Bot not found", http.StatusNotFound)
		return
	}

	channel, err := h.store.GetOrCreateBotDMChannel(userID, req.BotID)
	if err != nil {
		http.Error(w, "Failed to create bot DM channel", http.StatusInternalServerError)
		return
	}

	// Return channel with bot's display name
	channel.Name = bot.DisplayName

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channel)
}
