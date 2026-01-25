# Smack Server

A real-time chat server written in Go with WebSocket support, REST API, AI bot integration, and more.

## Features

- **Real-time messaging** via WebSocket
- **REST API** for all operations
- **Channels** - public channels and direct messages
- **Threaded conversations** - reply to messages in threads
- **Reactions** - emoji reactions on messages
- **AI Bots** - OpenAI GPT integration with streaming responses
- **Kanban boards** - built-in project management
- **Custom commands** - create slash commands with AI generation
- **File uploads** - share images and files
- **Webhooks** - incoming webhooks for integrations
- **Reminders** - set reminders for yourself

## Quick Start

### Prerequisites

- Go 1.21 or later
- (Optional) OpenAI API key for AI bot features

### Running

```bash
# Clone the repository
git clone https://github.com/schappim/smack-server.git
cd smack-server

# Run the server
go run .

# Or build and run
go build -o smack-server .
./smack-server
```

The server starts on `http://localhost:8080` by default.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DB_PATH` | SQLite database path | `./smack.db` |
| `UPLOAD_DIR` | File upload directory | `./uploads` |
| `OPENAI_KEY` | OpenAI API key for AI bot | - |

## API Overview

See [API.md](API.md) for full documentation.

### Authentication

Register or login to get a JWT token:

```bash
# Register
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "myuser", "display_name": "My Name", "password": "mypassword"}'

# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "myuser", "password": "mypassword"}'
```

Use the token in subsequent requests:

```bash
curl http://localhost:8080/api/channels \
  -H "Authorization: Bearer <your-token>"
```

### WebSocket

Connect to `/api/ws?token=<jwt_token>` for real-time events:

- `new_message` - New message in a channel
- `message_deleted` - Message was deleted
- `user_online` / `user_offline` - User presence
- `typing` - User is typing
- `message_stream_start/delta/end` - AI streaming responses
- `reminder` - Reminder triggered

### Key Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /api/auth/register` | Create account |
| `POST /api/auth/login` | Login |
| `GET /api/channels` | List your channels |
| `POST /api/channels` | Create a channel |
| `GET /api/channels/{id}/messages` | Get messages |
| `POST /api/messages` | Send a message |
| `POST /api/dm` | Create direct message |
| `GET /api/bots` | List AI bots |
| `POST /api/bots/dm` | Chat with a bot |
| `POST /api/files/upload` | Upload a file |
| `POST /api/webhooks` | Create a webhook |

## Project Structure

```
.
├── main.go              # Entry point and routes
├── ai/                  # AI client (OpenAI)
├── handlers/            # HTTP handlers
├── middleware/          # Auth middleware
├── models/              # Data models
├── store/               # SQLite database layer
├── commands/            # Custom command interpolation
└── API.md               # API documentation
```

## License

MIT
