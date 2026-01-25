# Smack Server API Documentation

Base URL: `http://localhost:8080`

## Authentication

All protected endpoints require a Bearer token in the Authorization header.

### Obtaining a Token

Tokens are obtained by registering or logging in:

**Register a new user:**
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "myuser", "display_name": "My Name", "password": "mypassword"}'
```

**Login with existing user:**
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "myuser", "password": "mypassword"}'
```

Both endpoints return a JWT token in the response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": { ... }
}
```

### Using the Token

Include the token in the `Authorization` header with the `Bearer` prefix:

```bash
curl http://localhost:8080/api/channels \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Token Details

- **Expiration:** 7 days from issue
- **Algorithm:** HS256 (HMAC SHA-256)
- **Format:** `Authorization: Bearer <token>`

---

## Public Endpoints

### Health Check

```
GET /health
```

Returns server health status.

**Response:**
```json
{
  "status": "ok"
}
```

---

### Register

```
POST /api/auth/register
```

Create a new user account.

**Request Body:**
```json
{
  "username": "string",
  "display_name": "string",
  "password": "string"
}
```

**Response:** `201 Created`
```json
{
  "token": "jwt_token_string",
  "user": {
    "id": "uuid",
    "username": "string",
    "display_name": "string",
    "avatar_url": "string|null",
    "status": "offline",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

**Errors:**
- `400` - Invalid request body or password too short (min 6 chars)
- `409` - Username already exists

---

### Login

```
POST /api/auth/login
```

Authenticate and receive a JWT token.

**Request Body:**
```json
{
  "username": "string",
  "password": "string"
}
```

**Response:** `200 OK`
```json
{
  "token": "jwt_token_string",
  "user": {
    "id": "uuid",
    "username": "string",
    "display_name": "string",
    "avatar_url": "string|null",
    "status": "online",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

**Errors:**
- `400` - Invalid request body
- `401` - Invalid credentials

---

### WebSocket Connection

```
GET /api/ws?token=<jwt_token>
```

Establish a WebSocket connection for real-time events.

**Query Parameters:**
- `token` - JWT authentication token

**WebSocket Message Types (Inbound from Server):**

| Type | Description |
|------|-------------|
| `new_message` | New message posted to a channel |
| `message_deleted` | Message was deleted |
| `user_online` | User came online |
| `user_offline` | User went offline |
| `typing` | User is typing in a channel |
| `channel_update` | Channel metadata changed |
| `reaction_update` | Reaction added/removed |
| `message_stream_start` | AI message streaming started |
| `message_stream_delta` | AI streaming chunk |
| `message_stream_end` | AI streaming complete |
| `reminder` | Reminder triggered |

**WebSocket Message Types (Outbound to Server):**

| Type | Description |
|------|-------------|
| `subscribe` | Subscribe to a channel's events |
| `typing` | Notify typing in a channel |

**Example Subscribe Message:**
```json
{
  "type": "subscribe",
  "channel_id": "uuid"
}
```

---

## Protected Endpoints

All endpoints below require authentication.

---

## Users

### Get Current User

```
GET /api/auth/me
GET /api/users/me
```

Returns the authenticated user's information.

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "username": "string",
  "display_name": "string",
  "avatar_url": "string|null",
  "status": "online|offline|away|dnd",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### List All Users

```
GET /api/users
```

Returns all registered users.

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "username": "string",
    "display_name": "string",
    "avatar_url": "string|null",
    "status": "online",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Get User by ID

```
GET /api/users/{id}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "username": "string",
  "display_name": "string",
  "avatar_url": "string|null",
  "status": "online",
  "created_at": "2024-01-01T00:00:00Z"
}
```

**Errors:**
- `404` - User not found

---

### Update Profile

```
PUT /api/users/me
```

Update the current user's profile.

**Request Body:**
```json
{
  "display_name": "string",
  "avatar_url": "string"
}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "username": "string",
  "display_name": "string",
  "avatar_url": "string",
  "status": "online",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### Update Status

```
PUT /api/users/me/status
```

Update user's online status.

**Request Body:**
```json
{
  "status": "online|offline|away|dnd"
}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "username": "string",
  "display_name": "string",
  "avatar_url": "string|null",
  "status": "away",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

## Channels

### List User's Channels

```
GET /api/channels
```

Returns channels the current user is a member of.

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "name": "general",
    "description": "string",
    "is_direct": false,
    "created_by": "uuid",
    "created_at": "2024-01-01T00:00:00Z",
    "unread_count": 5
  }
]
```

---

### List Public Channels

```
GET /api/channels/public
```

Returns all public (non-direct) channels.

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "name": "general",
    "description": "General discussion",
    "is_direct": false,
    "created_by": "uuid",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Create Channel

```
POST /api/channels
```

**Request Body:**
```json
{
  "name": "string",
  "description": "string"
}
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "name": "string",
  "description": "string",
  "is_direct": false,
  "created_by": "uuid",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### Get Channel

