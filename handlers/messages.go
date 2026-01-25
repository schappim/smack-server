package handlers

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"smack-server/ai"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MessageHandler struct {
	store     *store.Store
	hub       *Hub
	botID     string
	aiClients map[string]*ai.OpenAIClient // provider -> client

	// Track last bot response per channel for auto-follow-up
	lastBotResponse   map[string]botResponseInfo
	lastBotResponseMu sync.RWMutex
}

// botResponseInfo tracks when a bot last responded in a channel
type botResponseInfo struct {
	bot       *models.Bot
	timestamp time.Time
}

var botResponses = []string{
	"That's interesting! Tell me more.",
	"I totally agree with you on that!",
	"Hmm, I hadn't thought about it that way before.",
	"Great point! 👍",
	"Ha! That's funny 😄",
	"I'm just a bot, but I appreciate the conversation!",
	"Absolutely! You're making a lot of sense.",
	"Let me think about that for a moment... okay, I agree!",
	"That's a solid take.",
	"You know what? You're right.",
	"Interesting perspective!",
	"I was just thinking the same thing!",
	"Keep going, I'm listening!",
	"That reminds me of something... but I forgot what. Bot memory issues.",
}

func NewMessageHandler(s *store.Store, hub *Hub) *MessageHandler {
	handler := &MessageHandler{
		store:           s,
		hub:             hub,
		aiClients:       make(map[string]*ai.OpenAIClient),
		lastBotResponse: make(map[string]botResponseInfo),
	}
	handler.ensureBotUser()
	return handler
}

func (h *MessageHandler) RegisterAIClient(provider, model string) {
	h.aiClients[provider] = ai.NewOpenAIClient(model)
}

func (h *MessageHandler) GetAIClients() map[string]*ai.OpenAIClient {
	return h.aiClients
}

func (h *MessageHandler) ensureBotUser() {
	// Check if bot user exists
	bot, err := h.store.GetUserByUsername("smackbot")
	if err != nil {
		// Create bot user
		bot, err = h.store.CreateUser("smackbot", "Smack Bot", "botpassword123!")
		if err != nil {
			return
		}
	}
	h.botID = bot.ID
	h.store.UpdateUserStatus(h.botID, "online")
}

