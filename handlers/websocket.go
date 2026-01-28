package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8 * 1024
)

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	userID     string
	channels   map[string]bool
	channelsMu sync.RWMutex
	apps       map[string]bool
	appsMu     sync.RWMutex
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	store      *store.Store
	mu         sync.RWMutex
}

func NewHub(s *store.Store) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		store:      s,
	}
}

func (h *Hub) Run() {
	log.Printf("[WS HUB] Hub started and running")
	for {
		select {
		case client := <-h.register:
			log.Printf("[WS HUB] >>> Received register request for client %s", client.userID)
			h.mu.Lock()
			// Check if this user already has a connection (for online status broadcast)
			isFirstConnection := true
			for c := range h.clients {
				if c.userID == client.userID {
					isFirstConnection = false
					break
				}
			}
			h.clients[client] = true
			clientCount := len(h.clients)
			h.mu.Unlock()

			log.Printf("[WS HUB] ✅ Client registered: %s (total clients: %d, first=%v)", client.userID, clientCount, isFirstConnection)

			// Update user status to online
			h.store.UpdateUserStatus(client.userID, "online")

			// Only broadcast user_online on first connection to avoid duplicate notifications
			if isFirstConnection {
				go h.BroadcastAll(models.WSMessage{
					Type: models.WSTypeUserOnline,
					Payload: map[string]string{
						"user_id": client.userID,
					},
				})
			}

		case client := <-h.unregister:
			h.mu.Lock()
			wasPresent := false
			if _, ok := h.clients[client]; ok {
				wasPresent = true
				delete(h.clients, client)
				close(client.send)
			}
			// Check if user has any remaining connections
			hasOtherConnections := false
			if wasPresent {
				for c := range h.clients {
					if c.userID == client.userID {
						hasOtherConnections = true
						break
					}
				}
			}
			clientCount := len(h.clients)
			h.mu.Unlock()

			if wasPresent {
				log.Printf("[WS HUB] ❌ Client unregistered: %s (total clients: %d, other connections: %v)", client.userID, clientCount, hasOtherConnections)

				// Only mark offline and broadcast if this was the user's last connection
				if !hasOtherConnections {
					h.store.UpdateUserStatus(client.userID, "offline")

					go h.BroadcastAll(models.WSMessage{
						Type: models.WSTypeUserOffline,
						Payload: map[string]string{
							"user_id": client.userID,
						},
					})
				}
			} else {
				log.Printf("[WS HUB] Client unregister requested but not found: %s", client.userID)
			}

		case message := <-h.broadcast:
			var staleClients []*Client
			h.mu.RLock()
			totalClients := len(h.clients)
			sentCount := 0
			for client := range h.clients {
				select {
				case client.send <- message:
					sentCount++
				default:
					log.Printf("[WS HUB] ⚠️ Client %s buffer full - marking as stale", client.userID)
					staleClients = append(staleClients, client)
				}
			}
			h.mu.RUnlock()

			// Log broadcast message type
			var wsMsg models.WSMessage
			if json.Unmarshal(message, &wsMsg) == nil {
				log.Printf("[WS HUB] Broadcast type '%s' sent to %d/%d clients", wsMsg.Type, sentCount, totalClients)
			}

			if len(staleClients) > 0 {
				log.Printf("[WS HUB] Removing %d stale clients", len(staleClients))
				h.mu.Lock()
				for _, client := range staleClients {
					if _, ok := h.clients[client]; ok {
						close(client.send)
						delete(h.clients, client)
						log.Printf("[WS HUB] Removed stale client: %s", client.userID)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// disconnectUser closes all existing connections for a user
func (h *Hub) disconnectUser(userID string) {
	h.mu.Lock()
	var toRemove []*Client
	for client := range h.clients {
		if client.userID == userID {
			toRemove = append(toRemove, client)
		}
	}
	for _, client := range toRemove {
		log.Printf("[WS] Disconnecting existing connection for user %s (new connection incoming)", userID)
		delete(h.clients, client)
		close(client.send)
		client.conn.Close()
	}
	h.mu.Unlock()
	if len(toRemove) > 0 {
		log.Printf("[WS] Removed %d stale connection(s) for user %s", len(toRemove), userID)
	}
}

func (h *Hub) BroadcastToChannel(channelID string, msg models.WSMessage) {
	// Simplified: broadcast to ALL clients, let client filter by channel
	// This removes subscription complexity that was causing delivery issues
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	isTyping := msg.Type == models.WSTypeTyping

	sentCount := 0
	var staleClients []*Client
	h.mu.RLock()
	totalClients := len(h.clients)
	for client := range h.clients {
		select {
		case client.send <- data:
			sentCount++
		default:
			log.Printf("[WS] Client %s buffer full, closing", client.userID)
			staleClients = append(staleClients, client)
		}
	}
	h.mu.RUnlock()

	if len(staleClients) > 0 {
		h.mu.Lock()
		for _, client := range staleClients {
			if _, ok := h.clients[client]; ok {
				close(client.send)
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}

	if !isTyping {
		log.Printf("[WS] BroadcastToChannel %s type=%s sent to %d/%d clients", channelID, msg.Type, sentCount, totalClients)
	}
}

// BroadcastToChannelExcept sends a message to all clients in a channel except the specified one
func (h *Hub) BroadcastToChannelExcept(channelID string, except *Client, msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	for client := range h.clients {
		if client != except {
			select {
			case client.send <- data:
			default:
				// Buffer full, skip
			}
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) BroadcastAll(msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS] BroadcastAll marshal error for type '%s': %v", msg.Type, err)
		return
	}

	log.Printf("[WS] BroadcastAll queuing message type '%s'", msg.Type)
	h.broadcast <- data
}

func (h *Hub) SendToUser(userID string, msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS] SendToUser marshal error for type '%s': %v", msg.Type, err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	sentCount := 0
	for client := range h.clients {
		if client.userID == userID {
			select {
			case client.send <- data:
				sentCount++
			default:
				log.Printf("[WS] SendToUser: client %s buffer full, closing", client.userID)
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
	log.Printf("[WS] SendToUser type '%s' to user %s: sent to %d connections", msg.Type, userID, sentCount)
}

func (h *Hub) BroadcastToApp(appID string, msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	sentCount := 0
	var staleClients []*Client
	h.mu.RLock()
	for client := range h.clients {
		if client.isSubscribedToApp(appID) {
			select {
			case client.send <- data:
				sentCount++
			default:
				staleClients = append(staleClients, client)
			}
		}
	}
	h.mu.RUnlock()
	if len(staleClients) > 0 {
		h.mu.Lock()
		for _, client := range staleClients {
			if _, ok := h.clients[client]; ok {
				close(client.send)
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
	log.Printf("[WS] BroadcastToApp %s type=%s sent to %d clients", appID, msg.Type, sentCount)
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("[WS] HandleWebSocket called from %s", r.RemoteAddr)

	// Get token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		log.Printf("[WS] Connection rejected - no token provided from %s", r.RemoteAddr)
		http.Error(w, "Token required", http.StatusUnauthorized)
		return
	}

	claims, err := middleware.ValidateToken(token)
	if err != nil {
		log.Printf("[WS] Connection rejected - invalid token from %s: %v", r.RemoteAddr, err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	log.Printf("[WS] Token validated for user %s from %s", claims.UserID, r.RemoteAddr)

	// Allow multiple connections per user (e.g. desktop + laptop)
	// Do NOT disconnect existing connections

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error for user %s: %v", claims.UserID, err)
		return
	}

	log.Printf("[WS] Connection upgraded to WebSocket for user %s", claims.UserID)

	// Get user's channels
	channels, err := h.store.GetChannelsForUser(claims.UserID)
	if err != nil {
		log.Printf("[WS] ⚠️ Failed to get channels for user %s: %v", claims.UserID, err)
	}
	channelMap := make(map[string]bool)
	for _, ch := range channels {
		channelMap[ch.ID] = true
	}
	log.Printf("[WS] User %s auto-subscribed to %d channels", claims.UserID, len(channelMap))

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		userID:   claims.UserID,
		channels: channelMap,
		apps:     make(map[string]bool),
	}

	// Send a welcome message immediately BEFORE registering
	// This ensures the message is sent before any other activity
	welcomeMsg := []byte(`{"type":"welcome","payload":{"message":"connected"}}`)
	log.Printf("[WS] Sending welcome message to %s...", claims.UserID)
	if err := conn.WriteMessage(websocket.TextMessage, welcomeMsg); err != nil {
		log.Printf("[WS] ❌ Failed to send welcome message to %s: %v", claims.UserID, err)
		conn.Close()
		return
	}
	log.Printf("[WS] ✅ Welcome message sent successfully to %s", claims.UserID)

	// Start pumps BEFORE registering with hub to ensure we can receive messages
	go client.writePump()
	go client.readPump()

	// Small delay to ensure goroutines are running
	time.Sleep(10 * time.Millisecond)

	log.Printf("[WS] >>> Sending client %s to register channel...", claims.UserID)
	h.register <- client
	log.Printf("[WS] <<< Client %s sent to register channel (hub should process)", claims.UserID)
}

func (c *Client) readPump() {
	log.Printf("[WS] readPump started for client %s", c.userID)
	defer func() {
		log.Printf("[WS] readPump ending for client %s - sending unregister", c.userID)
		c.hub.unregister <- c
		c.conn.Close()
		log.Printf("[WS] readPump ended for client %s", c.userID)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	// Handle incoming pings from client - respond with pong
	c.conn.SetPingHandler(func(appData string) error {
		log.Printf("[WS] <<< PING received from client %s, sending PONG", c.userID)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		err := c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeWait))
		if err != nil {
			log.Printf("[WS] ❌ Failed to send PONG to client %s: %v", c.userID, err)
		} else {
			log.Printf("[WS] >>> PONG sent to client %s", c.userID)
		}
		return err
	})

	// Handle incoming pongs from client (response to our pings)
	c.conn.SetPongHandler(func(string) error {
		log.Printf("[WS] <<< PONG received from client %s - resetting read deadline", c.userID)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	log.Printf("[WS] readPump entering read loop for client %s", c.userID)
	readCount := 0
	for {
		if readCount == 0 {
			log.Printf("[WS] readPump calling ReadMessage() for first time for client %s", c.userID)
		}
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] ❌ Unexpected close error for client %s: %v", c.userID, err)
			} else if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("[WS] Client %s closed connection (expected): %v", c.userID, err)
			} else {
				log.Printf("[WS] ❌ Read error for client %s: %v", c.userID, err)
			}
			break
		}
		readCount++
		log.Printf("[WS] readPump got message #%d from client %s", readCount, c.userID)

		// Handle incoming messages (typing indicators, etc.)
		var wsMsg models.WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("[WS] Failed to unmarshal message from client %s: %v", c.userID, err)
			continue
		}

		// Don't log typing messages - too noisy
		if wsMsg.Type != models.WSTypeTyping {
			log.Printf("[WS] <<< Received message type '%s' from client %s", wsMsg.Type, c.userID)
		}

		switch wsMsg.Type {
		case models.WSTypeTyping:
			// Broadcast typing indicator to OTHER clients in channel (not back to sender)
			if payload, ok := wsMsg.Payload.(map[string]interface{}); ok {
				if channelID, ok := payload["channel_id"].(string); ok {
					c.hub.BroadcastToChannelExcept(channelID, c, models.WSMessage{
						Type: models.WSTypeTyping,
						Payload: map[string]string{
							"user_id":    c.userID,
							"channel_id": channelID,
						},
					})
				}
			}
		case "subscribe":
			// Subscribe to a new channel
			if payload, ok := wsMsg.Payload.(map[string]interface{}); ok {
				if channelID, ok := payload["channel_id"].(string); ok {
					c.channelsMu.Lock()
					c.channels[channelID] = true
					c.channelsMu.Unlock()
					log.Printf("[WS] Client %s subscribed to channel %s", c.userID, channelID)
				}
			}
		case "subscribe_app":
			// Subscribe to an app for live updates
			if payload, ok := wsMsg.Payload.(map[string]interface{}); ok {
				if appID, ok := payload["app_id"].(string); ok {
					c.appsMu.Lock()
					c.apps[appID] = true
					c.appsMu.Unlock()
					log.Printf("[WS] Client %s subscribed to app %s", c.userID, appID)
				}
			}
		case "unsubscribe_app":
			// Unsubscribe from an app
			if payload, ok := wsMsg.Payload.(map[string]interface{}); ok {
				if appID, ok := payload["app_id"].(string); ok {
					c.appsMu.Lock()
					delete(c.apps, appID)
					c.appsMu.Unlock()
					log.Printf("[WS] Client %s unsubscribed from app %s", c.userID, appID)
				}
			}
		default:
			log.Printf("[WS] Unknown message type '%s' from client %s", wsMsg.Type, c.userID)
		}
	}
}

func (c *Client) isSubscribed(channelID string) bool {
	c.channelsMu.RLock()
	defer c.channelsMu.RUnlock()
	return c.channels[channelID]
}

func (c *Client) isSubscribedToApp(appID string) bool {
	c.appsMu.RLock()
	defer c.appsMu.RUnlock()
	return c.apps[appID]
}

func (c *Client) writePump() {
	log.Printf("[WS] writePump started for client %s (ping interval: %v)", c.userID, pingPeriod)
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		log.Printf("[WS] writePump ended for client %s", c.userID)
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Printf("[WS] Send channel closed for client %s - sending close message", c.userID)
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[WS] ❌ Write error for client %s: %v", c.userID, err)
				return
			}
			// Log message type if we can parse it
			var wsMsg models.WSMessage
			if json.Unmarshal(message, &wsMsg) == nil {
				log.Printf("[WS] >>> Sent message type '%s' to client %s", wsMsg.Type, c.userID)
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			log.Printf("[WS] Sending PING to client %s", c.userID)
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[WS] ❌ Ping failed for client %s: %v", c.userID, err)
				return
			}
		}
	}
}
