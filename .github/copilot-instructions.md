<!-- Use this file to provide workspace-specific custom instructions to Copilot. For more details, visit https://code.visualstudio.com/docs/copilot/copilot-customization#_use-a-githubcopilotinstructionsmd-file -->

## Project: Secure Concurrent Attendance Engine with AI Face Recognition (Go)

### Architecture
- **API:** Gin HTTP + gRPC proto stub, JWT auth (HS256), device registration, check-in endpoints
- **Attendance core:** In-memory service with dedup stub, face client placeholder (HTTP call to Python microservice)
- **Storage:** Postgres (pgx) + Redis placeholders; queue is in-memory channel (replace with durable queue for production)
- **Security:** JWT tokens, mTLS-ready, configurable signing keys
- **Concurrency:** Worker pool pattern, context timeouts

### Key files
- `cmd/api/main.go` - HTTP server (Gin) with device register, check-in, healthz
- `cmd/worker/main.go` - Worker placeholder consuming queue messages
- `internal/config/config.go` - Env-based config with defaults
- `internal/auth/jwt.go` - JWT issue logic (access + refresh)
- `internal/attendance/service.go` - Check-in business logic
- `internal/faceclient/client.go` - Face service client stub
- `internal/store/` - DB and Redis wrappers
- `internal/queue/queue.go` - In-memory queue placeholder
- `proto/device.proto` - gRPC service definitions
- `deploy/docker-compose.yml` - Dev stack (Postgres, Redis, face svc)

### Development
- Run: `go run ./cmd/api` (port 8081 default)
- Task: "Run Attendance API" background task available
- Health: `http://localhost:8081/healthz`
- Device register: POST `/v1/devices/register` with `{"device_id":"..."}`
- Check-in: POST `/v1/checkins` with `{"user_id":"...", "device_id":"...", "image_url":"..."}`

### Next steps
- Replace in-memory queue with Redis/Kafka
- Integrate real face recognition service (Python FastAPI + onnxruntime)
- Add DB migrations and persistence for attendance events
- Implement mTLS for device-to-service auth
- Add rate limiting and circuit breakers

### Implemented (current scope)
- ✅ Postgres migrations for devices, attendance_events, refresh_tokens
- ✅ Repository pattern wired to HTTP API with dedup window
- ✅ Redis-backed queue (with in-memory fallback for tests)
- ✅ Worker consuming queue, calling face service stub, updating event status
- ✅ JWT auth on protected endpoints with device claims validation
- ✅ In-memory token-bucket rate limiting middleware
- ✅ Prometheus /metrics endpoint
- ✅ Events listing with pagination
- ✅ Updated proto (ListEvents, status, match_score) and OpenAPI