```
GET /api/channels/{id}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "string",
  "description": "string",
  "is_direct": false,
  "created_by": "uuid",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### Join Channel

```
POST /api/channels/{id}/join
```

**Response:** `200 OK`
```json
{
  "message": "joined channel"
}
```

---

### Leave Channel

```
POST /api/channels/{id}/leave
```

**Response:** `200 OK`
```json
{
  "message": "left channel"
}
```

---

### Get Channel Members

```
GET /api/channels/{id}/members
```

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "username": "string",
    "display_name": "string",
    "avatar_url": "string|null",
    "status": "online",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Mark Channel as Read

```
POST /api/channels/{id}/read
```

**Response:** `200 OK`
```json
{
  "message": "marked as read"
}
```

---

### Mute Channel

```
POST /api/channels/{id}/mute
```

**Response:** `200 OK`
```json
{
  "message": "channel muted"
}
```

---

### Unmute Channel

```
POST /api/channels/{id}/unmute
```

**Response:** `200 OK`
```json
{
  "message": "channel unmuted"
}
```

---

### Get Muted Channels

```
GET /api/channels/muted
```

**Response:** `200 OK`
```json
["channel_id_1", "channel_id_2"]
```

---

### Create Direct Message

```
POST /api/dm
```

Create or get existing DM channel with another user.

**Request Body:**
```json
{
  "user_id": "uuid"
}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "dm-user1-user2",
  "description": "",
  "is_direct": true,
  "created_by": "uuid",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

## Messages

### Get Channel Messages

```
GET /api/channels/{id}/messages?limit=50&before=<message_id>
```

**Query Parameters:**
- `limit` - Number of messages to return (default: 50, max: 100)
- `before` - Return messages before this message ID (for pagination)

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "channel_id": "uuid",
    "user": {
      "id": "uuid",
      "username": "string",
      "display_name": "string",
      "avatar_url": "string|null"
    },
    "content": "Hello world!",
    "thread_id": "uuid|null",
    "reply_count": 3,
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Send Message

```
POST /api/messages
```

**Request Body:**
```json
{
  "channel_id": "uuid",
  "content": "string"
}
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "channel_id": "uuid",
  "user": {
    "id": "uuid",
    "username": "string",
    "display_name": "string",
    "avatar_url": "string|null"
  },
  "content": "string",
  "thread_id": null,
  "reply_count": 0,
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### Delete Message

```
DELETE /api/messages/{id}
```

Only the message author can delete their messages.

**Response:** `200 OK`
```json
{
  "message": "deleted"
}
```

**Errors:**
- `403` - Not authorized to delete this message
- `404` - Message not found

---

### Get Thread Messages

```
GET /api/messages/{id}/thread
```

Get all replies to a parent message.

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "channel_id": "uuid",
    "user": {
      "id": "uuid",
      "username": "string",
      "display_name": "string",
      "avatar_url": "string|null"
    },
    "content": "string",
    "thread_id": "parent_message_id",
    "reply_count": 0,
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Reply to Thread

```
POST /api/messages/{id}/reply
```

**Request Body:**
```json
{
  "content": "string"
}
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "channel_id": "uuid",
  "user": {...},
  "content": "string",
  "thread_id": "parent_message_id",
  "reply_count": 0,
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

## Reactions

### Add Reaction

```
POST /api/reactions
```

**Request Body:**
```json
{
  "message_id": "uuid",
  "emoji": "string"
}
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "message_id": "uuid",
  "user_id": "uuid",
  "emoji": "string",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### Remove Reaction

```
DELETE /api/reactions
```

**Request Body:**
```json
{
  "message_id": "uuid",
  "emoji": "string"
}
```

**Response:** `200 OK`
```json
{
  "message": "reaction removed"
}
```

---

### Get Message Reactions

```
GET /api/messages/{id}/reactions
```

**Response:** `200 OK`
```json
[
  {
    "emoji": "string",
    "count": 3,
    "users": ["user_id_1", "user_id_2", "user_id_3"]
  }
]
```

---

## Reminders

### List Reminders

```
GET /api/reminders
```

Get all pending reminders for the current user.

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "user_id": "uuid",
    "channel_id": "uuid",
    "message": "string",
    "remind_at": "2024-01-01T12:00:00Z",
    "created_at": "2024-01-01T00:00:00Z",
    "completed": false
  }
]
```

