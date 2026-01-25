package models

import "time"

type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsDirect    bool      `json:"is_direct"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type ChannelMember struct {
	ChannelID string    `json:"channel_id"`
	UserID    string    `json:"user_id"`
	JoinedAt  time.Time `json:"joined_at"`
}

type CreateChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type UpdateChannelRequest struct {
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type ChannelWithMembers struct {
	Channel
	Members     []UserResponse `json:"members"`
	UnreadCount int            `json:"unread_count"`
}

type ChannelWithUnread struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsDirect    bool      `json:"is_direct"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UnreadCount int       `json:"unread_count"`
}