func (h *MessageHandler) GetChannelMessages(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	if channelID == "" {
		http.Error(w, "Channel ID required", http.StatusBadRequest)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// Parse optional 'before' parameter for pagination (ISO8601 timestamp)
	var beforeTime *time.Time
	if before := r.URL.Query().Get("before"); before != "" {
		// Try parsing various timestamp formats
		formats := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.999999Z07:00",
			"2006-01-02T15:04:05Z",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, before); err == nil {
				beforeTime = &t
				break
			}
		}
	}

	var messages []models.MessageWithUser
	var err error

	if beforeTime != nil {
		messages, err = h.store.GetChannelMessagesBefore(channelID, limit, beforeTime)
	} else {
		messages, err = h.store.GetChannelMessages(channelID, limit)
	}

	if err != nil {
		log.Printf("Error fetching messages for channel %s: %v", channelID, err)
		http.Error(w, "Failed to fetch messages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if messages == nil {
		messages = []models.MessageWithUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChannelID == "" || req.Content == "" {
		http.Error(w, "Channel ID and content are required", http.StatusBadRequest)
		return
	}

	msg, err := h.store.CreateMessage(req.ChannelID, userID, req.Content, req.ThreadID)
	if err != nil {
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	user, _ := h.store.GetUserByID(userID)
	msgWithUser := models.MessageWithUser{
		Message: *msg,
		User:    user.ToResponse(),
	}

	// Broadcast to WebSocket clients
	if h.hub != nil {
		h.hub.BroadcastToChannel(req.ChannelID, models.WSMessage{
			Type:    models.WSTypeNewMessage,
			Payload: msgWithUser,
		})
	}

	// Check if this is a bot DM channel
	isBotChannel := h.store.IsBotChannel(req.ChannelID)
	log.Printf("[MESSAGE] Channel %s - isBotChannel: %v, userID: %s, botID: %s", req.ChannelID, isBotChannel, userID, h.botID)
	if isBotChannel {
		log.Printf("[BOT] Triggering AI bot response for channel %s", req.ChannelID)
		go h.sendAIBotResponse(req.ChannelID, req.Content, req.ThreadID)
	} else {
		// Check for @bot mentions in regular channels
		if mentionedBot := h.findMentionedBot(req.Content); mentionedBot != nil {
			log.Printf("[BOT] Bot %s mentioned in channel %s", mentionedBot.Name, req.ChannelID)
			go h.sendMentionedBotResponse(req.ChannelID, req.Content, req.ThreadID, mentionedBot, false)
		} else if followUpBot := h.checkAutoFollowUp(req.ChannelID); followUpBot != nil {
			// Auto-follow-up: bot responded within last minute, respond without @mention
			log.Printf("[BOT] Auto-follow-up for bot %s in channel %s", followUpBot.Name, req.ChannelID)
			go h.sendMentionedBotResponse(req.ChannelID, req.Content, req.ThreadID, followUpBot, true)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(msgWithUser)
}

func (h *MessageHandler) sendBotResponse(channelID, userMessage string, threadID *string) {
	// Wait a bit to seem more natural
	delay := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
	time.Sleep(delay)

	// Generate response
	var response string
	lowerMsg := strings.ToLower(userMessage)

	switch {
	case strings.Contains(lowerMsg, "hello") || strings.Contains(lowerMsg, "hi"):
		response = "Hey there! 👋 How's it going?"
	case strings.Contains(lowerMsg, "how are you"):
		response = "I'm doing great, thanks for asking! Just bot things, you know."
	case strings.Contains(lowerMsg, "help"):
		response = "I'm Smack Bot! I'm here to keep you company and make this chat less lonely. Just say hi!"
	case strings.Contains(lowerMsg, "bye") || strings.Contains(lowerMsg, "goodbye"):
		response = "See you later! 👋"
	case strings.Contains(lowerMsg, "?"):
		response = "Good question! Let me think... actually, I'm just a bot so I'll say: yes, probably!"
	default:
		response = botResponses[rand.Intn(len(botResponses))]
	}

	// Create bot message
	msg, err := h.store.CreateMessage(channelID, h.botID, response, threadID)
	if err != nil {
		return
	}

	bot, _ := h.store.GetUserByID(h.botID)
	msgWithUser := models.MessageWithUser{
		Message: *msg,
		User:    bot.ToResponse(),
	}

	// Broadcast bot response
	if h.hub != nil {
		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type:    models.WSTypeNewMessage,
			Payload: msgWithUser,
		})
	}
}

// checkAutoFollowUp checks if a bot responded in this channel within the last minute
func (h *MessageHandler) checkAutoFollowUp(channelID string) *models.Bot {
	h.lastBotResponseMu.RLock()
	defer h.lastBotResponseMu.RUnlock()

	if info, ok := h.lastBotResponse[channelID]; ok {
		if time.Since(info.timestamp) <= time.Minute {
			return info.bot
		}
	}
	return nil
}

// recordBotResponse records when a bot responded in a channel for auto-follow-up
func (h *MessageHandler) recordBotResponse(channelID string, bot *models.Bot) {
	h.lastBotResponseMu.Lock()
	defer h.lastBotResponseMu.Unlock()

	h.lastBotResponse[channelID] = botResponseInfo{
		bot:       bot,
		timestamp: time.Now(),
	}
}

// findMentionedBot checks if message contains @bot-name mention and returns the bot
func (h *MessageHandler) findMentionedBot(content string) *models.Bot {
	// Get all bots
	bots, err := h.store.GetAllBots()
	if err != nil {
		return nil
	}

	lowerContent := strings.ToLower(content)

	for _, bot := range bots {
		// Check for @bot-name or @displayname mentions
		botMention := "@bot-" + strings.ToLower(bot.Name)
		displayMention := "@" + strings.ToLower(bot.DisplayName)

		if strings.Contains(lowerContent, botMention) || strings.Contains(lowerContent, displayMention) {
			return &bot
		}
	}

	return nil
}

// sendMentionedBotResponse handles bot responses when mentioned in a channel
// isFollowUp indicates this is an auto-follow-up response (bot may choose to give brief/no response if not relevant)
func (h *MessageHandler) sendMentionedBotResponse(channelID, userMessage string, threadID *string, bot *models.Bot, isFollowUp bool) {
	// Get the AI client for this provider
	client, ok := h.aiClients[bot.Provider]
	if !ok {
		log.Printf("No AI client registered for provider: %s", bot.Provider)
		return
	}

	// Get the bot user from the database
	botUser, err := h.store.GetUserByID(bot.ID)
	if err != nil {
		log.Printf("Failed to get bot user: %v", err)
		return
	}

	// Get recent messages for context (excluding the current message which may not be visible yet)
	recentMessages, err := h.store.GetChannelMessages(channelID, 4)
	if err != nil {
		log.Printf("Failed to get recent messages for context: %v", err)
		recentMessages = []models.MessageWithUser{}
	}

	// Build conversation context - DB returns newest-first, reverse for chronological order
	var contextMessages []ai.InputMessage
	for i := len(recentMessages) - 1; i >= 0; i-- {
		msg := recentMessages[i]
		// Skip if this is the same message we're about to add (avoid duplicates)
		if msg.Content == userMessage {
			continue
		}
		role := "user"
		if msg.UserID == bot.ID {
			role = "assistant"
		}
		contextMessages = append(contextMessages, ai.NewTextMessage(role, msg.Content))
	}

	// Always add the current user message as the LAST message (ensures it's included)
	contextMessages = append(contextMessages, ai.NewTextMessage("user", userMessage))
	log.Printf("[BOT] Built context with %d messages for channel %s", len(contextMessages), channelID)

	systemPrompt := "You are " + bot.DisplayName + ", a helpful AI assistant in a team chat. Keep responses concise and helpful. You're responding in a channel where multiple people may be chatting."
	if isFollowUp {
		systemPrompt += " This is a follow-up message in an ongoing conversation. Only respond if the message is relevant to you or the conversation you were having. If the message is not directed at you or doesn't need your input, respond with just 'pass' (lowercase, nothing else)."
	}

	// Create placeholder message in database
	msg, err := h.store.CreateMessage(channelID, bot.ID, "", threadID)
	if err != nil {
		log.Printf("Failed to create bot message: %v", err)
		return
	}

	log.Printf("[BOT] Starting streaming response to channel %s", channelID)

	// Broadcast stream start
	if h.hub != nil {
		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type: models.WSTypeMessageStreamStart,
			Payload: models.StreamStartPayload{
				MessageID: msg.ID,
				ChannelID: channelID,
				User:      botUser.ToResponse(),
				ThreadID:  threadID,
			},
		})
	}

	// Stream AI response
	finalContent, err := client.StreamResponseWithContext(contextMessages, systemPrompt, func(delta, fullText string) {
		// Broadcast each delta
		if h.hub != nil {
			h.hub.BroadcastToChannel(channelID, models.WSMessage{
				Type: models.WSTypeMessageStreamDelta,
				Payload: models.StreamDeltaPayload{
					MessageID: msg.ID,
					ChannelID: channelID,
					Delta:     delta,
					FullText:  fullText,
				},
			})
		}
	})

	if err != nil {
		log.Printf("Failed to get AI response: %v", err)
		finalContent = "Sorry, I'm having trouble connecting right now. Please try again later."
	}

	// If this was a follow-up and the bot decided to pass, clean up and don't broadcast
	if isFollowUp && strings.TrimSpace(strings.ToLower(finalContent)) == "pass" {
		log.Printf("[BOT] Bot chose to pass on follow-up in channel %s", channelID)
		// Delete the placeholder message
		h.store.DeleteMessage(msg.ID)
		// Broadcast stream end with empty content to clean up client state
		if h.hub != nil {
			h.hub.BroadcastToChannel(channelID, models.WSMessage{
				Type: models.WSTypeMessageStreamEnd,
				Payload: models.StreamEndPayload{
					MessageID: msg.ID,
					ChannelID: channelID,
					Content:   "",
				},
			})
		}
		return
	}

	// Update message in database with final content
	if err := h.store.UpdateMessageContent(msg.ID, finalContent); err != nil {
		log.Printf("Failed to update bot message: %v", err)
	}
	msg.Content = finalContent

	// Broadcast stream end
	if h.hub != nil {
		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type: models.WSTypeMessageStreamEnd,
			Payload: models.StreamEndPayload{
				MessageID: msg.ID,
				ChannelID: channelID,
				Content:   finalContent,
			},
		})

		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type: models.WSTypeNewMessage,
			Payload: models.MessageWithUser{
				Message:    *msg,
				User:       botUser.ToResponse(),
				ReplyCount: 0,
				LatestReply: nil,
			},
		})
	}

	// Record this response for auto-follow-up feature
	h.recordBotResponse(channelID, bot)

	log.Printf("[BOT] Finished streaming response to channel %s", channelID)
}

