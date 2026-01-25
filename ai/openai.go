package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type OpenAIClient struct {
	apiKey string
	model  string
}

func NewOpenAIClient(model string) *OpenAIClient {
	return &OpenAIClient{
		apiKey: os.Getenv("OPENAI_KEY"),
		model:  model,
	}
}

type ResponsesRequest struct {
	Model        string      `json:"model"`
	Input        interface{} `json:"input"` // Can be string or []InputMessage
	Instructions string      `json:"instructions,omitempty"`
	Stream       bool        `json:"stream,omitempty"`
	Tools        []Tool      `json:"tools,omitempty"`
}

// Tool represents a function tool definition for the Responses API
type Tool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool call from the model
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// StreamEvent represents a server-sent event from OpenAI streaming
type StreamEvent struct {
	Type  string          `json:"type"`
	Delta string          `json:"delta,omitempty"`
	Text  string          `json:"text,omitempty"`
	Error *ResponsesError `json:"error,omitempty"`
}

type ResponsesError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// InputMessage represents a conversation message for the API
type InputMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ContentPart
}

// ContentPart represents a content part in a message
type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewTextMessage creates a simple text message
func NewTextMessage(role, text string) InputMessage {
	return InputMessage{
		Role:    role,
		Content: text,
	}
}

type ResponsesResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output []struct {
		Type    string `json:"type"`
		ID      string `json:"id"`
		Status  string `json:"status"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *OpenAIClient) GetResponse(input string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENAI_KEY environment variable not set")
	}

	reqBody := ResponsesRequest{
		Model: c.model,
		Input: input,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response ResponsesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("OpenAI error: %s", response.Error.Message)
	}

	// Extract text from output
	for _, output := range response.Output {
		if output.Type == "message" && output.Role == "assistant" {
			for _, content := range output.Content {
				if content.Type == "output_text" {
					return content.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no text response found in OpenAI output")
}

func (c *OpenAIClient) IsConfigured() bool {
	return c.apiKey != ""
}

// GetResponseWithContext sends a request with conversation history for context
func (c *OpenAIClient) GetResponseWithContext(messages []InputMessage, systemPrompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENAI_KEY environment variable not set")
	}

	reqBody := ResponsesRequest{
		Model:        c.model,
		Input:        messages,
		Instructions: systemPrompt,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response ResponsesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("OpenAI error: %s", response.Error.Message)
	}

	// Extract text from output
	for _, output := range response.Output {
		if output.Type == "message" && output.Role == "assistant" {
			for _, content := range output.Content {
				if content.Type == "output_text" {
					return content.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no text response found in OpenAI output")
}

// StreamCallback is called for each text delta during streaming
type StreamCallback func(delta string, fullText string)

// StreamResponseWithContext sends a streaming request and calls the callback for each text chunk
func (c *OpenAIClient) StreamResponseWithContext(messages []InputMessage, systemPrompt string, callback StreamCallback) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENAI_KEY environment variable not set")
	}

	reqBody := ResponsesRequest{
		Model:        c.model,
		Input:        messages,
		Instructions: systemPrompt,
		Stream:       true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Debug log the request
	fmt.Printf("[AI DEBUG] Sending to OpenAI:\n  Model: %s\n  System: %s\n  Messages (%d):\n", c.model, systemPrompt, len(messages))
	for i, m := range messages {
		content := ""
		switch v := m.Content.(type) {
		case string:
			content = v
		default:
			content = fmt.Sprintf("%v", v)
		}
		if len(content) > 80 {
			content = content[:80] + "..."
		}
		fmt.Printf("    [%d] role=%s: %s\n", i, m.Role, content)
	}
	// Log last message specifically
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		fmt.Printf("[AI DEBUG] >>> LAST MESSAGE (should be user's question): role=%s\n", last.Role)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse SSE stream
	var fullText strings.Builder
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data lines
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				break
			}

			// Parse the JSON event
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			// Handle different event types
			eventType, _ := event["type"].(string)

			switch eventType {
			case "response.output_text.delta":
				// Extract the delta text
				if delta, ok := event["delta"].(string); ok {
					fullText.WriteString(delta)
					callback(delta, fullText.String())
				}
			case "response.output_text.done":
				// Final text received
				if text, ok := event["text"].(string); ok {
					fullText.Reset()
					fullText.WriteString(text)
				}
			case "error":
				if errData, ok := event["error"].(map[string]interface{}); ok {
					if msg, ok := errData["message"].(string); ok {
						return "", fmt.Errorf("OpenAI streaming error: %s", msg)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading stream: %w", err)
	}

	return fullText.String(), nil
}

// StreamResult contains the final result of a streaming response
type StreamResult struct {
	Text      string
	ToolCalls []ToolCall
}

// ToolStreamCallback is called for each event during streaming with tools
type ToolStreamCallback func(delta string, fullText string, toolCall *ToolCall)

// StreamResponseWithTools sends a streaming request with tool support
func (c *OpenAIClient) StreamResponseWithTools(messages []InputMessage, systemPrompt string, tools []Tool, callback ToolStreamCallback) (*StreamResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_KEY environment variable not set")
	}

	reqBody := ResponsesRequest{
		Model:        c.model,
		Input:        messages,
		Instructions: systemPrompt,
		Stream:       true,
		Tools:        tools,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse SSE stream
	var fullText strings.Builder
	var toolCalls []ToolCall
	var currentToolCall *ToolCall
	var toolArgs strings.Builder

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				break
			}

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)

			switch eventType {
			case "response.output_text.delta":
				if delta, ok := event["delta"].(string); ok {
					fullText.WriteString(delta)
					callback(delta, fullText.String(), nil)
				}

			case "response.output_text.done":
				if text, ok := event["text"].(string); ok {
					fullText.Reset()
					fullText.WriteString(text)
				}

			case "response.function_call_arguments.start":
				// Start of a function call
				currentToolCall = &ToolCall{}
				toolArgs.Reset()

			case "response.function_call_arguments.delta":
				if delta, ok := event["delta"].(string); ok {
					toolArgs.WriteString(delta)
				}

			case "response.function_call_arguments.done":
				if currentToolCall != nil {
					currentToolCall.Arguments = toolArgs.String()
				}

			case "response.output_item.done":
				// Check if this is a function call item
				if item, ok := event["item"].(map[string]interface{}); ok {
					if itemType, _ := item["type"].(string); itemType == "function_call" {
						tc := ToolCall{}
						if id, ok := item["call_id"].(string); ok {
							tc.ID = id
						}
						if name, ok := item["name"].(string); ok {
							tc.Name = name
						}
						if args, ok := item["arguments"].(string); ok {
							tc.Arguments = args
						}
						toolCalls = append(toolCalls, tc)
						callback("", fullText.String(), &tc)
					}
				}

			case "error":
				if errData, ok := event["error"].(map[string]interface{}); ok {
					if msg, ok := errData["message"].(string); ok {
						return nil, fmt.Errorf("OpenAI streaming error: %s", msg)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	return &StreamResult{
		Text:      fullText.String(),
		ToolCalls: toolCalls,
	}, nil
}

// TTSRequest represents a text-to-speech request
type TTSRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

// TextToSpeech converts text to speech using OpenAI's TTS API
// Returns the audio data as bytes (MP3 format)
func (c *OpenAIClient) TextToSpeech(text string, voice string) ([]byte, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_KEY environment variable not set")
	}

	if voice == "" {
		voice = "alloy" // Default voice
	}

	reqBody := TTSRequest{
		Model: "tts-1",
		Input: text,
		Voice: voice,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI TTS API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}
