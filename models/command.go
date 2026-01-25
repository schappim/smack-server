package models

import "time"

type CustomCommand struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	URL          string    `json:"url"`
	Method       string    `json:"method"`
	Headers      string    `json:"headers,omitempty"`
	BodyTemplate string    `json:"body_template,omitempty"`
	IsGlobal     bool      `json:"is_global"`
	CreatedBy    string    `json:"created_by"`
	ResponseMode string    `json:"response_mode"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateCommandRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	URL          string `json:"url"`
	Method       string `json:"method"`
	Headers      string `json:"headers,omitempty"`
	BodyTemplate string `json:"body_template,omitempty"`
	IsGlobal     bool   `json:"is_global"`
	ResponseMode string `json:"response_mode"`
}

type UpdateCommandRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	URL          *string `json:"url,omitempty"`
	Method       *string `json:"method,omitempty"`
	Headers      *string `json:"headers,omitempty"`
	BodyTemplate *string `json:"body_template,omitempty"`
	IsGlobal     *bool   `json:"is_global,omitempty"`
	ResponseMode *string `json:"response_mode,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

type ExecuteCommandRequest struct {
	CommandID string `json:"command_id"`
	Input     string `json:"input"`
	ChannelID string `json:"channel_id"`
}

type CommandExecutionResult struct {
	Success      bool   `json:"success"`
	StatusCode   int    `json:"status_code"`
	ResponseBody string `json:"response_body"`
	Error        string `json:"error,omitempty"`
}

type AIGenerateCommandRequest struct {
	Description string `json:"description"`
}

type AIGenerateCommandResponse struct {
	Command *CustomCommand `json:"command,omitempty"`
	Preview string         `json:"preview,omitempty"`
}
