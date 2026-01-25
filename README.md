# Smack

A powerful real-time messaging platform with webhooks, custom apps, kanban boards, and AI-powered bots.

## Features

- **50+ API Endpoints** - Comprehensive REST API
- **12 WebSocket Events** - Real-time messaging, typing indicators, presence, AI streaming
- **7 Widget Types** - Rich HTML widgets for webhooks
- **JWT Authentication** - Secure token-based auth with 7-day expiry

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
- **Kanban Boards** - Full project management with boards, columns, cards, labels, and comments
- **Apps** - Build custom web apps with AI chat assistance and private SQLite databases

## Quick Start

### Prerequisites

- Go 1.21+
- (Optional) OpenAI API key for AI features

### Run

```bash
git clone https://github.com/schappim/smack-server.git
cd smack-server
go run .
```

Server starts at `http://localhost:8080`

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DB_PATH` | SQLite database path | `./smack.db` |
| `UPLOAD_DIR` | File upload directory | `./uploads` |
| `OPENAI_KEY` | OpenAI API key | - |

## API Quick Start

### 1. Register a user

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "myuser", "display_name": "My Name", "password": "secret123"}'
```

### 2. Use the token

```bash
curl http://localhost:8080/api/channels \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 3. Send a message

```bash
curl -X POST http://localhost:8080/api/messages \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel_id": "CHANNEL_UUID", "content": "Hello, world!"}'
```

## Documentation

Full documentation available in the `docs/` folder:

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
