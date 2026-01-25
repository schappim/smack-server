package models

import "time"

type Reaction struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	UserID    string    `json:"user_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

type ReactionWithUser struct {
	Reaction
	User UserResponse `json:"user"`
}

// ReactionGroup groups reactions by emoji with user list
type ReactionGroup struct {
	Emoji string         `json:"emoji"`
	Count int            `json:"count"`
	Users []UserResponse `json:"users"`
}

type AddReactionRequest struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

type RemoveReactionRequest struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}
