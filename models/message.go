package models

import "time"

type Message struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
	UserID      string    `json:"user_id"`
	Content     string    `json:"content"`
	HTMLContent *string   `json:"html_content,omitempty"`
	WidgetSize  *string   `json:"widget_size,omitempty"`
	ThreadID    *string   `json:"thread_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type MessageWithUser struct {
	Message
	User        UserResponse `json:"user"`
	ReplyCount  int          `json:"reply_count"`
	LatestReply *time.Time   `json:"latest_reply,omitempty"`
}

type SendMessageRequest struct {
	ChannelID string  `json:"channel_id"`
	Content   string  `json:"content"`
	ThreadID  *string `json:"thread_id,omitempty"`
}

// WebSocket message types
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

const (
	WSTypeNewMessage        = "new_message"
	WSTypeMessageDeleted    = "message_deleted"
	WSTypeUserOnline        = "user_online"
	WSTypeUserOffline       = "user_offline"
	WSTypeTyping            = "typing"
	WSTypeChannelUpdate     = "channel_update"
	WSTypeReactionUpdate    = "reaction_update"
	WSTypeMessageStreamStart = "message_stream_start"
	WSTypeMessageStreamDelta = "message_stream_delta"
	WSTypeMessageStreamEnd   = "message_stream_end"
)

// Streaming message payloads
type StreamStartPayload struct {
	MessageID string       `json:"message_id"`
	ChannelID string       `json:"channel_id"`
	User      UserResponse `json:"user"`
	ThreadID  *string      `json:"thread_id,omitempty"`
}

type StreamDeltaPayload struct {
	MessageID string `json:"message_id"`
	ChannelID string `json:"channel_id"`
	Delta     string `json:"delta"`
	FullText  string `json:"full_text"`
}

type StreamEndPayload struct {
	MessageID string `json:"message_id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
}
