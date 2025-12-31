# webchat

**English** | [中文](README.zh.md)

A lightweight real-time group chat demo built with Go: Gin serves HTTP endpoints, Melody manages WebSocket sessions, and the UI is a single `index.html` file embedded into the binary via `embed`.

## Features

- Real-time group chat (WebSocket)
- Text messaging
- File/image sharing (HTTP upload, broadcast to the room with download links)
- Paste-to-upload (screenshots/clipboard files)
- Local chat history (localStorage), search, clear
- Unread counter + optional browser notifications
- User differentiation: a browser “fingerprint” generates a stable `userId` (persisted locally); nickname is display-only

## Quick Start

### Requirements

- Go (compatible with the version in `go.mod`)

### Run

```bash
go run .
```

Open:

- `http://localhost:5000`

## Routes & Protocol

### HTTP

- `GET /`: serves the embedded UI (`index.html`)
- `POST /upload`: upload a file (`multipart/form-data`)
  - fields: `file` (file), `userId`, `userName`
- `GET /files/:filename`: download a file

### WebSocket

- `GET /ws?userId=...&userName=...`
- Client -> server:
  - text: `{"type":"text","userId":"...","userName":"...","content":"..."}`
  - heartbeat: `{"type":"ping"}`
- Server broadcasts:
  - user message: `{"type":"user","userId":"...","userName":"...","content":"..."}`
  - system message: `{"type":"system","content":"..."}`
  - file message: `{"type":"file","filename":"...","filesize":123,"userId":"...","userName":"..."}`

## File Storage

- Uploaded files are saved to: `./files/`
- Name collisions are resolved by appending a numeric suffix: `name(1).ext`, `name(2).ext`, ...
- Download URL: `/files/{filename}` (the frontend URL-encodes `filename`)

## Security & Limits (Important)

This project is intended as a demo and does not include authentication/authorization. Do not expose it directly to the public internet as-is.

- Filename validation: blocks `..` and path separators to prevent directory traversal
- Upload size limit: default `20MiB`
- WebSocket Origin validation: same-origin only (same Host with `http/https` Origin)
- Download hardening: `X-Content-Type-Options: nosniff`; non-images are served as attachments

For any public deployment, you should add at least: auth, rate limiting, audit logging, file-type allowlists / malware scanning, and move storage to an object store.

## Development Notes

- Backend entry: `main.go`
- Frontend UI: `index.html` (embedded and served by the backend)
