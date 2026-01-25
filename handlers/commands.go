package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"smack-server/ai"
	"smack-server/commands"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"strings"
	"time"
)

type CommandHandler struct {
	store     *store.Store
	hub       *Hub
	aiClients map[string]*ai.OpenAIClient
}

func NewCommandHandler(s *store.Store, hub *Hub, aiClients map[string]*ai.OpenAIClient) *CommandHandler {
	return &CommandHandler{store: s, hub: hub, aiClients: aiClients}
}

// Create creates a new custom command
func (h *CommandHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.CreateCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, `{"error":"Name is required"}`, http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, `{"error":"URL is required"}`, http.StatusBadRequest)
		return
	}

	// Validate method
	method := strings.ToUpper(req.Method)
	if method != "GET" && method != "POST" {
		method = "GET"
	}

	// Validate response mode
	responseMode := req.ResponseMode
	if responseMode != "private" && responseMode != "channel" {
		responseMode = "private"
	}

	// Validate URL scheme
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		http.Error(w, `{"error":"URL must start with http:// or https://"}`, http.StatusBadRequest)
		return
	}

	cmd, err := h.store.CreateCommand(
		req.Name,
		req.Description,
		req.URL,
		method,
		req.Headers,
		req.BodyTemplate,
		responseMode,
		userID,
		req.IsGlobal,
	)
	if err != nil {
		http.Error(w, `{"error":"Failed to create command"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cmd)
}

// List returns all commands accessible to the user
func (h *CommandHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	cmds, err := h.store.GetCommandsForUser(userID)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch commands"}`, http.StatusInternalServerError)
		return
	}

	if cmds == nil {
		cmds = []models.CustomCommand{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cmds)
}

