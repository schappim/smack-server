package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"smack-server/ai"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type AppsHandler struct {
	store     *store.Store
	hub       *Hub
	aiClients map[string]*ai.OpenAIClient

	// Per-app database connections cache
	appDBs   map[string]*sql.DB
	appDBsMu sync.RWMutex

	// Git handler for syncing app repos
	gitHandler *GitHandler
}

func NewAppsHandler(s *store.Store, hub *Hub, aiClients map[string]*ai.OpenAIClient) *AppsHandler {
	return &AppsHandler{
		store:     s,
		hub:       hub,
		aiClients: aiClients,
		appDBs:    make(map[string]*sql.DB),
	}
}

// SetGitHandler sets the git handler for syncing repos when code is updated
func (h *AppsHandler) SetGitHandler(gh *GitHandler) {
	h.gitHandler = gh
}

// ==================== App CRUD ====================

func (h *AppsHandler) ListApps(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	apps, err := h.store.GetAppsForUser(userID)
	if err != nil {
		http.Error(w, "Failed to fetch apps", http.StatusInternalServerError)
		return
	}

	if apps == nil {
		apps = []models.AppWithDetails{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

func (h *AppsHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.CreateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	app, err := h.store.CreateApp(req.Name, req.Description, req.Icon, userID)
	if err != nil {
		http.Error(w, "Failed to create app", http.StatusInternalServerError)
		return
	}

	// Create the app's data directory
	if err := h.ensureAppDataDir(app.ID); err != nil {
		log.Printf("Warning: failed to create app data directory: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(app)
}

func (h *AppsHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	// Check membership
	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	app, err := h.store.GetApp(appID)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

func (h *AppsHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	// Check membership and role
	role, err := h.store.GetAppMemberRole(appID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Not authorized to update this app", http.StatusForbidden)
		return
	}

	var req models.UpdateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	app, err := h.store.GetApp(appID)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	name := app.Name
	description := app.Description
	if req.Name != "" {
		name = req.Name
	}
	if req.Description != "" {
		description = req.Description
	}

	err = h.store.UpdateApp(appID, name, description, req.Icon)
	if err != nil {
		http.Error(w, "Failed to update app", http.StatusInternalServerError)
		return
	}

	app, _ = h.store.GetApp(appID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

func (h *AppsHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	// Check ownership
	role, err := h.store.GetAppMemberRole(appID, userID)
	if err != nil || role != "owner" {
		http.Error(w, "Only the owner can delete an app", http.StatusForbidden)
		return
	}

	// Close and remove app database connection
	h.closeAppDB(appID)

	// Delete app data directory
	appDataDir := filepath.Join(".", "apps", appID)
	os.RemoveAll(appDataDir)

	err = h.store.DeleteApp(appID)
	if err != nil {
		http.Error(w, "Failed to delete app", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== App Code ====================

func (h *AppsHandler) GetAppCode(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	app, err := h.store.GetApp(appID)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	response := struct {
		HTMLContent string `json:"html_content"`
		CSSContent  string `json:"css_content"`
		JSContent   string `json:"js_content"`
	}{
		HTMLContent: app.HTMLContent,
		CSSContent:  app.CSSContent,
		JSContent:   app.JSContent,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AppsHandler) UpdateAppCode(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	var req models.UpdateAppCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	app, err := h.store.GetApp(appID)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	html := app.HTMLContent
	css := app.CSSContent
	js := app.JSContent

	if req.HTMLContent != "" {
		html = req.HTMLContent
	}
	if req.CSSContent != "" {
		css = req.CSSContent
	}
	if req.JSContent != "" {
		js = req.JSContent
	}

	err = h.store.UpdateAppCode(appID, html, css, js)
	if err != nil {
		http.Error(w, "Failed to update app code", http.StatusInternalServerError)
		return
	}

	// Sync to git repository (async to not block response)
	if h.gitHandler != nil {
		go func() {
			if err := h.gitHandler.SyncDatabaseToGit(appID); err != nil {
				log.Printf("[Apps] Failed to sync app %s to git: %v", appID, err)
			}
		}()
	}

	// Broadcast code update to all viewers
	h.broadcastCodeUpdate(appID)

	app, _ = h.store.GetApp(appID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

func (h *AppsHandler) ServeApp(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	app, err := h.store.GetApp(appID)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	// Generate complete HTML document
	html := h.generateFullHTML(app)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// ==================== App Members ====================

func (h *AppsHandler) GetAppMembers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	members, err := h.store.GetAppMembers(appID)
	if err != nil {
		http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

func (h *AppsHandler) AddAppMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	role, err := h.store.GetAppMemberRole(appID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Not authorized to add members", http.StatusForbidden)
		return
	}

	var req models.AddAppMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.store.AddAppMember(appID, req.UserID, req.Role)
	if err != nil {
		http.Error(w, "Failed to add member", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *AppsHandler) RemoveAppMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")
	targetUserID := r.PathValue("userId")

	if appID == "" || targetUserID == "" {
		http.Error(w, "App ID and User ID required", http.StatusBadRequest)
		return
	}

	role, err := h.store.GetAppMemberRole(appID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Not authorized to remove members", http.StatusForbidden)
		return
	}

	// Prevent removing the owner
	targetRole, _ := h.store.GetAppMemberRole(appID, targetUserID)
	if targetRole == "owner" {
		http.Error(w, "Cannot remove the owner", http.StatusBadRequest)
		return
	}

	err = h.store.RemoveAppMember(appID, targetUserID)
	if err != nil {
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==================== App Query (Database) ====================

func (h *AppsHandler) Query(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	var req models.AppQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate query
	if !h.isAllowedQuery(req.Query) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AppQueryResponse{
			Error: "Query type not allowed. Only SELECT, INSERT, UPDATE, DELETE, CREATE TABLE, CREATE INDEX are permitted.",
		})
		return
	}

	// Get or create app database
	db, err := h.getAppDB(appID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AppQueryResponse{
			Error: "Failed to connect to app database: " + err.Error(),
		})
		return
	}

	// Execute query
	result, err := h.executeQuery(db, req.Query, req.Params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.AppQueryResponse{
			Error: err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ==================== App Chat (AI) ====================

func (h *AppsHandler) Chat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	var req models.AppChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	app, err := h.store.GetApp(appID)
	if err != nil {
		http.Error(w, "App not found", http.StatusNotFound)
		return
	}

	// Store user message
	h.store.CreateAppMessage(appID, userID, "user", req.Message)

	// Get AI client
	client, ok := h.aiClients["openai"]
	if !ok {
		http.Error(w, "AI service not available", http.StatusServiceUnavailable)
		return
	}

	// Get conversation history
	history, _ := h.store.GetAppMessages(appID, 20)

	// Build context messages
	var contextMessages []ai.InputMessage
	for _, msg := range history {
		contextMessages = append(contextMessages, ai.InputMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add current message
	contextMessages = append(contextMessages, ai.InputMessage{
		Role:    "user",
		Content: req.Message,
	})

	// Create message ID for streaming
	messageID := uuid.New().String()

	// Broadcast stream start
	if h.hub != nil {
		h.hub.BroadcastToApp(appID, models.WSMessage{
			Type: models.WSTypeAppStreamStart,
			Payload: models.AppStreamStartPayload{
				MessageID: messageID,
				AppID:     appID,
			},
		})
	}

	// Get system prompt
	systemPrompt := h.getAppBuilderSystemPrompt(app)

	// Define the update_code tool for the Responses API
	tools := []ai.Tool{
		{
			Type:        "function",
			Name:        "update_code",
			Description: "Update the app's HTML, CSS, and/or JavaScript code. Call this function when you need to create or modify the app's code.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"html": map[string]interface{}{
						"type":        "string",
						"description": "The complete HTML content for the body of the app (no html/head/body tags, just the inner content)",
					},
					"css": map[string]interface{}{
						"type":        "string",
						"description": "The complete CSS styles for the app",
					},
					"js": map[string]interface{}{
						"type":        "string",
						"description": "The complete JavaScript code for the app",
					},
				},
				"required": []string{},
			},
		},
	}

	// Stream AI response with tool support
	var finalContent string
	var codeWasUpdated bool

	result, err := client.StreamResponseWithTools(contextMessages, systemPrompt, tools, func(delta, fullText string, toolCall *ai.ToolCall) {
		if toolCall != nil {
			// Handle tool call
			log.Printf("[Apps] Tool call received: %s", toolCall.Name)
			if toolCall.Name == "update_code" {
				var codeArgs struct {
					HTML string `json:"html"`
					CSS  string `json:"css"`
					JS   string `json:"js"`
				}
				if err := json.Unmarshal([]byte(toolCall.Arguments), &codeArgs); err != nil {
					log.Printf("[Apps] Failed to parse update_code args: %v", err)
					return
				}
				log.Printf("[Apps] Updating code - HTML: %d bytes, CSS: %d bytes, JS: %d bytes",
					len(codeArgs.HTML), len(codeArgs.CSS), len(codeArgs.JS))

				// Update the code immediately
				h.store.UpdateAppCode(appID, codeArgs.HTML, codeArgs.CSS, codeArgs.JS)

				// Sync to git repository (async)
				if h.gitHandler != nil {
					go h.gitHandler.SyncDatabaseToGit(appID)
				}

				h.broadcastCodeUpdate(appID)
				codeWasUpdated = true
			}
		} else if delta != "" {
			// Regular text streaming
			if h.hub != nil {
				h.hub.BroadcastToApp(appID, models.WSMessage{
					Type: models.WSTypeAppStreamDelta,
					Payload: models.AppStreamDeltaPayload{
						MessageID: messageID,
						AppID:     appID,
						Delta:     delta,
						FullText:  fullText,
					},
				})
			}
		}
	})

	if err != nil {
		log.Printf("Failed to get AI response: %v", err)
		finalContent = "Sorry, I'm having trouble connecting right now. Please try again later."
	} else {
		finalContent = result.Text
		// If tool was called but there's no text response, add a confirmation message
		if codeWasUpdated && finalContent == "" {
			finalContent = "I've updated the app code. Check the preview on the left!"
		}
	}

	// Store assistant response
	h.store.CreateAppMessage(appID, "assistant", "assistant", finalContent)

	// Broadcast stream end
	if h.hub != nil {
		h.hub.BroadcastToApp(appID, models.WSMessage{
			Type: models.WSTypeAppStreamEnd,
			Payload: models.AppStreamEndPayload{
				MessageID: messageID,
				AppID:     appID,
				Content:   finalContent,
			},
		})
	}

	// Return the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message_id": messageID,
		"content":    finalContent,
	})
}

func (h *AppsHandler) GetChatHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	appID := r.PathValue("id")

	if appID == "" {
		http.Error(w, "App ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsAppMember(appID, userID)
	if !isMember {
		http.Error(w, "Not a member of this app", http.StatusForbidden)
		return
	}

	messages, err := h.store.GetAppMessages(appID, 50)
	if err != nil {
		http.Error(w, "Failed to fetch chat history", http.StatusInternalServerError)
		return
	}

	if messages == nil {
		messages = []models.AppMessage{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// ==================== Helper Functions ====================

func (h *AppsHandler) ensureAppDataDir(appID string) error {
	appDataDir := filepath.Join(".", "apps", appID)
	return os.MkdirAll(appDataDir, 0755)
}

func (h *AppsHandler) getAppDB(appID string) (*sql.DB, error) {
	h.appDBsMu.RLock()
	db, exists := h.appDBs[appID]
	h.appDBsMu.RUnlock()

	if exists {
		return db, nil
	}

	// Create new connection
	h.appDBsMu.Lock()
	defer h.appDBsMu.Unlock()

	// Double-check after acquiring write lock
	if db, exists := h.appDBs[appID]; exists {
		return db, nil
	}

	// Ensure directory exists
	if err := h.ensureAppDataDir(appID); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(".", "apps", appID, "data.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	h.appDBs[appID] = db
	return db, nil
}

func (h *AppsHandler) closeAppDB(appID string) {
	h.appDBsMu.Lock()
	defer h.appDBsMu.Unlock()

	if db, exists := h.appDBs[appID]; exists {
		db.Close()
		delete(h.appDBs, appID)
	}
}

func (h *AppsHandler) isAllowedQuery(query string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(query))
	allowed := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE TABLE", "CREATE INDEX"}
	for _, prefix := range allowed {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func (h *AppsHandler) executeQuery(db *sql.DB, query string, params []interface{}) (*models.AppQueryResponse, error) {
	normalized := strings.ToUpper(strings.TrimSpace(query))

	// For SELECT queries, return rows
	if strings.HasPrefix(normalized, "SELECT") {
		rows, err := db.Query(query, params...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		var results []map[string]interface{}
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				return nil, err
			}

			row := make(map[string]interface{})
			for i, col := range columns {
				val := values[i]
				// Convert []byte to string
				if b, ok := val.([]byte); ok {
					row[col] = string(b)
				} else {
					row[col] = val
				}
			}
			results = append(results, row)
		}

		return &models.AppQueryResponse{
			Columns: columns,
			Rows:    results,
		}, nil
	}

	// For other queries, execute and return affected rows info
	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()

	return &models.AppQueryResponse{
		Columns: []string{"rows_affected", "last_insert_id"},
		Rows: []map[string]interface{}{
			{
				"rows_affected":  rowsAffected,
				"last_insert_id": lastInsertID,
			},
		},
	}, nil
}

func (h *AppsHandler) broadcastCodeUpdate(appID string) {
	app, err := h.store.GetApp(appID)
	if err != nil {
		return
	}

	if h.hub != nil {
		h.hub.BroadcastToApp(appID, models.WSMessage{
			Type: models.WSTypeAppCodeUpdated,
			Payload: models.AppCodeUpdatedPayload{
				AppID:       appID,
				HTMLContent: app.HTMLContent,
				CSSContent:  app.CSSContent,
				JSContent:   app.JSContent,
				UpdatedAt:   app.UpdatedAt.Format(time.RFC3339),
			},
		})
	}
}

func (h *AppsHandler) generateFullHTML(app *models.App) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
%s
    </style>
</head>
<body>
%s
    <script>
// SmackDB API helper
const SmackDB = {
    appId: '%s',
    async query(sql, params = []) {
        const response = await fetch('/api/apps/' + this.appId + '/query', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': 'Bearer ' + window.smackToken
            },
            body: JSON.stringify({ query: sql, params: params })
        });
        return response.json();
    }
};

%s
    </script>
</body>
</html>`, app.Name, app.CSSContent, app.HTMLContent, app.ID, app.JSContent)
}

func (h *AppsHandler) getAppBuilderSystemPrompt(app *models.App) string {
	return fmt.Sprintf(`You are an expert web application builder assistant. You help users create and modify web-based applications using HTML, CSS, and JavaScript.

CURRENT APP STATE:
- Name: %s
- Description: %s
- Current HTML:
`+"```html\n%s\n```"+`
- Current CSS:
`+"```css\n%s\n```"+`
- Current JS:
`+"```javascript\n%s\n```"+`

CAPABILITIES:
1. You can create/modify HTML, CSS, and JavaScript code using the update_code function
2. The app has access to a SQLite database via SmackDB.query(sql, params)
3. You can create database tables and perform CRUD operations

HOW TO UPDATE CODE:
Use the update_code function tool to update the app's code. Always provide COMPLETE code for each field (html, css, js) - not partial updates. The function will replace the existing code with what you provide.

When answering questions or explaining something, just respond with text - don't call update_code unless actually modifying the app.

DATABASE API - PERSISTENT SERVER STORAGE:
Each app has its own private SQLite database stored permanently on the server. Data persists across sessions and page reloads.

SmackDB.query(sql, params) - Returns a Promise with { columns, rows, error }

CREATING TABLES:
await SmackDB.query("CREATE TABLE IF NOT EXISTS todos (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, done INTEGER DEFAULT 0, created_at TEXT DEFAULT CURRENT_TIMESTAMP)")

INSERTING DATA:
const result = await SmackDB.query("INSERT INTO todos (title, done) VALUES (?, ?)", ["Buy milk", 0])
// result.rows will be empty for INSERT, but data is persisted on server

QUERYING DATA:
const result = await SmackDB.query("SELECT * FROM todos WHERE done = ? ORDER BY created_at DESC", [0])
// result.columns = ["id", "title", "done", "created_at"]
// result.rows = [{id: 1, title: "Buy milk", done: 0, created_at: "2024-01-15 10:30:00"}, ...]

UPDATING DATA:
await SmackDB.query("UPDATE todos SET done = ? WHERE id = ?", [1, 5])

DELETING DATA:
await SmackDB.query("DELETE FROM todos WHERE id = ?", [5])

ERROR HANDLING:
const result = await SmackDB.query("SELECT * FROM todos")
if (result.error) {
    console.error("Database error:", result.error)
    return
}
// Use result.rows safely

IMPORTANT NOTES:
1. Always use CREATE TABLE IF NOT EXISTS to avoid errors on page reload
2. Use parameterized queries (?, ?) to prevent SQL injection - never concatenate user input
3. SmackDB.query() is async - always use await or .then()
4. Initialize your database schema when the app loads (e.g., in a DOMContentLoaded handler)
5. Data types: TEXT, INTEGER, REAL, BLOB, NULL (SQLite is flexible with types)
6. The database is private to this app - other apps cannot access it

GUIDELINES:
1. Keep code clean and well-organized
2. Use modern JavaScript (ES6+)
3. Make the UI responsive and user-friendly
4. Handle errors gracefully
5. Provide clear feedback to users
6. When creating database schemas, use appropriate data types
7. Always include complete code in your response, not just partial updates
8. The HTML should only include body content (no html, head, or body tags)

Start by understanding what the user wants to build, then incrementally improve the app based on their feedback.`, app.Name, app.Description, app.HTMLContent, app.CSSContent, app.JSContent)
}

type codeUpdate struct {
	HTML string
	CSS  string
	JS   string
}

func (h *AppsHandler) parseCodeFromResponse(response string) *codeUpdate {
	// Look for JSON code block with action: "update_code"
	// Use a more flexible pattern that handles various whitespace
	jsonPattern := regexp.MustCompile("(?s)```json\\s*\n(.+?)\n\\s*```")
	matches := jsonPattern.FindStringSubmatch(response)
	if len(matches) < 2 {
		// Try alternate pattern without requiring newlines
		jsonPattern2 := regexp.MustCompile("(?s)```json\\s*(.+?)```")
		matches = jsonPattern2.FindStringSubmatch(response)
		if len(matches) < 2 {
			log.Printf("[Apps] No JSON code block found in response (length: %d)", len(response))
			return nil
		}
	}

	jsonStr := strings.TrimSpace(matches[1])
	log.Printf("[Apps] Found JSON block (length: %d)", len(jsonStr))

	var data struct {
		Action string `json:"action"`
		HTML   string `json:"html"`
		CSS    string `json:"css"`
		JS     string `json:"js"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		log.Printf("[Apps] Failed to parse code update JSON: %v", err)
		// Log first 500 chars of the JSON for debugging
		preview := jsonStr
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		log.Printf("[Apps] JSON preview: %s", preview)
		return nil
	}

	if data.Action != "update_code" {
		log.Printf("[Apps] JSON action is '%s', not 'update_code'", data.Action)
		return nil
	}

	log.Printf("[Apps] Successfully parsed code update - HTML: %d bytes, CSS: %d bytes, JS: %d bytes",
		len(data.HTML), len(data.CSS), len(data.JS))

	return &codeUpdate{
		HTML: data.HTML,
		CSS:  data.CSS,
		JS:   data.JS,
	}
}
