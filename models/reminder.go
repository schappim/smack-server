package models

import "time"

type Reminder struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ChannelID string    `json:"channel_id"`
	Message   string    `json:"message"`
	RemindAt  time.Time `json:"remind_at"`
	CreatedAt time.Time `json:"created_at"`
	Completed bool      `json:"completed"`
}

type CreateReminderRequest struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	RemindAt  string `json:"remind_at"` // ISO 8601 format or relative like "in 5 minutes"
}

const (
	WSTypeReminder = "reminder"
)