func (h *MessageHandler) sendAIBotResponse(channelID, userMessage string, threadID *string) {
	// Get the bot for this channel
	bot, err := h.store.GetBotForChannel(channelID)
	if err != nil {
		log.Printf("Failed to get bot for channel %s: %v", channelID, err)
		return
	}

	// Get the AI client for this provider
	client, ok := h.aiClients[bot.Provider]
	if !ok {
		log.Printf("No AI client registered for provider: %s", bot.Provider)
		return
	}

	// Get the bot user from the database
	botUser, err := h.store.GetUserByID(bot.ID)
	if err != nil {
		log.Printf("Failed to get bot user: %v", err)
		return
	}

	// Get recent messages for context (excluding current message which may not be visible yet)
	recentMessages, err := h.store.GetChannelMessages(channelID, 29)
	if err != nil {
		log.Printf("Failed to get recent messages for context: %v", err)
		recentMessages = []models.MessageWithUser{}
	}

	// Build conversation context
	var contextMessages []ai.InputMessage

	// Add recent messages as context (in chronological order, oldest first)
	for i := len(recentMessages) - 1; i >= 0; i-- {
		msg := recentMessages[i]
		// Skip if this is the same message we're about to add (avoid duplicates)
		if msg.Content == userMessage {
			continue
		}
		role := "user"
		if msg.UserID == bot.ID {
			role = "assistant"
		}
		contextMessages = append(contextMessages, ai.NewTextMessage(role, msg.Content))
	}

	// Always add the current user message as the LAST message (ensures it's included)
	contextMessages = append(contextMessages, ai.NewTextMessage("user", userMessage))

	systemPrompt := "You are " + bot.DisplayName + ", a helpful AI assistant. Be concise and helpful in your responses."

	// Create placeholder message in database
	msg, err := h.store.CreateMessage(channelID, bot.ID, "", threadID)
	if err != nil {
		log.Printf("Failed to create bot message: %v", err)
		return
	}

	log.Printf("[BOT DM] Starting streaming response to channel %s", channelID)

	// Broadcast stream start
	if h.hub != nil {
		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type: models.WSTypeMessageStreamStart,
			Payload: models.StreamStartPayload{
				MessageID: msg.ID,
				ChannelID: channelID,
				User:      botUser.ToResponse(),
				ThreadID:  threadID,
			},
		})
	}

	// Stream AI response
	finalContent, err := client.StreamResponseWithContext(contextMessages, systemPrompt, func(delta, fullText string) {
		// Broadcast each delta
		if h.hub != nil {
			h.hub.BroadcastToChannel(channelID, models.WSMessage{
				Type: models.WSTypeMessageStreamDelta,
				Payload: models.StreamDeltaPayload{
					MessageID: msg.ID,
					ChannelID: channelID,
					Delta:     delta,
					FullText:  fullText,
				},
			})
		}
	})

	if err != nil {
		log.Printf("Failed to get AI response: %v", err)
		finalContent = "Sorry, I'm having trouble connecting right now. Please try again later."
	}

	// Update message in database with final content
	if err := h.store.UpdateMessageContent(msg.ID, finalContent); err != nil {
		log.Printf("Failed to update bot message: %v", err)
	}
	msg.Content = finalContent

	// Broadcast stream end
	if h.hub != nil {
		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type: models.WSTypeMessageStreamEnd,
			Payload: models.StreamEndPayload{
				MessageID: msg.ID,
				ChannelID: channelID,
				Content:   finalContent,
			},
		})

		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type: models.WSTypeNewMessage,
			Payload: models.MessageWithUser{
				Message:    *msg,
				User:       botUser.ToResponse(),
				ReplyCount: 0,
				LatestReply: nil,
			},
		})
	}

	log.Printf("[BOT DM] Finished streaming response to channel %s", channelID)
}

