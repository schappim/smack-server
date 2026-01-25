package models

import "time"

// App represents a user-created web application
type App struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	HTMLContent string    `json:"html_content"`
	CSSContent  string    `json:"css_content"`
	JSContent   string    `json:"js_content"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AppWithDetails includes additional computed fields
type AppWithDetails struct {
	App
	MemberCount int `json:"member_count"`
}

// AppMember represents a user's membership in an app
type AppMember struct {
	AppID    string       `json:"app_id"`
	UserID   string       `json:"user_id"`
	Role     string       `json:"role"` // "owner", "admin", "member"
	JoinedAt time.Time    `json:"joined_at"`
	User     UserResponse `json:"user,omitempty"`
}

// AppMessage represents a message in an app's AI conversation
type AppMessage struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Request types

type CreateAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

type UpdateAppRequest struct {
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	Icon        *string `json:"icon,omitempty"`
}

type UpdateAppCodeRequest struct {
	HTMLContent string `json:"html_content,omitempty"`
	CSSContent  string `json:"css_content,omitempty"`
	JSContent   string `json:"js_content,omitempty"`
}

type AppChatRequest struct {
	Message string `json:"message"`
}

type AppQueryRequest struct {
	Query  string        `json:"query"`
	Params []interface{} `json:"params,omitempty"`
}

type AppQueryResponse struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Error   string                   `json:"error,omitempty"`
}

type AddAppMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role,omitempty"` // defaults to "member"
}

// WebSocket event types for apps
const (
	WSTypeAppCodeUpdated   = "app_code_updated"
	WSTypeAppStreamStart   = "app_stream_start"
	WSTypeAppStreamDelta   = "app_stream_delta"
	WSTypeAppStreamEnd     = "app_stream_end"
)

// App WebSocket payloads
type AppCodeUpdatedPayload struct {
	AppID       string `json:"app_id"`
	HTMLContent string `json:"html_content"`
	CSSContent  string `json:"css_content"`
	JSContent   string `json:"js_content"`
	UpdatedAt   string `json:"updated_at"`
}

type AppStreamStartPayload struct {
	MessageID string `json:"message_id"`
	AppID     string `json:"app_id"`
}

type AppStreamDeltaPayload struct {
	MessageID string `json:"message_id"`
	AppID     string `json:"app_id"`
	Delta     string `json:"delta"`
	FullText  string `json:"full_text"`
}

type AppStreamEndPayload struct {
	MessageID string `json:"message_id"`
	AppID     string `json:"app_id"`
	Content   string `json:"content"`
}
