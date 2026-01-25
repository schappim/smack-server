package handlers

import (
	"encoding/json"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"strings"
	"time"
)

type ReminderHandler struct {
	store *store.Store
	hub   *Hub
}

func NewReminderHandler(s *store.Store, h *Hub) *ReminderHandler {
	return &ReminderHandler{store: s, hub: h}
}

func (h *ReminderHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.CreateReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Parse the remind_at time
	remindAt, err := parseRemindTime(req.RemindAt)
	if err != nil {
		http.Error(w, "Invalid time format: "+err.Error(), http.StatusBadRequest)
		return
	}

	reminder, err := h.store.CreateReminder(userID, req.ChannelID, req.Message, remindAt)
	if err != nil {
		http.Error(w, "Failed to create reminder", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(reminder)
}

func (h *ReminderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	reminders, err := h.store.GetRemindersForUser(userID)
	if err != nil {
		http.Error(w, "Failed to fetch reminders", http.StatusInternalServerError)
		return
	}

	if reminders == nil {
		reminders = []models.Reminder{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reminders)
}

func (h *ReminderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	reminderID := r.PathValue("id")

	if reminderID == "" {
		http.Error(w, "Reminder ID required", http.StatusBadRequest)
		return
	}

	err := h.store.DeleteReminder(reminderID, userID)
	if err != nil {
		http.Error(w, "Failed to delete reminder", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// StartReminderChecker starts a goroutine that checks for due reminders
func (h *ReminderHandler) StartReminderChecker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			h.checkAndSendReminders()
		}
	}()
}

func (h *ReminderHandler) checkAndSendReminders() {
	reminders, err := h.store.GetDueReminders()
	if err != nil {
		return
	}

	for _, reminder := range reminders {
		// Get or create Smackbot DM channel with the user
		dmChannel, err := h.store.GetOrCreateSmackbotDM(reminder.UserID)
		if err != nil {
			continue
		}

		// Get Smackbot user info for the message
		smackbot, err := h.store.GetSmackbot()
		if err != nil {
			continue
		}

		// Create a message from Smackbot
		reminderMsg := "🔔 **Reminder:** " + reminder.Message
		msg, err := h.store.CreateMessage(dmChannel.ID, smackbot.ID, reminderMsg, nil)
		if err != nil {
			continue
		}

		msgWithUser := models.MessageWithUser{
			Message: *msg,
			User:    smackbot.ToResponse(),
		}

		// Broadcast the message to the user
		h.hub.BroadcastToChannel(dmChannel.ID, models.WSMessage{
			Type:    models.WSTypeNewMessage,
			Payload: msgWithUser,
		})

		// Also send reminder notification via WebSocket
		h.hub.SendToUser(reminder.UserID, models.WSMessage{
			Type:    models.WSTypeReminder,
			Payload: reminder,
		})

		// Mark as completed
		h.store.MarkReminderComplete(reminder.ID)
	}
}

// parseRemindTime parses various time formats
func parseRemindTime(input string) (time.Time, error) {
	input = strings.TrimSpace(strings.ToLower(input))

	// Try ISO 8601 first
	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t, nil
	}

	// Try common formats
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, input); err == nil {
			return t, nil
		}
	}

	// Parse relative times like "in 5 minutes", "in 1 hour", "tomorrow"
	now := time.Now()

	if strings.HasPrefix(input, "in ") {
		return parseRelativeTime(input[3:], now)
	}

	switch input {
	case "tomorrow":
		return now.Add(24 * time.Hour), nil
	case "next week":
		return now.Add(7 * 24 * time.Hour), nil
	}

	// Try parsing as duration directly
	return parseRelativeTime(input, now)
}

func parseRelativeTime(input string, now time.Time) (time.Time, error) {
	input = strings.TrimSpace(input)

	// Parse "5 minutes", "1 hour", "2 days", etc.
	var value int
	var unit string

	_, err := parseTimeComponents(input, &value, &unit)
	if err != nil {
		return time.Time{}, err
	}

	unit = strings.TrimSuffix(unit, "s") // normalize plural

	switch unit {
	case "second":
		return now.Add(time.Duration(value) * time.Second), nil
	case "minute", "min":
		return now.Add(time.Duration(value) * time.Minute), nil
	case "hour", "hr":
		return now.Add(time.Duration(value) * time.Hour), nil
	case "day":
		return now.Add(time.Duration(value) * 24 * time.Hour), nil
	case "week":
		return now.Add(time.Duration(value) * 7 * 24 * time.Hour), nil
	default:
		return time.Time{}, &time.ParseError{Message: "unknown time unit: " + unit}
	}
}

func parseTimeComponents(input string, value *int, unit *string) (bool, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		// Try parsing "5min", "1hr" format
		for i, c := range input {
			if c < '0' || c > '9' {
				numStr := input[:i]
				*unit = input[i:]
				_, err := parseNumber(numStr, value)
				return err == nil, err
			}
		}
		return false, &time.ParseError{Message: "invalid time format"}
	}

	_, err := parseNumber(parts[0], value)
	if err != nil {
		return false, err
	}
	*unit = parts[1]
	return true, nil
}

func parseNumber(s string, result *int) (bool, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, &time.ParseError{Message: "invalid number"}
		}
		n = n*10 + int(c-'0')
	}
	*result = n
	return true, nil
}
