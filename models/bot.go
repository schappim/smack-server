package models

import "time"

type Bot struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description,omitempty"`
	Provider    string    `json:"provider"` // "openai", "anthropic", etc.
	Model       string    `json:"model"`    // "gpt-5.2", etc.
	AvatarURL   string    `json:"avatar_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type BotResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
	Provider    string `json:"provider"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

func (b *Bot) ToResponse() BotResponse {
	return BotResponse{
		ID:          b.ID,
		Name:        b.Name,
		DisplayName: b.DisplayName,
		Description: b.Description,
		Provider:    b.Provider,
		AvatarURL:   b.AvatarURL,
	}
}

type CreateBotDMRequest struct {
	BotID string `json:"bot_id"`
}