func (h *MessageHandler) GetThread(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("id")
	if threadID == "" {
		http.Error(w, "Thread ID required", http.StatusBadRequest)
		return
	}

	messages, err := h.store.GetThreadMessages(threadID)
	if err != nil {
		http.Error(w, "Failed to fetch thread", http.StatusInternalServerError)
		return
	}

	if messages == nil {
		messages = []models.MessageWithUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func (h *MessageHandler) Reply(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	threadID := r.PathValue("id")

	if threadID == "" {
		http.Error(w, "Thread ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	// Get parent message to find channel
	parent, err := h.store.GetMessage(threadID)
	if err != nil {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	msg, err := h.store.CreateMessage(parent.ChannelID, userID, req.Content, &threadID)
	if err != nil {
		http.Error(w, "Failed to send reply", http.StatusInternalServerError)
		return
	}

	user, _ := h.store.GetUserByID(userID)
	msgWithUser := models.MessageWithUser{
		Message: *msg,
		User:    user.ToResponse(),
	}

	// Broadcast to WebSocket clients
	if h.hub != nil {
		h.hub.BroadcastToChannel(parent.ChannelID, models.WSMessage{
			Type:    models.WSTypeNewMessage,
			Payload: msgWithUser,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(msgWithUser)
}

func (h *MessageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	messageID := r.PathValue("id")

	if messageID == "" {
		http.Error(w, "Message ID required", http.StatusBadRequest)
		return
	}

	// Get the message to check ownership and get channel ID
	msg, err := h.store.GetMessage(messageID)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Only the message author can delete it
	if msg.UserID != userID {
		http.Error(w, "You can only delete your own messages", http.StatusForbidden)
		return
	}

	// Delete the message
	if err := h.store.DeleteMessage(messageID); err != nil {
		log.Printf("Error deleting message %s: %v", messageID, err)
		http.Error(w, "Failed to delete message", http.StatusInternalServerError)
		return
	}

	// Broadcast deletion to WebSocket clients
	if h.hub != nil {
		h.hub.BroadcastToChannel(msg.ChannelID, models.WSMessage{
			Type: models.WSTypeMessageDeleted,
			Payload: map[string]string{
				"message_id": messageID,
				"channel_id": msg.ChannelID,
			},
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// TextToSpeech converts message text to audio
func (h *MessageHandler) TextToSpeech(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text  string `json:"text"`
		Voice string `json:"voice"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "Text is required", http.StatusBadRequest)
		return
	}

	// Limit text length to prevent abuse
	if len(req.Text) > 4096 {
		http.Error(w, "Text too long (max 4096 characters)", http.StatusBadRequest)
		return
	}

	// Get any available AI client for TTS
	var client *ai.OpenAIClient
	for _, c := range h.aiClients {
		if c.IsConfigured() {
			client = c
			break
		}
	}

	if client == nil {
		http.Error(w, "TTS not configured", http.StatusServiceUnavailable)
		return
	}

	audioData, err := client.TextToSpeech(req.Text, req.Voice)
	if err != nil {
		log.Printf("TTS error: %v", err)
		http.Error(w, "Failed to generate speech", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(audioData)))
	w.Write(audioData)
}
