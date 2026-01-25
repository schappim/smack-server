# Smack

A powerful real-time messaging platform with webhooks, custom apps, kanban boards, and AI-powered bots.

## Features

| Feature | Description |
|---------|-------------|
| **50+ API Endpoints** | Comprehensive REST API for all operations |
| **12 WebSocket Events** | Real-time messaging, typing, presence, AI streaming |
| **7 Widget Types** | Rich HTML widgets for webhooks |
| **JWT Authentication** | Secure token-based auth with 7-day expiry |

### Core Features

- **Channels & DMs** - Public channels and direct messages
- **Threaded Conversations** - Reply to messages in threads
- **Reactions** - Emoji reactions on messages
- **File Uploads** - Share images and files
- **Reminders** - Set time-based reminders

### Advanced Features

- **AI Bots** - OpenAI GPT integration with streaming responses
- **Webhooks** - Incoming webhooks with rich HTML widget support
- **Custom Commands** - Slash commands with HTTP calls and AI builder
- **Kanban Boards** - Project management with boards, columns, cards, labels
- **Apps** - Build custom web apps with AI chat and private SQLite databases

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap schappim/smack-server
brew install smack-server
```

### Direct Download

```bash
curl -fsSL https://raw.githubusercontent.com/schappim/smack-server/main/install.sh | sh
```

### From Source

```bash
git clone https://github.com/schappim/smack-server.git
cd smack-server
go build -o smack-server .
```

## Quick Start

```bash
# Set OpenAI API key (optional, for AI bot features)
export OPENAI_KEY="your-openai-api-key"

# Start the server
smack-server
```

Server starts at `http://localhost:8080`

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DB_PATH` | SQLite database path | `./smack.db` |
| `UPLOAD_DIR` | File upload directory | `./uploads` |
| `OPENAI_KEY` | OpenAI API key | - |

## Authentication

The API uses JWT tokens. Register or login to get a token:

```bash
# Register
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "johndoe", "display_name": "John Doe", "password": "secret123"}'

# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "johndoe", "password": "secret123"}'
```

Use the token in requests:

```bash
curl http://localhost:8080/api/channels \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Token Details:**
- Algorithm: HS256
- Expiration: 7 days
- Format: `Authorization: Bearer <token>`

## WebSocket

Connect for real-time events:

```
ws://localhost:8080/api/ws?token=YOUR_JWT_TOKEN
```

### Events

| Event | Direction | Description |
|-------|-----------|-------------|
| `new_message` | Server → Client | New message posted |
| `message_deleted` | Server → Client | Message deleted |
| `user_online` | Server → Client | User came online |
| `user_offline` | Server → Client | User went offline |
| `typing` | Both | User typing indicator |
| `reaction_update` | Server → Client | Reaction added/removed |
| `reminder` | Server → Client | Reminder triggered |
| `message_stream_start` | Server → Client | AI streaming started |
| `message_stream_delta` | Server → Client | AI streaming chunk |
| `message_stream_end` | Server → Client | AI streaming complete |
| `subscribe` | Client → Server | Subscribe to channel |

### Example

```javascript
const ws = new WebSocket(`ws://localhost:8080/api/ws?token=${token}`);

ws.onopen = () => {
  // Subscribe to a channel
  ws.send(JSON.stringify({
    type: 'subscribe',
    channel_id: 'channel-uuid'
  }));
};

ws.onmessage = (event) => {
  const { type, payload } = JSON.parse(event.data);
  console.log('Received:', type, payload);
};
```

## Webhooks

Post messages from external services with optional rich HTML widgets.

### Create a Webhook

```bash
curl -X POST http://localhost:8080/api/webhooks \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "GitHub Notifications", "channel_id": "CHANNEL_UUID"}'
```

### Post via Webhook

No auth required - the token in the URL acts as authentication:

```bash
curl -X POST http://localhost:8080/api/webhooks/incoming/WEBHOOK_ID/TOKEN \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Build completed!",
    "username": "CI Bot",
    "widget_size": "medium",
    "html": "<div style=\"background: #10b981; color: white; padding: 16px; border-radius: 12px;\">Deploy successful</div>"
  }'
