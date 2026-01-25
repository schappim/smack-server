package models

import "time"

type Webhook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ChannelID string    `json:"channel_id"`
	Token     string    `json:"token"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateWebhookRequest struct {
	Name      string `json:"name"`
	ChannelID string `json:"channel_id"`
}

type IncomingWebhookRequest struct {
	Content    string `json:"content"`
	HTML       string `json:"html,omitempty"`
	WidgetSize string `json:"widget_size,omitempty"` // small, medium, large, xlarge
	Username   string `json:"username,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type WebhookResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ChannelID string    `json:"channel_id"`
	Token     string    `json:"token"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"url,omitempty"`
}
