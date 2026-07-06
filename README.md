# Real-Time Chat Application Backend

A production-grade real-time chat backend built with Go, WebSockets, Redis, and MySQL.

## Features

- **Real-Time Messaging** — WebSocket-based instant messaging with sub-second latency
- **JWT Authentication** — Secure access & refresh token system with session management
- **Chat Rooms** — Create, join, and manage public/private chat rooms
- **Message History** — Paginated message retrieval stored in MySQL
- **Redis Caching** — Active session caching, online user tracking, and pub/sub broadcasting
- **Rate Limiting** — Redis-backed rate limiting to prevent message spam
- **Typing Indicators** — Real-time typing/stop-typing notifications
- **Horizontal Scaling** — Redis Pub/Sub enables running multiple server instances
- **Graceful Shutdown** — Clean connection teardown on server stop

## Tech Stack

| Technology | Purpose |
|-----------|---------|
| **Go** | Backend language |
| **Gin** | HTTP framework |
| **Gorilla WebSocket** | WebSocket protocol |
| **MySQL** | Persistent storage |
| **Redis** | Caching & Pub/Sub |
| **JWT** | Authentication |
| **bcrypt** | Password hashing |

## Prerequisites

- Go 1.21+
- MySQL 8.0+
- Redis 7.0+

## Setup

### 1. Clone & Configure

```bash
git clone https://github.com/yaswa/go-chat-backend.git
cd go-chat-backend
cp .env.example .env
# Edit .env with your MySQL and Redis credentials
```

### 2. Setup Database

```bash
mysql -u root -p < migrations/schema.sql
```

Or let the application auto-migrate on first run.

### 3. Install Dependencies & Run

```bash
go mod tidy
go run cmd/server/main.go
```

Server starts at `http://localhost:8080`

## API Endpoints

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/register` | Register new user |
| POST | `/api/auth/login` | Login & get JWT tokens |
| POST | `/api/auth/refresh` | Refresh access token |
| POST | `/api/auth/logout` | Logout & invalidate session |

### Users

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/users/me` | Get own profile |
| PUT | `/api/users/me` | Update own profile |
| GET | `/api/users/:id` | Get user by ID |

### Chat Rooms

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/rooms` | Create room |
| GET | `/api/rooms` | List my rooms |
| GET | `/api/rooms/:id` | Get room details |
| PUT | `/api/rooms/:id` | Update room (owner) |
| DELETE | `/api/rooms/:id` | Delete room (owner) |
| POST | `/api/rooms/:id/join` | Join room |
| POST | `/api/rooms/:id/leave` | Leave room |
| GET | `/api/rooms/:id/members` | List room members |

### Messages

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/rooms/:id/messages` | Get messages (paginated) |

### WebSocket

Connect: `ws://localhost:8080/ws?token=<jwt_access_token>`

#### Message Types (Client → Server)

```json
// Send a chat message
{"type": "chat_message", "room_id": 1, "content": "Hello!"}

// Typing indicator
{"type": "typing", "room_id": 1}

// Stop typing
{"type": "stop_typing", "room_id": 1}

// Join room broadcast
{"type": "join_room", "room_id": 1}

// Leave room broadcast
{"type": "leave_room", "room_id": 1}
```

#### Message Types (Server → Client)

```json
// Incoming chat message
{
  "type": "chat_message",
  "room_id": 1,
  "content": "Hello!",
  "sender_id": 42,
  "sender": "john",
  "timestamp": "2026-05-17T10:30:00Z",
  "data": {"message_id": 123}
}

// System notification
{"type": "system", "room_id": 1, "content": "john is now online"}

// Error
{"type": "error", "content": "Rate limit exceeded"}
```

## Project Structure

```
go-chat-backend/
├── cmd/server/main.go           # Entry point
├── internal/
│   ├── config/config.go         # Environment configuration
│   ├── database/
│   │   ├── mysql.go             # MySQL connection & migrations
│   │   └── redis.go             # Redis client & helpers
│   ├── models/
│   │   ├── user.go              # User model & queries
│   │   ├── chatroom.go          # Chat room model & queries
│   │   └── message.go           # Message model & queries
│   ├── auth/
│   │   ├── jwt.go               # JWT token generation/validation
│   │   └── middleware.go        # Auth middleware
│   ├── handlers/
│   │   ├── user_handler.go      # User REST endpoints
│   │   ├── chatroom_handler.go  # Room REST endpoints
│   │   └── message_handler.go   # Message REST endpoints
│   └── websocket/
│       ├── hub.go               # WebSocket connection hub
│       ├── client.go            # WebSocket client handler
│       └── message.go           # Message type definitions
├── migrations/schema.sql        # Database schema
├── .env.example                 # Configuration template
└── go.mod                       # Go module definition
```

## Architecture

```
Client ──► HTTP Server (Gin) ──► JWT Auth Middleware
                │                        │
                ├── REST Handlers ───► MySQL (persistent storage)
                │                        │
                └── WebSocket Hub ──► Redis Pub/Sub (cross-instance)
                        │                │
                        ├── Client Pool  └── Session Cache
                        └── Room Registry
```