```

### Widget Sizes

| Size | Height |
|------|--------|
| `small` | 100px |
| `medium` | 150px |
| `large` | 200px |
| `xlarge` | 300px |

## API Endpoints

### Auth
- `POST /api/auth/register` - Create account
- `POST /api/auth/login` - Login
- `GET /api/auth/me` - Get current user

### Users
- `GET /api/users` - List users
- `GET /api/users/{id}` - Get user
- `PUT /api/users/me` - Update profile
- `PUT /api/users/me/status` - Update status (online/offline/away/dnd)

### Channels
- `GET /api/channels` - List your channels
- `GET /api/channels/public` - List public channels
- `POST /api/channels` - Create channel
- `GET /api/channels/{id}` - Get channel
- `POST /api/channels/{id}/join` - Join channel
- `POST /api/channels/{id}/leave` - Leave channel
- `GET /api/channels/{id}/members` - Get members
- `POST /api/channels/{id}/read` - Mark as read
- `POST /api/channels/{id}/mute` - Mute channel
- `POST /api/channels/{id}/unmute` - Unmute channel
- `POST /api/dm` - Create direct message

### Messages
- `GET /api/channels/{id}/messages` - Get messages (supports `?limit=` and `?before=`)
- `POST /api/messages` - Send message
- `DELETE /api/messages/{id}` - Delete message
- `GET /api/messages/{id}/thread` - Get thread replies
- `POST /api/messages/{id}/reply` - Reply to thread

### Reactions
- `POST /api/reactions` - Add reaction
- `DELETE /api/reactions` - Remove reaction
- `GET /api/messages/{id}/reactions` - Get reactions

### Bots
- `GET /api/bots` - List AI bots
- `GET /api/bots/{id}` - Get bot
- `POST /api/bots/dm` - Create bot DM channel

### Files
- `POST /api/files/upload` - Upload file (multipart/form-data)
- `GET /api/files/{filename}` - Get file

### Webhooks
- `POST /api/webhooks` - Create webhook
- `GET /api/webhooks` - List webhooks
- `GET /api/webhooks/{id}` - Get webhook
- `DELETE /api/webhooks/{id}` - Delete webhook
- `POST /api/webhooks/incoming/{id}/{token}` - Post via webhook (no auth)

### Reminders
- `GET /api/reminders` - List reminders
- `POST /api/reminders` - Create reminder
- `DELETE /api/reminders/{id}` - Delete reminder

### Commands
- `GET /api/commands` - List commands
- `POST /api/commands` - Create command
- `PUT /api/commands/{id}` - Update command
- `DELETE /api/commands/{id}` - Delete command
- `POST /api/commands/{id}/execute` - Execute command

### Kanban
- `GET /api/kanban/boards` - List boards
- `POST /api/kanban/boards` - Create board
- Full CRUD for boards, columns, cards, labels, comments

### Apps
- `GET /api/apps` - List apps
- `POST /api/apps` - Create app
- Full CRUD with AI chat, live preview, and private databases

## Documentation

Full HTML documentation available in the `docs/` folder:

- [Index](docs/index.html) - Overview
- [Authentication](docs/authentication.html) - JWT tokens and auth flow
- [API Reference](docs/api-reference.html) - All REST endpoints
- [WebSocket](docs/websocket.html) - Real-time events and streaming
- [Webhooks](docs/webhooks.html) - Incoming webhooks with HTML widgets
- [Apps & Widgets](docs/apps-widgets.html) - Custom apps with AI and databases
- [Kanban](docs/kanban.html) - Project management boards
- [Commands](docs/commands.html) - Custom slash commands

## Project Structure

```
├── main.go              # Entry point and routes
├── ai/                  # OpenAI client
├── handlers/            # HTTP handlers
├── middleware/          # Auth middleware
├── models/              # Data models
├── store/               # SQLite database
├── commands/            # Command interpolation
├── docs/                # HTML documentation
└── API.md               # API reference (markdown)
```

## License

MIT