---

### Create Reminder

```
POST /api/reminders
```

**Request Body:**
```json
{
  "channel_id": "uuid",
  "message": "string",
  "remind_at": "2024-01-01T12:00:00Z"
}
```

Alternatively, use relative time:
```json
{
  "channel_id": "uuid",
  "message": "string",
  "remind_in": "30m"
}
```

Supported relative formats: `5m`, `1h`, `2h30m`, `1d`

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "channel_id": "uuid",
  "message": "string",
  "remind_at": "2024-01-01T12:00:00Z",
  "created_at": "2024-01-01T00:00:00Z",
  "completed": false
}
```

---

### Delete Reminder

```
DELETE /api/reminders/{id}
```

**Response:** `200 OK`
```json
{
  "message": "reminder deleted"
}
```

---

## Bots

### List Bots

```
GET /api/bots
```

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "name": "openai",
    "display_name": "ChatGPT",
    "description": "AI assistant powered by OpenAI",
    "provider": "openai",
    "model": "gpt-4",
    "avatar_url": "string|null",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Get Bot

```
GET /api/bots/{id}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "openai",
  "display_name": "ChatGPT",
  "description": "string",
  "provider": "openai",
  "model": "gpt-4",
  "avatar_url": "string|null",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### Create Bot DM

```
POST /api/bots/dm
```

Create a direct message channel with a bot.

**Request Body:**
```json
{
  "bot_id": "uuid"
}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "bot-dm-openai-username",
  "description": "",
  "is_direct": true,
  "created_by": "uuid",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

## Files

### Upload File

```
POST /api/files/upload
Content-Type: multipart/form-data
```

**Form Fields:**
- `file` - The file to upload

**Response:** `200 OK`
```json
{
  "url": "/api/files/abc123-filename.png",
  "filename": "abc123-filename.png"
}
```

---

### Get File

```
GET /api/files/{filename}
```

Returns the file with appropriate Content-Type header.

---

## Webhooks

### Create Webhook

```
POST /api/webhooks
```

Create an inbound webhook for posting messages to a channel.

**Request Body:**
```json
{
  "name": "string",
  "channel_id": "uuid"
}
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "name": "GitHub Notifications",
  "channel_id": "uuid",
  "token": "webhook_token_string",
  "created_by": "uuid",
  "created_at": "2024-01-01T00:00:00Z",
  "url": "/api/webhooks/incoming/{id}/{token}"
}
```

---

### List Webhooks

```
GET /api/webhooks?channel_id={channel_id}
```

List webhooks. Optionally filter by channel.

**Query Parameters:**
- `channel_id` (optional) - Filter by channel

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "name": "GitHub Notifications",
    "channel_id": "uuid",
    "token": "webhook_token_string",
    "created_by": "uuid",
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### Get Webhook

```
GET /api/webhooks/{id}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "string",
  "channel_id": "uuid",
  "token": "webhook_token_string",
  "created_by": "uuid",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### Delete Webhook

```
DELETE /api/webhooks/{id}
```

**Response:** `200 OK`
```json
{
  "message": "webhook deleted"
}
```

---

### Incoming Webhook (Public)

```
POST /api/webhooks/incoming/{id}/{token}
```

Post a message via webhook. No authentication required - the token in the URL acts as authentication.

**Request Body:**
```json
{
  "content": "string",
  "username": "string (optional)",
  "avatar_url": "string (optional)"
}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "channel_id": "uuid",
  "content": "string",
  "created_at": "2024-01-01T00:00:00Z"
}
```

**Errors:**
- `400` - Invalid request body or empty content
- `404` - Webhook not found or invalid token

---

## Error Responses

All endpoints return errors in this format:

```json
{
  "error": "Error message description"
}
```

Common HTTP status codes:
- `400` - Bad Request (invalid input)
- `401` - Unauthorized (missing or invalid token)
- `403` - Forbidden (not allowed to perform action)
- `404` - Not Found
- `409` - Conflict (e.g., duplicate username)
- `500` - Internal Server Error

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DB_PATH` | SQLite database path | `./smack.db` |
| `UPLOAD_DIR` | File upload directory | `./uploads` |
| `OPENAI_KEY` | OpenAI API key for bot | - |