// Get returns a single command
func (h *CommandHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cmdID := r.PathValue("id")

	if cmdID == "" {
		http.Error(w, `{"error":"Command ID required"}`, http.StatusBadRequest)
		return
	}

	cmd, err := h.store.GetCommand(cmdID)
	if err != nil {
		http.Error(w, `{"error":"Command not found"}`, http.StatusNotFound)
		return
	}

	// Check access
	if !cmd.IsGlobal && cmd.CreatedBy != userID {
		http.Error(w, `{"error":"Not authorized"}`, http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cmd)
}

// Update updates a command
func (h *CommandHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cmdID := r.PathValue("id")

	if cmdID == "" {
		http.Error(w, `{"error":"Command ID required"}`, http.StatusBadRequest)
		return
	}

	// Check ownership
	cmd, err := h.store.GetCommand(cmdID)
	if err != nil {
		http.Error(w, `{"error":"Command not found"}`, http.StatusNotFound)
		return
	}

	if cmd.CreatedBy != userID {
		http.Error(w, `{"error":"Not authorized to update this command"}`, http.StatusForbidden)
		return
	}

	var req models.UpdateCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate method if provided
	if req.Method != nil {
		method := strings.ToUpper(*req.Method)
		if method != "GET" && method != "POST" {
			http.Error(w, `{"error":"Method must be GET or POST"}`, http.StatusBadRequest)
			return
		}
		req.Method = &method
	}

	// Validate URL if provided
	if req.URL != nil {
		if !strings.HasPrefix(*req.URL, "http://") && !strings.HasPrefix(*req.URL, "https://") {
			http.Error(w, `{"error":"URL must start with http:// or https://"}`, http.StatusBadRequest)
			return
		}
	}

	// Validate response mode if provided
	if req.ResponseMode != nil {
		if *req.ResponseMode != "private" && *req.ResponseMode != "channel" {
			http.Error(w, `{"error":"Response mode must be 'private' or 'channel'"}`, http.StatusBadRequest)
			return
		}
	}

	err = h.store.UpdateCommand(cmdID, &req)
	if err != nil {
		http.Error(w, `{"error":"Failed to update command"}`, http.StatusInternalServerError)
		return
	}

	// Return updated command
	updated, _ := h.store.GetCommand(cmdID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// Delete removes a command
func (h *CommandHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cmdID := r.PathValue("id")

	if cmdID == "" {
		http.Error(w, `{"error":"Command ID required"}`, http.StatusBadRequest)
		return
	}

	err := h.store.DeleteCommand(cmdID, userID)
	if err != nil {
		http.Error(w, `{"error":"Failed to delete command"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "command deleted"})
}

// Execute runs a custom command
func (h *CommandHandler) Execute(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.ExecuteCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.CommandID == "" {
		http.Error(w, `{"error":"Command ID is required"}`, http.StatusBadRequest)
		return
	}

	// Get command
	cmd, err := h.store.GetCommand(req.CommandID)
	if err != nil {
		http.Error(w, `{"error":"Command not found"}`, http.StatusNotFound)
		return
	}

	// Check access
	if !cmd.IsGlobal && cmd.CreatedBy != userID {
		http.Error(w, `{"error":"Not authorized"}`, http.StatusForbidden)
		return
	}

	if !cmd.Enabled {
		http.Error(w, `{"error":"Command is disabled"}`, http.StatusBadRequest)
		return
	}

	// Get user and channel info for interpolation
	user, _ := h.store.GetUserByID(userID)
	channel, _ := h.store.GetChannel(req.ChannelID)

	ctx := &commands.InterpolationContext{
		Input:   req.Input,
		UserID:  userID,
		ChannelID: req.ChannelID,
	}

	if user != nil {
		ctx.Username = user.Username
		ctx.DisplayName = user.DisplayName
	}

	if channel != nil {
		ctx.ChannelName = channel.Name
	}

	// Execute HTTP request
	result := h.executeHTTPRequest(cmd, ctx)

	// If response mode is channel, post result as a message
	if cmd.ResponseMode == "channel" && result.Success && req.ChannelID != "" {
		h.postResultToChannel(req.ChannelID, userID, cmd.Name, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *CommandHandler) executeHTTPRequest(cmd *models.CustomCommand, ctx *commands.InterpolationContext) *models.CommandExecutionResult {
	// Interpolate URL (with URL encoding for safe query parameters)
	url := commands.InterpolateURL(cmd.URL, ctx)

	// Prepare body if POST
	var body io.Reader
	if cmd.Method == "POST" && cmd.BodyTemplate != "" {
		interpolatedBody := commands.Interpolate(cmd.BodyTemplate, ctx)
		body = bytes.NewBufferString(interpolatedBody)
	}

	// Create request
	req, err := http.NewRequest(cmd.Method, url, body)
	if err != nil {
		return &models.CommandExecutionResult{
			Success: false,
			Error:   "Failed to create request: " + err.Error(),
		}
	}

	// Add headers
	if cmd.Headers != "" {
		var headers map[string]string
		if json.Unmarshal([]byte(cmd.Headers), &headers) == nil {
			for k, v := range headers {
				req.Header.Set(k, commands.Interpolate(v, ctx))
			}
		}
	}

	// Set default content-type for POST
	if cmd.Method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute with timeout
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &models.CommandExecutionResult{
			Success: false,
			Error:   "Request failed: " + err.Error(),
		}
	}
	defer resp.Body.Close()

	// Read response (limit to 1MB)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return &models.CommandExecutionResult{
			Success:    false,
			StatusCode: resp.StatusCode,
			Error:      "Failed to read response: " + err.Error(),
		}
	}

	return &models.CommandExecutionResult{
		Success:      resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode:   resp.StatusCode,
		ResponseBody: string(respBody),
	}
}

func (h *CommandHandler) postResultToChannel(channelID, userID, cmdName string, result *models.CommandExecutionResult) {
	// Format response for posting
	content := "**/" + cmdName + "** result:\n```\n"
	if len(result.ResponseBody) > 2000 {
		content += result.ResponseBody[:2000] + "\n...(truncated)"
	} else {
		content += result.ResponseBody
	}
	content += "\n```"

	// Create message
	msg, err := h.store.CreateMessage(channelID, userID, content, nil)
	if err != nil {
		return
	}

	// Get user for broadcast
	user, _ := h.store.GetUserByID(userID)
	var userResponse models.UserResponse
	if user != nil {
		userResponse = user.ToResponse()
	}

	msgWithUser := models.MessageWithUser{
		Message: *msg,
		User:    userResponse,
	}

	// Broadcast via WebSocket
	if h.hub != nil {
		h.hub.BroadcastToChannel(channelID, models.WSMessage{
			Type:    models.WSTypeNewMessage,
			Payload: msgWithUser,
		})
	}
}

// AIGenerate uses AI to generate a command configuration from natural language
func (h *CommandHandler) AIGenerate(w http.ResponseWriter, r *http.Request) {
	var req models.AIGenerateCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Description == "" {
		http.Error(w, `{"error":"Description is required"}`, http.StatusBadRequest)
		return
	}

	client, ok := h.aiClients["openai"]
	if !ok {
		http.Error(w, `{"error":"AI service not available"}`, http.StatusServiceUnavailable)
		return
	}

	systemPrompt := h.getCommandBuilderSystemPrompt()
	messages := []ai.InputMessage{
		ai.NewTextMessage("user", req.Description),
	}

	response, err := client.GetResponseWithContext(messages, systemPrompt)
	if err != nil {
		http.Error(w, `{"error":"AI generation failed"}`, http.StatusInternalServerError)
		return
	}

	// Try to parse AI response as command config
	var generatedCmd models.CustomCommand
	if err := json.Unmarshal([]byte(response), &generatedCmd); err != nil {
		// AI didn't return valid JSON, return the text as preview
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AIGenerateCommandResponse{
			Preview: response,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.AIGenerateCommandResponse{
		Command: &generatedCmd,
		Preview: "Generated command configuration ready for review.",
	})
}

func (h *CommandHandler) getCommandBuilderSystemPrompt() string {
	return `You are a command configuration generator for a chat application's custom slash commands.

When the user describes what they want a command to do, generate a JSON configuration object with these fields:
- name: Command name (lowercase, no spaces, e.g., "weather")
- description: Human-readable description
- url: Target URL (can include {{variables}})
- method: "GET" or "POST"
- headers: JSON string of headers, e.g. "{\"Authorization\": \"Bearer xxx\"}" (or empty string)
- body_template: JSON body for POST requests with {{variables}} (or empty string)
- response_mode: "private" (only sender sees) or "channel" (posts to channel)
- is_global: false (user should decide later)

AVAILABLE VARIABLES for interpolation:
- {{input}} - Full text after command
- {{input.0}}, {{input.1}}, etc. - Individual words by index
- {{input.rest}} - Everything except first word
- {{user.id}}, {{user.username}}, {{user.displayName}} - User info
- {{channel.id}}, {{channel.name}} - Channel info
- {{timestamp}}, {{date}}, {{datetime}} - Current time

EXAMPLE: User says "I want a weather command that looks up weather by city name"
Response:
{
  "name": "weather",
  "description": "Get weather for a city",
  "url": "https://api.weatherapi.com/v1/current.json?key=YOUR_API_KEY&q={{input}}",
  "method": "GET",
  "headers": "",
  "body_template": "",
  "response_mode": "private",
  "is_global": false
}

EXAMPLE: User says "Create a command to post a message to my Slack webhook"
Response:
{
  "name": "slack",
  "description": "Post a message to Slack",
  "url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
  "method": "POST",
  "headers": "{\"Content-Type\": \"application/json\"}",
  "body_template": "{\"text\": \"{{input}}\", \"username\": \"{{user.displayName}}\"}",
  "response_mode": "private",
  "is_global": false
}

Respond ONLY with the JSON object, no additional text or markdown formatting.`
}
