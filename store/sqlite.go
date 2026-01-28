package store

import (
	"database/sql"
	"smack-server/models"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.init(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		display_name TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		avatar_url TEXT,
		status TEXT DEFAULT 'offline',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS channels (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		is_direct BOOLEAN DEFAULT FALSE,
		created_by TEXT REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS channel_members (
		channel_id TEXT REFERENCES channels(id),
		user_id TEXT REFERENCES users(id),
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (channel_id, user_id)
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		channel_id TEXT REFERENCES channels(id),
		user_id TEXT REFERENCES users(id),
		content TEXT NOT NULL,
		html_content TEXT,
		widget_size TEXT,
		thread_id TEXT REFERENCES messages(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel_id);
	CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id);
	CREATE INDEX IF NOT EXISTS idx_channel_members_user ON channel_members(user_id);

	CREATE TABLE IF NOT EXISTS reminders (
		id TEXT PRIMARY KEY,
		user_id TEXT REFERENCES users(id),
		channel_id TEXT REFERENCES channels(id),
		message TEXT NOT NULL,
		remind_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed BOOLEAN DEFAULT FALSE
	);

	CREATE INDEX IF NOT EXISTS idx_reminders_user ON reminders(user_id);
	CREATE INDEX IF NOT EXISTS idx_reminders_time ON reminders(remind_at);

	CREATE TABLE IF NOT EXISTS muted_channels (
		user_id TEXT REFERENCES users(id),
		channel_id TEXT REFERENCES channels(id),
		muted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, channel_id)
	);

	CREATE TABLE IF NOT EXISTS bots (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		display_name TEXT NOT NULL,
		description TEXT,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		avatar_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS bot_channels (
		channel_id TEXT PRIMARY KEY REFERENCES channels(id),
		bot_id TEXT NOT NULL REFERENCES bots(id),
		user_id TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS reactions (
		id TEXT PRIMARY KEY,
		message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL REFERENCES users(id),
		emoji TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(message_id, user_id, emoji)
	);

	CREATE INDEX IF NOT EXISTS idx_reactions_message ON reactions(message_id);

	CREATE TABLE IF NOT EXISTS webhooks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		channel_id TEXT NOT NULL REFERENCES channels(id),
		token TEXT NOT NULL,
		created_by TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_webhooks_channel ON webhooks(channel_id);
	CREATE INDEX IF NOT EXISTS idx_webhooks_token ON webhooks(token);

	-- Kanban boards
	CREATE TABLE IF NOT EXISTS kanban_boards (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		icon TEXT,
		created_by TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Board members
	CREATE TABLE IF NOT EXISTS kanban_board_members (
		board_id TEXT NOT NULL REFERENCES kanban_boards(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL REFERENCES users(id),
		role TEXT DEFAULT 'member',
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (board_id, user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_kanban_board_members_user ON kanban_board_members(user_id);

	-- Kanban columns
	CREATE TABLE IF NOT EXISTS kanban_columns (
		id TEXT PRIMARY KEY,
		board_id TEXT NOT NULL REFERENCES kanban_boards(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		position INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_kanban_columns_board ON kanban_columns(board_id);

	-- Kanban labels
	CREATE TABLE IF NOT EXISTS kanban_labels (
		id TEXT PRIMARY KEY,
		board_id TEXT NOT NULL REFERENCES kanban_boards(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		color TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_kanban_labels_board ON kanban_labels(board_id);

	-- Kanban cards
	CREATE TABLE IF NOT EXISTS kanban_cards (
		id TEXT PRIMARY KEY,
		column_id TEXT NOT NULL REFERENCES kanban_columns(id) ON DELETE CASCADE,
		board_id TEXT NOT NULL REFERENCES kanban_boards(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		description TEXT,
		position INTEGER NOT NULL DEFAULT 0,
		due_date DATETIME,
		created_by TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_kanban_cards_column ON kanban_cards(column_id);
	CREATE INDEX IF NOT EXISTS idx_kanban_cards_board ON kanban_cards(board_id);

	-- Card assignees
	CREATE TABLE IF NOT EXISTS kanban_card_assignees (
		card_id TEXT NOT NULL REFERENCES kanban_cards(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL REFERENCES users(id),
		assigned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (card_id, user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_kanban_card_assignees_user ON kanban_card_assignees(user_id);

	-- Card labels (many-to-many)
	CREATE TABLE IF NOT EXISTS kanban_card_labels (
		card_id TEXT NOT NULL REFERENCES kanban_cards(id) ON DELETE CASCADE,
		label_id TEXT NOT NULL REFERENCES kanban_labels(id) ON DELETE CASCADE,
		PRIMARY KEY (card_id, label_id)
	);

	-- Kanban comments
	CREATE TABLE IF NOT EXISTS kanban_comments (
		id TEXT PRIMARY KEY,
		card_id TEXT NOT NULL REFERENCES kanban_cards(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL REFERENCES users(id),
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_kanban_comments_card ON kanban_comments(card_id);

	-- Apps
	CREATE TABLE IF NOT EXISTS apps (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		icon TEXT,
		html_content TEXT NOT NULL DEFAULT '',
		css_content TEXT NOT NULL DEFAULT '',
		js_content TEXT NOT NULL DEFAULT '',
		created_by TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- App members
	CREATE TABLE IF NOT EXISTS app_members (
		app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL REFERENCES users(id),
		role TEXT DEFAULT 'member',
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (app_id, user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_app_members_user ON app_members(user_id);

	-- App conversation history (for AI context)
	CREATE TABLE IF NOT EXISTS app_messages (
		id TEXT PRIMARY KEY,
		app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL REFERENCES users(id),
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_app_messages_app ON app_messages(app_id);

	-- Custom commands
	CREATE TABLE IF NOT EXISTS custom_commands (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		url TEXT NOT NULL,
		method TEXT NOT NULL DEFAULT 'GET',
		headers TEXT,
		body_template TEXT,
		is_global BOOLEAN DEFAULT FALSE,
		created_by TEXT NOT NULL REFERENCES users(id),
		response_mode TEXT DEFAULT 'private',
		enabled BOOLEAN DEFAULT TRUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_custom_commands_name ON custom_commands(name);
	CREATE INDEX IF NOT EXISTS idx_custom_commands_created_by ON custom_commands(created_by);

	-- User preferences (JSON key-value per user)
	CREATE TABLE IF NOT EXISTS user_preferences (
		user_id TEXT NOT NULL REFERENCES users(id),
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, key)
	);

	-- Server settings (key-value)
	CREATE TABLE IF NOT EXISTS server_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Run migrations for existing databases
	s.runMigrations()

	// Create default #general channel if it doesn't exist
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM channels WHERE name = 'general'").Scan(&count)
	if count == 0 {
		_, err = s.db.Exec(`
			INSERT INTO channels (id, name, description, is_direct, created_by)
			VALUES (?, 'general', 'General discussion', FALSE, 'system')
		`, uuid.New().String())
	}

	// Create Smackbot system user if it doesn't exist
	s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'smackbot'").Scan(&count)
	if count == 0 {
		_, err = s.db.Exec(`
			INSERT INTO users (id, username, display_name, password_hash, avatar_url, status, created_at)
			VALUES ('smackbot', 'smackbot', 'Smackbot', '', '', 'online', CURRENT_TIMESTAMP)
		`)
	}

	return err
}

func (s *Store) runMigrations() {
	var count int

	// Add html_content column to messages table if it doesn't exist
	s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='html_content'`).Scan(&count)
	if count == 0 {
		s.db.Exec(`ALTER TABLE messages ADD COLUMN html_content TEXT`)
	}

	// Add widget_size column to messages table if it doesn't exist
	s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='widget_size'`).Scan(&count)
	if count == 0 {
		s.db.Exec(`ALTER TABLE messages ADD COLUMN widget_size TEXT`)
	}

	// Add icon column to apps table if it doesn't exist
	s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('apps') WHERE name='icon'`).Scan(&count)
	if count == 0 {
		s.db.Exec(`ALTER TABLE apps ADD COLUMN icon TEXT`)
	}

	// Add icon column to kanban_boards table if it doesn't exist
	s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('kanban_boards') WHERE name='icon'`).Scan(&count)
	if count == 0 {
		s.db.Exec(`ALTER TABLE kanban_boards ADD COLUMN icon TEXT`)
	}
}

// GetSmackbot returns the Smackbot system user
func (s *Store) GetSmackbot() (*models.User, error) {
	return s.GetUserByUsername("smackbot")
}

// GetOrCreateSmackbotDM gets or creates a DM channel between Smackbot and a user
func (s *Store) GetOrCreateSmackbotDM(userID string) (*models.Channel, error) {
	// Get Smackbot user
	smackbot, err := s.GetSmackbot()
	if err != nil {
		return nil, err
	}

	// Check if DM channel already exists
	var channelID string
	err = s.db.QueryRow(`
		SELECT c.id FROM channels c
		JOIN channel_members cm1 ON c.id = cm1.channel_id AND cm1.user_id = ?
		JOIN channel_members cm2 ON c.id = cm2.channel_id AND cm2.user_id = ?
		WHERE c.is_direct = TRUE
	`, smackbot.ID, userID).Scan(&channelID)

	if err == nil {
		return s.GetChannel(channelID)
	}

	// Create new DM channel with Smackbot
	channelName := "dm-smackbot-" + userID[:8]
	channel, err := s.CreateChannel(channelName, "", smackbot.ID, true)
	if err != nil {
		return nil, err
	}

	// Add the user to the channel
	s.JoinChannel(channel.ID, userID)

	return channel, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// User operations

func (s *Store) CreateUser(username, displayName, password string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		ID:           uuid.New().String(),
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		Status:       "online",
		CreatedAt:    time.Now(),
	}

	_, err = s.db.Exec(`
		INSERT INTO users (id, username, display_name, password_hash, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.DisplayName, user.PasswordHash, user.Status, user.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Auto-join the general channel
	var generalID string
	s.db.QueryRow("SELECT id FROM channels WHERE name = 'general'").Scan(&generalID)
	if generalID != "" {
		s.JoinChannel(generalID, user.ID)
	}

	return user, nil
}

func (s *Store) GetUserByUsername(username string) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(`
		SELECT id, username, display_name, password_hash, COALESCE(avatar_url, ''), status, created_at
		FROM users WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.DisplayName, &user.PasswordHash, &user.AvatarURL, &user.Status, &user.CreatedAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) GetUserByID(id string) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(`
		SELECT id, username, display_name, password_hash, COALESCE(avatar_url, ''), status, created_at
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.DisplayName, &user.PasswordHash, &user.AvatarURL, &user.Status, &user.CreatedAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) GetAllUsers() ([]models.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, display_name, COALESCE(avatar_url, ''), status, created_at
		FROM users ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Status, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) UpdateUserStatus(userID, status string) error {
	_, err := s.db.Exec("UPDATE users SET status = ? WHERE id = ?", status, userID)
	return err
}

func (s *Store) UpdateUserAvatar(userID, avatarURL string) error {
	_, err := s.db.Exec("UPDATE users SET avatar_url = ? WHERE id = ?", avatarURL, userID)
	return err
}

func (s *Store) UpdateUserDisplayName(userID, displayName string) error {
	_, err := s.db.Exec("UPDATE users SET display_name = ? WHERE id = ?", displayName, userID)
	return err
}

func (s *Store) ValidatePassword(user *models.User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// Channel operations

func (s *Store) CreateChannel(name, description, createdBy string, isDirect bool) (*models.Channel, error) {
	channel := &models.Channel{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		IsDirect:    isDirect,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO channels (id, name, description, is_direct, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, channel.ID, channel.Name, channel.Description, channel.IsDirect, channel.CreatedBy, channel.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Creator auto-joins
	s.JoinChannel(channel.ID, createdBy)

	return channel, nil
}

func (s *Store) GetChannel(id string) (*models.Channel, error) {
	channel := &models.Channel{}
	err := s.db.QueryRow(`
		SELECT id, name, COALESCE(description, ''), is_direct, created_by, created_at
		FROM channels WHERE id = ?
	`, id).Scan(&channel.ID, &channel.Name, &channel.Description, &channel.IsDirect, &channel.CreatedBy, &channel.CreatedAt)

	if err != nil {
		return nil, err
	}
	return channel, nil
}

func (s *Store) UpdateChannel(id, name, description string) error {
	_, err := s.db.Exec(`
		UPDATE channels SET name = ?, description = ? WHERE id = ?
	`, name, description, id)
	return err
}

func (s *Store) GetChannelsForUser(userID string) ([]models.ChannelWithUnread, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, COALESCE(c.description, ''), c.is_direct, c.created_by, c.created_at,
			   (SELECT COUNT(*) FROM messages m
			    WHERE m.channel_id = c.id
			    AND m.thread_id IS NULL
			    AND m.created_at > COALESCE(cm.last_read_at, '1970-01-01')) as unread_count
		FROM channels c
		JOIN channel_members cm ON c.id = cm.channel_id
		WHERE cm.user_id = ?
		ORDER BY c.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.ChannelWithUnread
	for rows.Next() {
		var c models.ChannelWithUnread
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.IsDirect, &c.CreatedBy, &c.CreatedAt, &c.UnreadCount)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}

	// For DM channels, replace the name with the other participant's display name
	for i, c := range channels {
		if c.IsDirect {
			otherUser, err := s.GetDMOtherParticipant(c.ID, userID)
			if err == nil && otherUser != nil {
				channels[i].Name = otherUser.DisplayName
			}
		}
	}

	return channels, nil
}

// GetDMOtherParticipant returns the other user in a DM channel
func (s *Store) GetDMOtherParticipant(channelID, currentUserID string) (*models.User, error) {
	var user models.User
	err := s.db.QueryRow(`
		SELECT u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM users u
		JOIN channel_members cm ON u.id = cm.user_id
		WHERE cm.channel_id = ? AND u.id != ?
		LIMIT 1
	`, channelID, currentUserID).Scan(&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Status, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetPublicChannels() ([]models.Channel, error) {
	rows, err := s.db.Query(`
		SELECT id, name, COALESCE(description, ''), is_direct, created_by, created_at
		FROM channels WHERE is_direct = FALSE
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.Channel
	for rows.Next() {
		var c models.Channel
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.IsDirect, &c.CreatedBy, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (s *Store) JoinChannel(channelID, userID string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO channel_members (channel_id, user_id, joined_at, last_read_at)
		VALUES (?, ?, ?, ?)
	`, channelID, userID, time.Now(), time.Now())
	return err
}

func (s *Store) GetChannelMembers(channelID string) ([]models.User, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM users u
		JOIN channel_members cm ON u.id = cm.user_id
		WHERE cm.channel_id = ?
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Status, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) GetOrCreateDMChannel(user1ID, user2ID string) (*models.Channel, error) {
	// Check if DM channel already exists
	var channelID string
	err := s.db.QueryRow(`
		SELECT c.id FROM channels c
		JOIN channel_members cm1 ON c.id = cm1.channel_id AND cm1.user_id = ?
		JOIN channel_members cm2 ON c.id = cm2.channel_id AND cm2.user_id = ?
		WHERE c.is_direct = TRUE
	`, user1ID, user2ID).Scan(&channelID)

	if err == nil {
		return s.GetChannel(channelID)
	}

	// Create new DM channel
	user2, _ := s.GetUserByID(user2ID)
	channelName := "dm-" + user1ID[:8] + "-" + user2ID[:8]

	channel, err := s.CreateChannel(channelName, "", user1ID, true)
	if err != nil {
		return nil, err
	}

	// Set display name to other user's name
	channel.Name = user2.DisplayName

	// Add second user
	s.JoinChannel(channel.ID, user2ID)

	return channel, nil
}

// Message operations

func (s *Store) CreateMessage(channelID, userID, content string, threadID *string) (*models.Message, error) {
	return s.CreateMessageWithHTML(channelID, userID, content, nil, nil, threadID)
}

func (s *Store) CreateMessageWithHTML(channelID, userID, content string, htmlContent *string, widgetSize *string, threadID *string) (*models.Message, error) {
	msg := &models.Message{
		ID:          uuid.New().String(),
		ChannelID:   channelID,
		UserID:      userID,
		Content:     content,
		HTMLContent: htmlContent,
		WidgetSize:  widgetSize,
		ThreadID:    threadID,
		CreatedAt:   time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO messages (id, channel_id, user_id, content, html_content, widget_size, thread_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.ChannelID, msg.UserID, msg.Content, msg.HTMLContent, msg.WidgetSize, msg.ThreadID, msg.CreatedAt)

	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Store) UpdateMessageContent(messageID, content string) error {
	_, err := s.db.Exec(`UPDATE messages SET content = ? WHERE id = ?`, content, messageID)
	return err
}

func (s *Store) GetChannelMessages(channelID string, limit int) ([]models.MessageWithUser, error) {
	return s.GetChannelMessagesBefore(channelID, limit, nil)
}

// GetChannelMessagesBefore fetches messages before a given timestamp (for pagination)
func (s *Store) GetChannelMessagesBefore(channelID string, limit int, before *time.Time) ([]models.MessageWithUser, error) {
	var rows *sql.Rows
	var err error

	if before != nil {
		rows, err = s.db.Query(`
			SELECT m.id, m.channel_id, m.user_id, m.content, m.html_content, m.widget_size, m.thread_id, m.created_at,
				   u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at,
				   (SELECT COUNT(*) FROM messages WHERE thread_id = m.id) as reply_count,
				   (SELECT MAX(created_at) FROM messages WHERE thread_id = m.id) as latest_reply
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.channel_id = ? AND m.thread_id IS NULL AND m.created_at < ?
			ORDER BY m.created_at DESC
			LIMIT ?
		`, channelID, before.Format("2006-01-02 15:04:05.999999"), limit)
	} else {
		rows, err = s.db.Query(`
			SELECT m.id, m.channel_id, m.user_id, m.content, m.html_content, m.widget_size, m.thread_id, m.created_at,
				   u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at,
				   (SELECT COUNT(*) FROM messages WHERE thread_id = m.id) as reply_count,
				   (SELECT MAX(created_at) FROM messages WHERE thread_id = m.id) as latest_reply
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.channel_id = ? AND m.thread_id IS NULL
			ORDER BY m.created_at DESC
			LIMIT ?
		`, channelID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.MessageWithUser
	for rows.Next() {
		var msg models.MessageWithUser
		var user models.User
		var htmlContent sql.NullString
		var widgetSize sql.NullString
		var threadID sql.NullString
		var latestReplyStr sql.NullString

		err := rows.Scan(
			&msg.ID, &msg.ChannelID, &msg.UserID, &msg.Content, &htmlContent, &widgetSize, &threadID, &msg.CreatedAt,
			&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Status, &user.CreatedAt,
			&msg.ReplyCount, &latestReplyStr,
		)
		if err != nil {
			return nil, err
		}

		if htmlContent.Valid {
			msg.HTMLContent = &htmlContent.String
		}
		if widgetSize.Valid {
			msg.WidgetSize = &widgetSize.String
		}
		if threadID.Valid {
			msg.ThreadID = &threadID.String
		}
		if latestReplyStr.Valid && latestReplyStr.String != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", latestReplyStr.String); err == nil {
				msg.LatestReply = &t
			}
		}

		msg.User = user.ToResponse()
		messages = append(messages, msg)
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (s *Store) GetThreadMessages(threadID string) ([]models.MessageWithUser, error) {
	// First get the parent message
	rows, err := s.db.Query(`
		SELECT m.id, m.channel_id, m.user_id, m.content, m.html_content, m.widget_size, m.thread_id, m.created_at,
			   u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM messages m
		JOIN users u ON m.user_id = u.id
		WHERE m.id = ? OR m.thread_id = ?
		ORDER BY m.created_at ASC
	`, threadID, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.MessageWithUser
	for rows.Next() {
		var msg models.MessageWithUser
		var user models.User
		var htmlContent sql.NullString
		var widgetSize sql.NullString
		var tid sql.NullString

		err := rows.Scan(
			&msg.ID, &msg.ChannelID, &msg.UserID, &msg.Content, &htmlContent, &widgetSize, &tid, &msg.CreatedAt,
			&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Status, &user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if htmlContent.Valid {
			msg.HTMLContent = &htmlContent.String
		}
		if widgetSize.Valid {
			msg.WidgetSize = &widgetSize.String
		}
		if tid.Valid {
			msg.ThreadID = &tid.String
		}
		msg.User = user.ToResponse()
		messages = append(messages, msg)
	}

	return messages, nil
}

func (s *Store) GetMessage(id string) (*models.Message, error) {
	msg := &models.Message{}
	var htmlContent sql.NullString
	var widgetSize sql.NullString
	var threadID sql.NullString

	err := s.db.QueryRow(`
		SELECT id, channel_id, user_id, content, html_content, widget_size, thread_id, created_at
		FROM messages WHERE id = ?
	`, id).Scan(&msg.ID, &msg.ChannelID, &msg.UserID, &msg.Content, &htmlContent, &widgetSize, &threadID, &msg.CreatedAt)

	if err != nil {
		return nil, err
	}

	if htmlContent.Valid {
		msg.HTMLContent = &htmlContent.String
	}
	if widgetSize.Valid {
		msg.WidgetSize = &widgetSize.String
	}
	if threadID.Valid {
		msg.ThreadID = &threadID.String
	}

	return msg, nil
}

func (s *Store) DeleteMessage(id string) error {
	// First delete any replies to this message
	_, err := s.db.Exec("DELETE FROM messages WHERE thread_id = ?", id)
	if err != nil {
		return err
	}

	// Then delete the message itself
	_, err = s.db.Exec("DELETE FROM messages WHERE id = ?", id)
	return err
}

// ClearChannelMessages deletes all messages in a channel
func (s *Store) ClearChannelMessages(channelID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE channel_id = ?", channelID)
	return err
}

func (s *Store) MarkChannelAsRead(channelID, userID string) error {
	_, err := s.db.Exec(`
		UPDATE channel_members
		SET last_read_at = ?
		WHERE channel_id = ? AND user_id = ?
	`, time.Now(), channelID, userID)
	return err
}

// Reminder operations

func (s *Store) CreateReminder(userID, channelID, message string, remindAt time.Time) (*models.Reminder, error) {
	reminder := &models.Reminder{
		ID:        uuid.New().String(),
		UserID:    userID,
		ChannelID: channelID,
		Message:   message,
		RemindAt:  remindAt,
		CreatedAt: time.Now(),
		Completed: false,
	}

	_, err := s.db.Exec(`
		INSERT INTO reminders (id, user_id, channel_id, message, remind_at, created_at, completed)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, reminder.ID, reminder.UserID, reminder.ChannelID, reminder.Message, reminder.RemindAt, reminder.CreatedAt, reminder.Completed)

	if err != nil {
		return nil, err
	}
	return reminder, nil
}

func (s *Store) GetRemindersForUser(userID string) ([]models.Reminder, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, channel_id, message, remind_at, created_at, completed
		FROM reminders
		WHERE user_id = ? AND completed = FALSE
		ORDER BY remind_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []models.Reminder
	for rows.Next() {
		var r models.Reminder
		err := rows.Scan(&r.ID, &r.UserID, &r.ChannelID, &r.Message, &r.RemindAt, &r.CreatedAt, &r.Completed)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}
	return reminders, nil
}

func (s *Store) GetDueReminders() ([]models.Reminder, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, channel_id, message, remind_at, created_at, completed
		FROM reminders
		WHERE completed = FALSE AND remind_at <= ?
		ORDER BY remind_at ASC
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []models.Reminder
	for rows.Next() {
		var r models.Reminder
		err := rows.Scan(&r.ID, &r.UserID, &r.ChannelID, &r.Message, &r.RemindAt, &r.CreatedAt, &r.Completed)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}
	return reminders, nil
}

func (s *Store) MarkReminderComplete(id string) error {
	_, err := s.db.Exec("UPDATE reminders SET completed = TRUE WHERE id = ?", id)
	return err
}

func (s *Store) DeleteReminder(id, userID string) error {
	_, err := s.db.Exec("DELETE FROM reminders WHERE id = ? AND user_id = ?", id, userID)
	return err
}

// Muted channels operations

func (s *Store) MuteChannel(userID, channelID string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO muted_channels (user_id, channel_id, muted_at)
		VALUES (?, ?, ?)
	`, userID, channelID, time.Now())
	return err
}

func (s *Store) UnmuteChannel(userID, channelID string) error {
	_, err := s.db.Exec("DELETE FROM muted_channels WHERE user_id = ? AND channel_id = ?", userID, channelID)
	return err
}

func (s *Store) IsChannelMuted(userID, channelID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM muted_channels WHERE user_id = ? AND channel_id = ?", userID, channelID).Scan(&count)
	return count > 0, err
}

func (s *Store) GetMutedChannels(userID string) ([]string, error) {
	rows, err := s.db.Query("SELECT channel_id FROM muted_channels WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channelIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		channelIDs = append(channelIDs, id)
	}
	return channelIDs, nil
}

// Leave channel operation

func (s *Store) LeaveChannel(channelID, userID string) error {
	_, err := s.db.Exec("DELETE FROM channel_members WHERE channel_id = ? AND user_id = ?", channelID, userID)
	return err
}

// Bot operations

// EnsureBotUser ensures a user entry exists for a bot (needed for message foreign keys)
func (s *Store) EnsureBotUser(id, username, displayName, avatarURL string) {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO users (id, username, display_name, password_hash, avatar_url, status, created_at)
		VALUES (?, ?, ?, '', ?, 'online', ?)
	`, id, username, displayName, avatarURL, time.Now())
	if err != nil {
		// Log but don't fail - user might already exist
		return
	}
}

func (s *Store) CreateBot(id, name, displayName, description, provider, model, avatarURL string) (*models.Bot, error) {
	bot := &models.Bot{
		ID:          id,
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Provider:    provider,
		Model:       model,
		AvatarURL:   avatarURL,
		CreatedAt:   time.Now(),
	}

	// Create a corresponding user entry for the bot (needed for message foreign keys)
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO users (id, username, display_name, password_hash, avatar_url, status, created_at)
		VALUES (?, ?, ?, '', ?, 'online', ?)
	`, bot.ID, "bot-"+bot.Name, bot.DisplayName, bot.AvatarURL, bot.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO bots (id, name, display_name, description, provider, model, avatar_url, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, bot.ID, bot.Name, bot.DisplayName, bot.Description, bot.Provider, bot.Model, bot.AvatarURL, bot.CreatedAt)

	if err != nil {
		return nil, err
	}
	return bot, nil
}

func (s *Store) GetBot(id string) (*models.Bot, error) {
	bot := &models.Bot{}
	err := s.db.QueryRow(`
		SELECT id, name, display_name, COALESCE(description, ''), provider, model, COALESCE(avatar_url, ''), created_at
		FROM bots WHERE id = ?
	`, id).Scan(&bot.ID, &bot.Name, &bot.DisplayName, &bot.Description, &bot.Provider, &bot.Model, &bot.AvatarURL, &bot.CreatedAt)

	if err != nil {
		return nil, err
	}
	return bot, nil
}

func (s *Store) GetBotByName(name string) (*models.Bot, error) {
	bot := &models.Bot{}
	err := s.db.QueryRow(`
		SELECT id, name, display_name, COALESCE(description, ''), provider, model, COALESCE(avatar_url, ''), created_at
		FROM bots WHERE name = ?
	`, name).Scan(&bot.ID, &bot.Name, &bot.DisplayName, &bot.Description, &bot.Provider, &bot.Model, &bot.AvatarURL, &bot.CreatedAt)

	if err != nil {
		return nil, err
	}
	return bot, nil
}

func (s *Store) GetAllBots() ([]models.Bot, error) {
	rows, err := s.db.Query(`
		SELECT id, name, display_name, COALESCE(description, ''), provider, model, COALESCE(avatar_url, ''), created_at
		FROM bots ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []models.Bot
	for rows.Next() {
		var b models.Bot
		err := rows.Scan(&b.ID, &b.Name, &b.DisplayName, &b.Description, &b.Provider, &b.Model, &b.AvatarURL, &b.CreatedAt)
		if err != nil {
			return nil, err
		}
		bots = append(bots, b)
	}
	return bots, nil
}

func (s *Store) GetOrCreateBotDMChannel(userID, botID string) (*models.Channel, error) {
	// Check if bot DM channel already exists for this user
	var channelID string
	err := s.db.QueryRow(`
		SELECT channel_id FROM bot_channels
		WHERE bot_id = ? AND user_id = ?
	`, botID, userID).Scan(&channelID)

	if err == nil {
		return s.GetChannel(channelID)
	}

	// Get bot info for channel name
	bot, err := s.GetBot(botID)
	if err != nil {
		return nil, err
	}

	// Create new DM channel
	channelName := "bot-dm-" + botID[:8] + "-" + userID[:8]
	channel, err := s.CreateChannel(channelName, "", userID, true)
	if err != nil {
		return nil, err
	}

	// Track this as a bot channel
	_, err = s.db.Exec(`
		INSERT INTO bot_channels (channel_id, bot_id, user_id, created_at)
		VALUES (?, ?, ?, ?)
	`, channel.ID, botID, userID, time.Now())
	if err != nil {
		return nil, err
	}

	// Set display name to bot's name
	channel.Name = bot.DisplayName

	return channel, nil
}

func (s *Store) GetBotForChannel(channelID string) (*models.Bot, error) {
	var botID string
	err := s.db.QueryRow(`
		SELECT bot_id FROM bot_channels WHERE channel_id = ?
	`, channelID).Scan(&botID)

	if err != nil {
		return nil, err
	}

	return s.GetBot(botID)
}

func (s *Store) IsBotChannel(channelID string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM bot_channels WHERE channel_id = ?", channelID).Scan(&count)
	return count > 0
}

// Reaction operations

func (s *Store) AddReaction(messageID, userID, emoji string) (*models.Reaction, error) {
	reaction := &models.Reaction{
		ID:        uuid.New().String(),
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO reactions (id, message_id, user_id, emoji, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, reaction.ID, reaction.MessageID, reaction.UserID, reaction.Emoji, reaction.CreatedAt)

	if err != nil {
		return nil, err
	}
	return reaction, nil
}

func (s *Store) RemoveReaction(messageID, userID, emoji string) error {
	_, err := s.db.Exec(`
		DELETE FROM reactions WHERE message_id = ? AND user_id = ? AND emoji = ?
	`, messageID, userID, emoji)
	return err
}

func (s *Store) GetReactionsForMessage(messageID string) ([]models.ReactionGroup, error) {
	rows, err := s.db.Query(`
		SELECT r.emoji, u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM reactions r
		JOIN users u ON r.user_id = u.id
		WHERE r.message_id = ?
		ORDER BY r.emoji, r.created_at
	`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Group reactions by emoji
	groupMap := make(map[string]*models.ReactionGroup)
	var order []string

	for rows.Next() {
		var emoji string
		var user models.User
		err := rows.Scan(&emoji, &user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Status, &user.CreatedAt)
		if err != nil {
			return nil, err
		}

		if _, exists := groupMap[emoji]; !exists {
			groupMap[emoji] = &models.ReactionGroup{
				Emoji: emoji,
				Users: []models.UserResponse{},
			}
			order = append(order, emoji)
		}
		groupMap[emoji].Users = append(groupMap[emoji].Users, user.ToResponse())
		groupMap[emoji].Count++
	}

	// Convert to slice preserving order
	var groups []models.ReactionGroup
	for _, emoji := range order {
		groups = append(groups, *groupMap[emoji])
	}

	return groups, nil
}

func (s *Store) GetReactionsForMessages(messageIDs []string) (map[string][]models.ReactionGroup, error) {
	if len(messageIDs) == 0 {
		return make(map[string][]models.ReactionGroup), nil
	}

	// Build placeholder string
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT r.message_id, r.emoji, u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM reactions r
		JOIN users u ON r.user_id = u.id
		WHERE r.message_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY r.message_id, r.emoji, r.created_at
	`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Group reactions by message and emoji
	result := make(map[string]map[string]*models.ReactionGroup)

	for rows.Next() {
		var messageID, emoji string
		var user models.User
		err := rows.Scan(&messageID, &emoji, &user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Status, &user.CreatedAt)
		if err != nil {
			return nil, err
		}

		if result[messageID] == nil {
			result[messageID] = make(map[string]*models.ReactionGroup)
		}
		if result[messageID][emoji] == nil {
			result[messageID][emoji] = &models.ReactionGroup{
				Emoji: emoji,
				Users: []models.UserResponse{},
			}
		}
		result[messageID][emoji].Users = append(result[messageID][emoji].Users, user.ToResponse())
		result[messageID][emoji].Count++
	}

	// Convert to final format
	finalResult := make(map[string][]models.ReactionGroup)
	for msgID, emojiMap := range result {
		for _, group := range emojiMap {
			finalResult[msgID] = append(finalResult[msgID], *group)
		}
	}

	return finalResult, nil
}

// Webhook operations

func (s *Store) CreateWebhook(name, channelID, createdBy string) (*models.Webhook, error) {
	webhook := &models.Webhook{
		ID:        uuid.New().String(),
		Name:      name,
		ChannelID: channelID,
		Token:     uuid.New().String(),
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO webhooks (id, name, channel_id, token, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, webhook.ID, webhook.Name, webhook.ChannelID, webhook.Token, webhook.CreatedBy, webhook.CreatedAt)

	if err != nil {
		return nil, err
	}
	return webhook, nil
}

func (s *Store) GetWebhook(id string) (*models.Webhook, error) {
	webhook := &models.Webhook{}
	err := s.db.QueryRow(`
		SELECT id, name, channel_id, token, created_by, created_at
		FROM webhooks WHERE id = ?
	`, id).Scan(&webhook.ID, &webhook.Name, &webhook.ChannelID, &webhook.Token, &webhook.CreatedBy, &webhook.CreatedAt)

	if err != nil {
		return nil, err
	}
	return webhook, nil
}

func (s *Store) GetWebhookByToken(id, token string) (*models.Webhook, error) {
	webhook := &models.Webhook{}
	err := s.db.QueryRow(`
		SELECT id, name, channel_id, token, created_by, created_at
		FROM webhooks WHERE id = ? AND token = ?
	`, id, token).Scan(&webhook.ID, &webhook.Name, &webhook.ChannelID, &webhook.Token, &webhook.CreatedBy, &webhook.CreatedAt)

	if err != nil {
		return nil, err
	}
	return webhook, nil
}

func (s *Store) GetWebhooksForChannel(channelID string) ([]models.Webhook, error) {
	rows, err := s.db.Query(`
		SELECT id, name, channel_id, token, created_by, created_at
		FROM webhooks WHERE channel_id = ?
		ORDER BY created_at DESC
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []models.Webhook
	for rows.Next() {
		var w models.Webhook
		err := rows.Scan(&w.ID, &w.Name, &w.ChannelID, &w.Token, &w.CreatedBy, &w.CreatedAt)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, w)
	}
	return webhooks, nil
}

func (s *Store) GetWebhooksByUser(userID string) ([]models.Webhook, error) {
	rows, err := s.db.Query(`
		SELECT id, name, channel_id, token, created_by, created_at
		FROM webhooks WHERE created_by = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []models.Webhook
	for rows.Next() {
		var w models.Webhook
		err := rows.Scan(&w.ID, &w.Name, &w.ChannelID, &w.Token, &w.CreatedBy, &w.CreatedAt)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, w)
	}
	return webhooks, nil
}

func (s *Store) DeleteWebhook(id, userID string) error {
	_, err := s.db.Exec("DELETE FROM webhooks WHERE id = ? AND created_by = ?", id, userID)
	return err
}

// Kanban Board operations

func (s *Store) CreateBoard(name, description, icon, createdBy string) (*models.Board, error) {
	board := &models.Board{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Icon:        icon,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO kanban_boards (id, name, description, icon, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, board.ID, board.Name, board.Description, board.Icon, board.CreatedBy, board.CreatedAt, board.UpdatedAt)

	if err != nil {
		return nil, err
	}

	// Add creator as owner
	s.AddBoardMember(board.ID, createdBy, "owner")

	return board, nil
}

func (s *Store) GetBoard(id string) (*models.Board, error) {
	board := &models.Board{}
	err := s.db.QueryRow(`
		SELECT id, name, COALESCE(description, ''), COALESCE(icon, ''), created_by, created_at, updated_at
		FROM kanban_boards WHERE id = ?
	`, id).Scan(&board.ID, &board.Name, &board.Description, &board.Icon, &board.CreatedBy, &board.CreatedAt, &board.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return board, nil
}

func (s *Store) GetBoardsForUser(userID string) ([]models.BoardWithDetails, error) {
	rows, err := s.db.Query(`
		SELECT b.id, b.name, COALESCE(b.description, ''), COALESCE(b.icon, ''), b.created_by, b.created_at, b.updated_at,
			(SELECT COUNT(*) FROM kanban_columns WHERE board_id = b.id) as column_count,
			(SELECT COUNT(*) FROM kanban_cards WHERE board_id = b.id) as card_count,
			(SELECT COUNT(*) FROM kanban_board_members WHERE board_id = b.id) as member_count
		FROM kanban_boards b
		JOIN kanban_board_members bm ON b.id = bm.board_id
		WHERE bm.user_id = ?
		ORDER BY b.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []models.BoardWithDetails
	for rows.Next() {
		var b models.BoardWithDetails
		err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.Icon, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
			&b.ColumnCount, &b.CardCount, &b.MemberCount)
		if err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, nil
}

func (s *Store) UpdateBoard(id, name, description string, icon *string) error {
	if icon != nil {
		_, err := s.db.Exec(`
			UPDATE kanban_boards SET name = ?, description = ?, icon = ?, updated_at = ?
			WHERE id = ?
		`, name, description, *icon, time.Now(), id)
		return err
	}
	_, err := s.db.Exec(`
		UPDATE kanban_boards SET name = ?, description = ?, updated_at = ?
		WHERE id = ?
	`, name, description, time.Now(), id)
	return err
}

func (s *Store) DeleteBoard(id string) error {
	_, err := s.db.Exec("DELETE FROM kanban_boards WHERE id = ?", id)
	return err
}

// Board member operations

func (s *Store) AddBoardMember(boardID, userID, role string) error {
	if role == "" {
		role = "member"
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO kanban_board_members (board_id, user_id, role, joined_at)
		VALUES (?, ?, ?, ?)
	`, boardID, userID, role, time.Now())
	return err
}

func (s *Store) RemoveBoardMember(boardID, userID string) error {
	_, err := s.db.Exec("DELETE FROM kanban_board_members WHERE board_id = ? AND user_id = ?", boardID, userID)
	return err
}

func (s *Store) GetBoardMembers(boardID string) ([]models.BoardMember, error) {
	rows, err := s.db.Query(`
		SELECT bm.board_id, bm.user_id, bm.role, bm.joined_at,
			u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM kanban_board_members bm
		JOIN users u ON bm.user_id = u.id
		WHERE bm.board_id = ?
		ORDER BY bm.joined_at
	`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.BoardMember
	for rows.Next() {
		var m models.BoardMember
		var u models.User
		err := rows.Scan(&m.BoardID, &m.UserID, &m.Role, &m.JoinedAt,
			&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Status, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		m.User = u.ToResponse()
		members = append(members, m)
	}
	return members, nil
}

func (s *Store) IsBoardMember(boardID, userID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM kanban_board_members WHERE board_id = ? AND user_id = ?", boardID, userID).Scan(&count)
	return count > 0, err
}

func (s *Store) GetBoardMemberRole(boardID, userID string) (string, error) {
	var role string
	err := s.db.QueryRow("SELECT role FROM kanban_board_members WHERE board_id = ? AND user_id = ?", boardID, userID).Scan(&role)
	return role, err
}

// Kanban Column operations

func (s *Store) CreateColumn(boardID, name string, position int) (*models.KanbanColumn, error) {
	// If position not specified, add to end
	if position < 0 {
		var maxPos int
		s.db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM kanban_columns WHERE board_id = ?", boardID).Scan(&maxPos)
		position = maxPos + 1
	}

	column := &models.KanbanColumn{
		ID:        uuid.New().String(),
		BoardID:   boardID,
		Name:      name,
		Position:  position,
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO kanban_columns (id, board_id, name, position, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, column.ID, column.BoardID, column.Name, column.Position, column.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Update board timestamp
	s.db.Exec("UPDATE kanban_boards SET updated_at = ? WHERE id = ?", time.Now(), boardID)

	return column, nil
}

func (s *Store) GetColumn(id string) (*models.KanbanColumn, error) {
	column := &models.KanbanColumn{}
	err := s.db.QueryRow(`
		SELECT id, board_id, name, position, created_at
		FROM kanban_columns WHERE id = ?
	`, id).Scan(&column.ID, &column.BoardID, &column.Name, &column.Position, &column.CreatedAt)

	if err != nil {
		return nil, err
	}
	return column, nil
}

func (s *Store) GetColumnsForBoard(boardID string) ([]models.KanbanColumn, error) {
	rows, err := s.db.Query(`
		SELECT id, board_id, name, position, created_at
		FROM kanban_columns WHERE board_id = ?
		ORDER BY position
	`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []models.KanbanColumn
	for rows.Next() {
		var c models.KanbanColumn
		err := rows.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		columns = append(columns, c)
	}
	return columns, nil
}

func (s *Store) GetColumnsWithCards(boardID string) ([]models.KanbanColumnWithCards, error) {
	columns, err := s.GetColumnsForBoard(boardID)
	if err != nil {
		return nil, err
	}

	result := make([]models.KanbanColumnWithCards, len(columns))
	for i, col := range columns {
		result[i] = models.KanbanColumnWithCards{
			KanbanColumn: col,
			Cards:        []models.CardWithDetails{},
		}
		cards, err := s.GetCardsForColumn(col.ID)
		if err != nil {
			return nil, err
		}
		result[i].Cards = cards
	}
	return result, nil
}

func (s *Store) UpdateColumn(id, name string) error {
	_, err := s.db.Exec("UPDATE kanban_columns SET name = ? WHERE id = ?", name, id)
	return err
}

func (s *Store) UpdateColumnPosition(id string, position int) error {
	_, err := s.db.Exec("UPDATE kanban_columns SET position = ? WHERE id = ?", position, id)
	return err
}

func (s *Store) DeleteColumn(id string) error {
	_, err := s.db.Exec("DELETE FROM kanban_columns WHERE id = ?", id)
	return err
}

func (s *Store) ReorderColumns(boardID string, columnIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, colID := range columnIDs {
		_, err := tx.Exec("UPDATE kanban_columns SET position = ? WHERE id = ? AND board_id = ?", i, colID, boardID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Kanban Card operations

func (s *Store) CreateCard(columnID, boardID, title, description, createdBy string, dueDate *time.Time) (*models.KanbanCard, error) {
	// Get next position
	var maxPos int
	s.db.QueryRow("SELECT COALESCE(MAX(position), -1) FROM kanban_cards WHERE column_id = ?", columnID).Scan(&maxPos)

	card := &models.KanbanCard{
		ID:          uuid.New().String(),
		ColumnID:    columnID,
		BoardID:     boardID,
		Title:       title,
		Description: description,
		Position:    maxPos + 1,
		DueDate:     dueDate,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO kanban_cards (id, column_id, board_id, title, description, position, due_date, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, card.ID, card.ColumnID, card.BoardID, card.Title, card.Description, card.Position, card.DueDate, card.CreatedBy, card.CreatedAt, card.UpdatedAt)

	if err != nil {
		return nil, err
	}

	// Update board timestamp
	s.db.Exec("UPDATE kanban_boards SET updated_at = ? WHERE id = ?", time.Now(), boardID)

	return card, nil
}

func (s *Store) GetCard(id string) (*models.KanbanCard, error) {
	card := &models.KanbanCard{}
	var dueDate sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, column_id, board_id, title, COALESCE(description, ''), position, due_date, created_by, created_at, updated_at
		FROM kanban_cards WHERE id = ?
	`, id).Scan(&card.ID, &card.ColumnID, &card.BoardID, &card.Title, &card.Description, &card.Position, &dueDate, &card.CreatedBy, &card.CreatedAt, &card.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if dueDate.Valid {
		card.DueDate = &dueDate.Time
	}

	return card, nil
}

func (s *Store) GetCardWithDetails(id string) (*models.CardWithDetails, error) {
	card, err := s.GetCard(id)
	if err != nil {
		return nil, err
	}

	result := &models.CardWithDetails{
		KanbanCard: *card,
		Assignees:  []models.UserResponse{},
		Labels:     []models.KanbanLabel{},
	}

	// Get assignees
	assignees, err := s.GetCardAssignees(id)
	if err == nil {
		result.Assignees = assignees
	}

	// Get labels
	labels, err := s.GetCardLabels(id)
	if err == nil {
		result.Labels = labels
	}

	// Get comment count
	s.db.QueryRow("SELECT COUNT(*) FROM kanban_comments WHERE card_id = ?", id).Scan(&result.CommentCount)

	// Get creator
	creator, err := s.GetUserByID(card.CreatedBy)
	if err == nil {
		result.Creator = creator.ToResponse()
	}

	return result, nil
}

func (s *Store) GetCardsForColumn(columnID string) ([]models.CardWithDetails, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.column_id, c.board_id, c.title, COALESCE(c.description, ''), c.position, c.due_date, c.created_by, c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM kanban_comments WHERE card_id = c.id) as comment_count
		FROM kanban_cards c
		WHERE c.column_id = ?
		ORDER BY c.position
	`, columnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []models.CardWithDetails
	for rows.Next() {
		var c models.CardWithDetails
		var dueDate sql.NullTime
		err := rows.Scan(&c.ID, &c.ColumnID, &c.BoardID, &c.Title, &c.Description, &c.Position, &dueDate, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.CommentCount)
		if err != nil {
			return nil, err
		}
		if dueDate.Valid {
			c.DueDate = &dueDate.Time
		}

		// Get assignees and labels
		c.Assignees, _ = s.GetCardAssignees(c.ID)
		if c.Assignees == nil {
			c.Assignees = []models.UserResponse{}
		}
		c.Labels, _ = s.GetCardLabels(c.ID)
		if c.Labels == nil {
			c.Labels = []models.KanbanLabel{}
		}

		cards = append(cards, c)
	}
	return cards, nil
}

func (s *Store) UpdateCard(id, title, description string, dueDate *time.Time) error {
	_, err := s.db.Exec(`
		UPDATE kanban_cards SET title = ?, description = ?, due_date = ?, updated_at = ?
		WHERE id = ?
	`, title, description, dueDate, time.Now(), id)
	return err
}

func (s *Store) DeleteCard(id string) error {
	_, err := s.db.Exec("DELETE FROM kanban_cards WHERE id = ?", id)
	return err
}

func (s *Store) MoveCard(cardID, targetColumnID string, position int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update the card's column and position
	_, err = tx.Exec(`
		UPDATE kanban_cards SET column_id = ?, position = ?, updated_at = ?
		WHERE id = ?
	`, targetColumnID, position, time.Now(), cardID)
	if err != nil {
		return err
	}

	// Reorder other cards in the target column
	_, err = tx.Exec(`
		UPDATE kanban_cards SET position = position + 1
		WHERE column_id = ? AND id != ? AND position >= ?
	`, targetColumnID, cardID, position)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Card assignee operations

func (s *Store) SetCardAssignees(cardID string, userIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove existing assignees
	_, err = tx.Exec("DELETE FROM kanban_card_assignees WHERE card_id = ?", cardID)
	if err != nil {
		return err
	}

	// Add new assignees
	for _, userID := range userIDs {
		_, err = tx.Exec(`
			INSERT INTO kanban_card_assignees (card_id, user_id, assigned_at)
			VALUES (?, ?, ?)
		`, cardID, userID, time.Now())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetCardAssignees(cardID string) ([]models.UserResponse, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM users u
		JOIN kanban_card_assignees ca ON u.id = ca.user_id
		WHERE ca.card_id = ?
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignees []models.UserResponse
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Status, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		assignees = append(assignees, u.ToResponse())
	}
	return assignees, nil
}

// Card label operations

func (s *Store) SetCardLabels(cardID string, labelIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove existing labels
	_, err = tx.Exec("DELETE FROM kanban_card_labels WHERE card_id = ?", cardID)
	if err != nil {
		return err
	}

	// Add new labels
	for _, labelID := range labelIDs {
		_, err = tx.Exec(`
			INSERT INTO kanban_card_labels (card_id, label_id)
			VALUES (?, ?)
		`, cardID, labelID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetCardLabels(cardID string) ([]models.KanbanLabel, error) {
	rows, err := s.db.Query(`
		SELECT l.id, l.board_id, l.name, l.color, l.created_at
		FROM kanban_labels l
		JOIN kanban_card_labels cl ON l.id = cl.label_id
		WHERE cl.card_id = ?
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []models.KanbanLabel
	for rows.Next() {
		var l models.KanbanLabel
		err := rows.Scan(&l.ID, &l.BoardID, &l.Name, &l.Color, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, nil
}

// Kanban Label operations

func (s *Store) CreateLabel(boardID, name, color string) (*models.KanbanLabel, error) {
	label := &models.KanbanLabel{
		ID:        uuid.New().String(),
		BoardID:   boardID,
		Name:      name,
		Color:     color,
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO kanban_labels (id, board_id, name, color, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, label.ID, label.BoardID, label.Name, label.Color, label.CreatedAt)

	if err != nil {
		return nil, err
	}
	return label, nil
}

func (s *Store) GetLabel(id string) (*models.KanbanLabel, error) {
	label := &models.KanbanLabel{}
	err := s.db.QueryRow(`
		SELECT id, board_id, name, color, created_at
		FROM kanban_labels WHERE id = ?
	`, id).Scan(&label.ID, &label.BoardID, &label.Name, &label.Color, &label.CreatedAt)

	if err != nil {
		return nil, err
	}
	return label, nil
}

func (s *Store) GetLabelsForBoard(boardID string) ([]models.KanbanLabel, error) {
	rows, err := s.db.Query(`
		SELECT id, board_id, name, color, created_at
		FROM kanban_labels WHERE board_id = ?
		ORDER BY name
	`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []models.KanbanLabel
	for rows.Next() {
		var l models.KanbanLabel
		err := rows.Scan(&l.ID, &l.BoardID, &l.Name, &l.Color, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, nil
}

func (s *Store) UpdateLabel(id, name, color string) error {
	_, err := s.db.Exec("UPDATE kanban_labels SET name = ?, color = ? WHERE id = ?", name, color, id)
	return err
}

func (s *Store) DeleteLabel(id string) error {
	_, err := s.db.Exec("DELETE FROM kanban_labels WHERE id = ?", id)
	return err
}

// Kanban Comment operations

func (s *Store) CreateKanbanComment(cardID, userID, content string) (*models.KanbanComment, error) {
	comment := &models.KanbanComment{
		ID:        uuid.New().String(),
		CardID:    cardID,
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO kanban_comments (id, card_id, user_id, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, comment.ID, comment.CardID, comment.UserID, comment.Content, comment.CreatedAt, comment.UpdatedAt)

	if err != nil {
		return nil, err
	}

	// Get user for response
	user, err := s.GetUserByID(userID)
	if err == nil {
		comment.User = user.ToResponse()
	}

	return comment, nil
}

func (s *Store) GetKanbanComment(id string) (*models.KanbanComment, error) {
	comment := &models.KanbanComment{}
	err := s.db.QueryRow(`
		SELECT id, card_id, user_id, content, created_at, updated_at
		FROM kanban_comments WHERE id = ?
	`, id).Scan(&comment.ID, &comment.CardID, &comment.UserID, &comment.Content, &comment.CreatedAt, &comment.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *Store) GetCommentsForCard(cardID string) ([]models.KanbanComment, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.card_id, c.user_id, c.content, c.created_at, c.updated_at,
			u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM kanban_comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.card_id = ?
		ORDER BY c.created_at
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.KanbanComment
	for rows.Next() {
		var c models.KanbanComment
		var u models.User
		err := rows.Scan(&c.ID, &c.CardID, &c.UserID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
			&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Status, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		c.User = u.ToResponse()
		comments = append(comments, c)
	}
	return comments, nil
}

func (s *Store) UpdateKanbanComment(id, content string) error {
	_, err := s.db.Exec("UPDATE kanban_comments SET content = ?, updated_at = ? WHERE id = ?", content, time.Now(), id)
	return err
}

func (s *Store) DeleteKanbanComment(id string) error {
	_, err := s.db.Exec("DELETE FROM kanban_comments WHERE id = ?", id)
	return err
}

// ==================== App Operations ====================

func (s *Store) CreateApp(name, description, icon, createdBy string) (*models.App, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO apps (id, name, description, icon, html_content, css_content, js_content, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, '', '', '', ?, ?, ?)
	`, id, name, description, icon, createdBy, now, now)
	if err != nil {
		return nil, err
	}

	// Add creator as owner
	_, err = s.db.Exec(`
		INSERT INTO app_members (app_id, user_id, role, joined_at)
		VALUES (?, ?, 'owner', ?)
	`, id, createdBy, now)
	if err != nil {
		return nil, err
	}

	return s.GetApp(id)
}

func (s *Store) GetApp(id string) (*models.App, error) {
	app := &models.App{}
	err := s.db.QueryRow(`
		SELECT id, name, COALESCE(description, ''), COALESCE(icon, ''), html_content, css_content, js_content, created_by, created_at, updated_at
		FROM apps WHERE id = ?
	`, id).Scan(&app.ID, &app.Name, &app.Description, &app.Icon, &app.HTMLContent, &app.CSSContent, &app.JSContent, &app.CreatedBy, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return app, nil
}

func (s *Store) GetAppsForUser(userID string) ([]models.AppWithDetails, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.name, COALESCE(a.description, ''), COALESCE(a.icon, ''), a.html_content, a.css_content, a.js_content, a.created_by, a.created_at, a.updated_at,
			(SELECT COUNT(*) FROM app_members WHERE app_id = a.id) as member_count
		FROM apps a
		JOIN app_members am ON a.id = am.app_id
		WHERE am.user_id = ?
		ORDER BY a.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []models.AppWithDetails
	for rows.Next() {
		var app models.AppWithDetails
		err := rows.Scan(&app.ID, &app.Name, &app.Description, &app.Icon, &app.HTMLContent, &app.CSSContent, &app.JSContent,
			&app.CreatedBy, &app.CreatedAt, &app.UpdatedAt, &app.MemberCount)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func (s *Store) UpdateApp(id, name, description string, icon *string) error {
	if icon != nil {
		_, err := s.db.Exec(`
			UPDATE apps SET name = ?, description = ?, icon = ?, updated_at = ? WHERE id = ?
		`, name, description, *icon, time.Now(), id)
		return err
	}
	_, err := s.db.Exec(`
		UPDATE apps SET name = ?, description = ?, updated_at = ? WHERE id = ?
	`, name, description, time.Now(), id)
	return err
}

func (s *Store) UpdateAppCode(id, htmlContent, cssContent, jsContent string) error {
	_, err := s.db.Exec(`
		UPDATE apps SET html_content = ?, css_content = ?, js_content = ?, updated_at = ? WHERE id = ?
	`, htmlContent, cssContent, jsContent, time.Now(), id)
	return err
}

func (s *Store) DeleteApp(id string) error {
	_, err := s.db.Exec("DELETE FROM apps WHERE id = ?", id)
	return err
}

// App member operations

func (s *Store) AddAppMember(appID, userID, role string) error {
	if role == "" {
		role = "member"
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO app_members (app_id, user_id, role, joined_at)
		VALUES (?, ?, ?, ?)
	`, appID, userID, role, time.Now())
	return err
}

func (s *Store) RemoveAppMember(appID, userID string) error {
	_, err := s.db.Exec("DELETE FROM app_members WHERE app_id = ? AND user_id = ?", appID, userID)
	return err
}

func (s *Store) GetAppMembers(appID string) ([]models.AppMember, error) {
	rows, err := s.db.Query(`
		SELECT am.app_id, am.user_id, am.role, am.joined_at,
			u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''), u.status, u.created_at
		FROM app_members am
		JOIN users u ON am.user_id = u.id
		WHERE am.app_id = ?
		ORDER BY am.joined_at
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.AppMember
	for rows.Next() {
		var m models.AppMember
		var u models.User
		err := rows.Scan(&m.AppID, &m.UserID, &m.Role, &m.JoinedAt,
			&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Status, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		m.User = u.ToResponse()
		members = append(members, m)
	}
	return members, nil
}

func (s *Store) IsAppMember(appID, userID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM app_members WHERE app_id = ? AND user_id = ?", appID, userID).Scan(&count)
	return count > 0, err
}

func (s *Store) GetAppMemberRole(appID, userID string) (string, error) {
	var role string
	err := s.db.QueryRow("SELECT role FROM app_members WHERE app_id = ? AND user_id = ?", appID, userID).Scan(&role)
	return role, err
}

// App message operations (for AI conversation history)

func (s *Store) CreateAppMessage(appID, userID, role, content string) (*models.AppMessage, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO app_messages (id, app_id, user_id, role, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, appID, userID, role, content, now)
	if err != nil {
		return nil, err
	}

	return &models.AppMessage{
		ID:        id,
		AppID:     appID,
		UserID:    userID,
		Role:      role,
		Content:   content,
		CreatedAt: now,
	}, nil
}

func (s *Store) GetAppMessages(appID string, limit int) ([]models.AppMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, app_id, user_id, role, content, created_at
		FROM app_messages
		WHERE app_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.AppMessage
	for rows.Next() {
		var m models.AppMessage
		err := rows.Scan(&m.ID, &m.AppID, &m.UserID, &m.Role, &m.Content, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// Custom command operations

func (s *Store) CreateCommand(name, description, url, method, headers, bodyTemplate, responseMode, createdBy string, isGlobal bool) (*models.CustomCommand, error) {
	id := uuid.New().String()
	now := time.Now()

	// Default method to GET if not specified
	if method == "" {
		method = "GET"
	}

	// Default response mode to private
	if responseMode == "" {
		responseMode = "private"
	}

	_, err := s.db.Exec(`
		INSERT INTO custom_commands (id, name, description, url, method, headers, body_template, is_global, created_by, response_mode, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?)
	`, id, name, description, url, method, headers, bodyTemplate, isGlobal, createdBy, responseMode, now, now)
	if err != nil {
		return nil, err
	}

	return &models.CustomCommand{
		ID:           id,
		Name:         name,
		Description:  description,
		URL:          url,
		Method:       method,
		Headers:      headers,
		BodyTemplate: bodyTemplate,
		IsGlobal:     isGlobal,
		CreatedBy:    createdBy,
		ResponseMode: responseMode,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *Store) GetCommand(id string) (*models.CustomCommand, error) {
	cmd := &models.CustomCommand{}
	var description, headers, bodyTemplate sql.NullString

	err := s.db.QueryRow(`
		SELECT id, name, description, url, method, headers, body_template, is_global, created_by, response_mode, enabled, created_at, updated_at
		FROM custom_commands WHERE id = ?
	`, id).Scan(&cmd.ID, &cmd.Name, &description, &cmd.URL, &cmd.Method, &headers, &bodyTemplate, &cmd.IsGlobal, &cmd.CreatedBy, &cmd.ResponseMode, &cmd.Enabled, &cmd.CreatedAt, &cmd.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if description.Valid {
		cmd.Description = description.String
	}
	if headers.Valid {
		cmd.Headers = headers.String
	}
	if bodyTemplate.Valid {
		cmd.BodyTemplate = bodyTemplate.String
	}

	return cmd, nil
}

func (s *Store) GetCommandByName(name, userID string) (*models.CustomCommand, error) {
	cmd := &models.CustomCommand{}
	var description, headers, bodyTemplate sql.NullString

	// First try user's private command, then global
	err := s.db.QueryRow(`
		SELECT id, name, description, url, method, headers, body_template, is_global, created_by, response_mode, enabled, created_at, updated_at
		FROM custom_commands
		WHERE name = ? AND ((is_global = FALSE AND created_by = ?) OR is_global = TRUE) AND enabled = TRUE
		ORDER BY is_global ASC
		LIMIT 1
	`, name, userID).Scan(&cmd.ID, &cmd.Name, &description, &cmd.URL, &cmd.Method, &headers, &bodyTemplate, &cmd.IsGlobal, &cmd.CreatedBy, &cmd.ResponseMode, &cmd.Enabled, &cmd.CreatedAt, &cmd.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if description.Valid {
		cmd.Description = description.String
	}
	if headers.Valid {
		cmd.Headers = headers.String
	}
	if bodyTemplate.Valid {
		cmd.BodyTemplate = bodyTemplate.String
	}

	return cmd, nil
}

func (s *Store) GetCommandsForUser(userID string) ([]models.CustomCommand, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, url, method, headers, body_template, is_global, created_by, response_mode, enabled, created_at, updated_at
		FROM custom_commands
		WHERE (is_global = TRUE OR created_by = ?) AND enabled = TRUE
		ORDER BY name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []models.CustomCommand
	for rows.Next() {
		var cmd models.CustomCommand
		var description, headers, bodyTemplate sql.NullString
		err := rows.Scan(&cmd.ID, &cmd.Name, &description, &cmd.URL, &cmd.Method, &headers, &bodyTemplate, &cmd.IsGlobal, &cmd.CreatedBy, &cmd.ResponseMode, &cmd.Enabled, &cmd.CreatedAt, &cmd.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if description.Valid {
			cmd.Description = description.String
		}
		if headers.Valid {
			cmd.Headers = headers.String
		}
		if bodyTemplate.Valid {
			cmd.BodyTemplate = bodyTemplate.String
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

func (s *Store) UpdateCommand(id string, req *models.UpdateCommandRequest) error {
	// Build dynamic update query
	var updates []string
	var args []interface{}

	if req.Name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *req.Description)
	}
	if req.URL != nil {
		updates = append(updates, "url = ?")
		args = append(args, *req.URL)
	}
	if req.Method != nil {
		updates = append(updates, "method = ?")
		args = append(args, *req.Method)
	}
	if req.Headers != nil {
		updates = append(updates, "headers = ?")
		args = append(args, *req.Headers)
	}
	if req.BodyTemplate != nil {
		updates = append(updates, "body_template = ?")
		args = append(args, *req.BodyTemplate)
	}
	if req.IsGlobal != nil {
		updates = append(updates, "is_global = ?")
		args = append(args, *req.IsGlobal)
	}
	if req.ResponseMode != nil {
		updates = append(updates, "response_mode = ?")
		args = append(args, *req.ResponseMode)
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = ?")
		args = append(args, *req.Enabled)
	}

	if len(updates) == 0 {
		return nil
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)

	query := "UPDATE custom_commands SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	_, err := s.db.Exec(query, args...)
	return err
}

func (s *Store) DeleteCommand(id, userID string) error {
	_, err := s.db.Exec("DELETE FROM custom_commands WHERE id = ? AND created_by = ?", id, userID)
	return err
}

// User Preferences

func (s *Store) GetUserPreference(userID, key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM user_preferences WHERE user_id = ? AND key = ?", userID, key).Scan(&value)
	return value, err
}

func (s *Store) GetAllUserPreferences(userID string) ([]models.UserPreference, error) {
	rows, err := s.db.Query("SELECT key, value FROM user_preferences WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []models.UserPreference
	for rows.Next() {
		var p models.UserPreference
		if err := rows.Scan(&p.Key, &p.Value); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, nil
}

func (s *Store) SetUserPreference(userID, key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_preferences (user_id, key, value, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, userID, key, value, time.Now())
	return err
}

func (s *Store) DeleteUserPreference(userID, key string) error {
	_, err := s.db.Exec("DELETE FROM user_preferences WHERE user_id = ? AND key = ?", userID, key)
	return err
}

// Server settings

func (s *Store) GetServerSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM server_settings WHERE key = ?", key).Scan(&value)
	return value, err
}

func (s *Store) SetServerSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO server_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}
