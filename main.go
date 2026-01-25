package main

import (
	"log"
	"net/http"
	"os"
	"smack-server/handlers"
	"smack-server/middleware"
	"smack-server/store"
	"strings"
)

func main() {
	// Initialize store
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./smack.db"
	}

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer s.Close()

	// Initialize WebSocket hub
	hub := handlers.NewHub(s)
	go hub.Run()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(s)
	channelHandler := handlers.NewChannelHandler(s)
	messageHandler := handlers.NewMessageHandler(s, hub)
	userHandler := handlers.NewUserHandler(s)
	reminderHandler := handlers.NewReminderHandler(s, hub)
	botHandler := handlers.NewBotHandler(s)
	reactionHandler := handlers.NewReactionHandler(s, hub)
	webhookHandler := handlers.NewWebhookHandler(s, hub)
	kanbanHandler := handlers.NewKanbanHandler(s)

	// Initialize OpenAI bot
	initOpenAIBot(s, messageHandler)

	// Initialize apps handler (needs AI clients from messageHandler)
	appsHandler := handlers.NewAppsHandler(s, hub, messageHandler.GetAIClients())

	// Initialize commands handler (needs AI clients from messageHandler)
	commandHandler := handlers.NewCommandHandler(s, hub, messageHandler.GetAIClients())

	// Initialize git handler for app repositories
	gitHandler := handlers.NewGitHandler(s, "./apps", hub)
	appsHandler.SetGitHandler(gitHandler)

	// File upload directory
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	fileHandler := handlers.NewFileHandler(uploadDir)

	// Start reminder checker
	reminderHandler.StartReminderChecker()

	// Create router
	mux := http.NewServeMux()

	// Public routes (no auth required)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/ws", hub.HandleWebSocket)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Protected routes (auth required)
	mux.HandleFunc("GET /api/auth/me", withAuth(authHandler.Me))

	// Channels
	mux.HandleFunc("GET /api/channels", withAuth(channelHandler.List))
	mux.HandleFunc("GET /api/channels/public", withAuth(channelHandler.ListPublic))
	mux.HandleFunc("POST /api/channels", withAuth(channelHandler.Create))
	mux.HandleFunc("GET /api/channels/{id}", withAuth(channelHandler.Get))
	mux.HandleFunc("PUT /api/channels/{id}", withAuth(channelHandler.Update))
	mux.HandleFunc("POST /api/channels/{id}/join", withAuth(channelHandler.Join))
	mux.HandleFunc("POST /api/channels/{id}/read", withAuth(channelHandler.MarkAsRead))
	mux.HandleFunc("POST /api/channels/{id}/mute", withAuth(channelHandler.Mute))
	mux.HandleFunc("POST /api/channels/{id}/unmute", withAuth(channelHandler.Unmute))
	mux.HandleFunc("POST /api/channels/{id}/leave", withAuth(channelHandler.Leave))
	mux.HandleFunc("POST /api/channels/{id}/clear", withAuth(channelHandler.Clear))
	mux.HandleFunc("GET /api/channels/{id}/members", withAuth(channelHandler.Members))
	mux.HandleFunc("GET /api/channels/{id}/messages", withAuth(messageHandler.GetChannelMessages))
	mux.HandleFunc("GET /api/channels/muted", withAuth(channelHandler.GetMuted))
	mux.HandleFunc("POST /api/dm", withAuth(channelHandler.CreateDM))

	// Messages
	mux.HandleFunc("POST /api/messages", withAuth(messageHandler.Send))
	mux.HandleFunc("DELETE /api/messages/{id}", withAuth(messageHandler.Delete))
	mux.HandleFunc("GET /api/messages/{id}/thread", withAuth(messageHandler.GetThread))
	mux.HandleFunc("POST /api/messages/{id}/reply", withAuth(messageHandler.Reply))

	// Text-to-Speech
	mux.HandleFunc("POST /api/tts", withAuth(messageHandler.TextToSpeech))

	// Users
	mux.HandleFunc("GET /api/users", withAuth(userHandler.List))
	mux.HandleFunc("PUT /api/users/me", withAuth(userHandler.UpdateProfile))
	mux.HandleFunc("PUT /api/users/me/status", withAuth(userHandler.UpdateStatus))
	mux.HandleFunc("GET /api/users/me", withAuth(userHandler.GetMe))
	mux.HandleFunc("GET /api/users/{id}", withAuth(userHandler.Get))

	// Reminders
	mux.HandleFunc("GET /api/reminders", withAuth(reminderHandler.List))
	mux.HandleFunc("POST /api/reminders", withAuth(reminderHandler.Create))
	mux.HandleFunc("DELETE /api/reminders/{id}", withAuth(reminderHandler.Delete))

	// Files
	mux.HandleFunc("POST /api/files/upload", withAuth(fileHandler.Upload))
	mux.HandleFunc("GET /api/files/{filename}", fileHandler.Serve)

	// Bots
	mux.HandleFunc("GET /api/bots", withAuth(botHandler.List))
	mux.HandleFunc("GET /api/bots/{id}", withAuth(botHandler.Get))
	mux.HandleFunc("POST /api/bots/dm", withAuth(botHandler.CreateBotDM))

	// Reactions
	mux.HandleFunc("POST /api/reactions", withAuth(reactionHandler.Add))
	mux.HandleFunc("DELETE /api/reactions", withAuth(reactionHandler.Remove))
	mux.HandleFunc("GET /api/messages/{id}/reactions", withAuth(reactionHandler.GetForMessage))

	// Webhooks (protected)
	mux.HandleFunc("POST /api/webhooks", withAuth(webhookHandler.Create))
	mux.HandleFunc("GET /api/webhooks", withAuth(webhookHandler.List))
	mux.HandleFunc("GET /api/webhooks/{id}", withAuth(webhookHandler.Get))
	mux.HandleFunc("DELETE /api/webhooks/{id}", withAuth(webhookHandler.Delete))

	// Webhooks (public - incoming)
	mux.HandleFunc("POST /api/webhooks/incoming/{id}/{token}", webhookHandler.Incoming)

	// Kanban Boards
	mux.HandleFunc("GET /api/boards", withAuth(kanbanHandler.ListBoards))
	mux.HandleFunc("POST /api/boards", withAuth(kanbanHandler.CreateBoard))
	mux.HandleFunc("GET /api/boards/{id}", withAuth(kanbanHandler.GetBoard))
	mux.HandleFunc("PUT /api/boards/{id}", withAuth(kanbanHandler.UpdateBoard))
	mux.HandleFunc("DELETE /api/boards/{id}", withAuth(kanbanHandler.DeleteBoard))

	// Board members
	mux.HandleFunc("GET /api/boards/{id}/members", withAuth(kanbanHandler.GetBoardMembers))
	mux.HandleFunc("POST /api/boards/{id}/members", withAuth(kanbanHandler.AddBoardMember))
	mux.HandleFunc("DELETE /api/boards/{id}/members/{userId}", withAuth(kanbanHandler.RemoveBoardMember))

	// Kanban Columns
	mux.HandleFunc("POST /api/boards/{id}/columns", withAuth(kanbanHandler.CreateColumn))
	mux.HandleFunc("PUT /api/columns/{id}", withAuth(kanbanHandler.UpdateColumn))
	mux.HandleFunc("DELETE /api/columns/{id}", withAuth(kanbanHandler.DeleteColumn))
	mux.HandleFunc("POST /api/boards/{id}/columns/reorder", withAuth(kanbanHandler.ReorderColumns))

	// Kanban Cards
	mux.HandleFunc("POST /api/boards/{id}/cards", withAuth(kanbanHandler.CreateCard))
	mux.HandleFunc("GET /api/cards/{id}", withAuth(kanbanHandler.GetCard))
	mux.HandleFunc("PUT /api/cards/{id}", withAuth(kanbanHandler.UpdateCard))
	mux.HandleFunc("DELETE /api/cards/{id}", withAuth(kanbanHandler.DeleteCard))
	mux.HandleFunc("POST /api/cards/{id}/move", withAuth(kanbanHandler.MoveCard))

	// Kanban Labels
	mux.HandleFunc("GET /api/boards/{id}/labels", withAuth(kanbanHandler.GetLabels))
	mux.HandleFunc("POST /api/boards/{id}/labels", withAuth(kanbanHandler.CreateLabel))
	mux.HandleFunc("PUT /api/labels/{id}", withAuth(kanbanHandler.UpdateLabel))
	mux.HandleFunc("DELETE /api/labels/{id}", withAuth(kanbanHandler.DeleteLabel))

	// Kanban Comments
	mux.HandleFunc("GET /api/cards/{id}/comments", withAuth(kanbanHandler.GetComments))
	mux.HandleFunc("POST /api/cards/{id}/comments", withAuth(kanbanHandler.CreateComment))
	mux.HandleFunc("DELETE /api/comments/{id}", withAuth(kanbanHandler.DeleteComment))

	// Apps
	mux.HandleFunc("GET /api/apps", withAuth(appsHandler.ListApps))
	mux.HandleFunc("POST /api/apps", withAuth(appsHandler.CreateApp))
	mux.HandleFunc("GET /api/apps/{id}", withAuth(appsHandler.GetApp))
	mux.HandleFunc("PUT /api/apps/{id}", withAuth(appsHandler.UpdateApp))
	mux.HandleFunc("DELETE /api/apps/{id}", withAuth(appsHandler.DeleteApp))

	// App Code
	mux.HandleFunc("GET /api/apps/{id}/code", withAuth(appsHandler.GetAppCode))
	mux.HandleFunc("PUT /api/apps/{id}/code", withAuth(appsHandler.UpdateAppCode))
	mux.HandleFunc("GET /api/apps/{id}/serve", withAuth(appsHandler.ServeApp))

	// App Members
	mux.HandleFunc("GET /api/apps/{id}/members", withAuth(appsHandler.GetAppMembers))
	mux.HandleFunc("POST /api/apps/{id}/members", withAuth(appsHandler.AddAppMember))
	mux.HandleFunc("DELETE /api/apps/{id}/members/{userId}", withAuth(appsHandler.RemoveAppMember))

	// App Query (Database)
	mux.HandleFunc("POST /api/apps/{id}/query", withAuth(appsHandler.Query))

	// App Chat (AI)
	mux.HandleFunc("POST /api/apps/{id}/chat", withAuth(appsHandler.Chat))
	mux.HandleFunc("GET /api/apps/{id}/chat/history", withAuth(appsHandler.GetChatHistory))

	// Custom Commands
	mux.HandleFunc("GET /api/commands", withAuth(commandHandler.List))
	mux.HandleFunc("POST /api/commands", withAuth(commandHandler.Create))
	mux.HandleFunc("GET /api/commands/{id}", withAuth(commandHandler.Get))
	mux.HandleFunc("PUT /api/commands/{id}", withAuth(commandHandler.Update))
	mux.HandleFunc("DELETE /api/commands/{id}", withAuth(commandHandler.Delete))
	mux.HandleFunc("POST /api/commands/execute", withAuth(commandHandler.Execute))
	mux.HandleFunc("POST /api/commands/ai-generate", withAuth(commandHandler.AIGenerate))

	// Git HTTP (for cloning/pushing app repos - uses HTTP Basic Auth with JWT token)
	mux.HandleFunc("GET /git/{appID}/info/refs", gitHandler.InfoRefs)
	mux.HandleFunc("POST /git/{appID}/git-upload-pack", gitHandler.UploadPack)
	mux.HandleFunc("POST /git/{appID}/git-receive-pack", gitHandler.ReceivePack)

	// CORS wrapper
	handler := corsMiddleware(mux)

	// Get port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Smack server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

// withAuth wraps a handler with authentication
func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		claims, err := middleware.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = middleware.SetUserID(ctx, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func initOpenAIBot(s *store.Store, messageHandler *handlers.MessageHandler) {
	const (
		botID       = "openai-gpt"
		botName     = "openai"
		displayName = "ChatGPT"
		description = "OpenAI's GPT-5.2 language model"
		provider    = "openai"
		model       = "gpt-5.2"
	)

	// Ensure bot user exists in users table (for message foreign keys)
	s.EnsureBotUser(botID, "bot-"+botName, displayName, "")

	// Create the bot if it doesn't exist
	_, err := s.CreateBot(botID, botName, displayName, description, provider, model, "")
	if err != nil {
		log.Printf("Note: OpenAI bot may already exist: %v", err)
	}

	// Register the AI client
	messageHandler.RegisterAIClient(provider, model)

	// Check if OpenAI key is configured
	if os.Getenv("OPENAI_KEY") == "" {
		log.Println("Warning: OPENAI_KEY not set. OpenAI bot will not work.")
	} else {
		log.Println("OpenAI bot initialized with model:", model)
	}
}
