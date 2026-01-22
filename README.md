# Attendance Engine

A secure, concurrent attendance tracking system with AI-based face recognition, built with Go.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-336791?logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)

## Features

- 🔐 **JWT Authentication** - Secure device registration and token-based auth
- 👤 **Face Recognition** - Pluggable AI face service integration
- ⚡ **High Performance** - Go + Redis queue for concurrent processing
- 📊 **Real-time Dashboard** - Modern web UI for monitoring
- 🐳 **Docker Ready** - One-command production deployment
- 📈 **Observability** - Prometheus metrics, structured logging
- 🔄 **Background Workers** - Async event processing

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Web Frontend  │────▶│    Go API       │────▶│   PostgreSQL    │
│   (Dashboard)   │     │  (Gin + JWT)    │     │   (Devices,     │
└─────────────────┘     └────────┬────────┘     │    Events)      │
                                 │              └─────────────────┘
                                 ▼
                        ┌─────────────────┐     ┌─────────────────┐
                        │   Redis Queue   │────▶│    Workers      │
                        │   (Check-ins)   │     │  (Face Match)   │
                        └─────────────────┘     └────────┬────────┘
                                                         │
                                                         ▼
                                                ┌─────────────────┐
                                                │  Face Service   │
                                                │  (Python/ONNX)  │
                                                └─────────────────┘
```

## Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- Make (optional but recommended)

### Development Setup

```bash
# Clone and enter directory
cd attendance-engine

# Start infrastructure (Postgres, Redis)
make dev

# In another terminal, run the API
make run-api

# In another terminal, run the worker
make run-worker
```

Or manually:

```bash
# Start Docker services
docker compose -f deploy/docker-compose.yml up -d

# Wait for services and apply migrations
docker cp migrations/0001_init.up.sql deploy-postgres-1:/tmp/init.sql
docker exec -e PGPASSWORD=attendance deploy-postgres-1 psql -U attendance -d attendance -f /tmp/init.sql

# Run API
go run ./cmd/api

# Run Worker (separate terminal)
go run ./cmd/worker
```

### Access the Application

- **Web Dashboard**: http://localhost:8081
- **Health Check**: http://localhost:8081/healthz
- **Metrics**: http://localhost:8081/metrics

## API Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/healthz` | Health check | No |
| GET | `/metrics` | Prometheus metrics | No |
| POST | `/v1/devices/register` | Register device, get JWT | No |
| POST | `/v1/checkins` | Submit attendance check-in | Yes |
| GET | `/v1/events` | List attendance events | Yes |

### Example Usage

```bash
# Register a device
curl -X POST http://localhost:8081/v1/devices/register \
  -H "Content-Type: application/json" \
  -d '{"device_id": "kiosk-001"}'

# Response:
# {"access_token": "eyJ...", "refresh_token": "eyJ...", "expires_at": 1234567890}

# Submit check-in (use access_token from above)
curl -X POST http://localhost:8081/v1/checkins \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{"user_id": "emp-123", "device_id": "kiosk-001", "image_url": "https://..."}'

# List events
curl -H "Authorization: Bearer <access_token>" \
  "http://localhost:8081/v1/events?limit=20"
```

## Production Deployment

### 1. Configure Environment

```bash
# Copy and edit production config
cp .env.production .env

# Generate secure JWT key
openssl rand -base64 32
# Add to .env: JWT_SIGNING_KEY=<generated_key>

# Set strong database password
# Edit .env: DB_PASSWORD=<strong_password>
```

### 2. Deploy with Docker Compose

```bash
# Build and start all services
docker compose -f docker-compose.prod.yml up -d

# View logs
docker compose -f docker-compose.prod.yml logs -f

# With Nginx reverse proxy (recommended)
docker compose -f docker-compose.prod.yml --profile with-nginx up -d
```

### 3. Verify Deployment

```bash
# Check all services
docker compose -f docker-compose.prod.yml ps

# Check health
curl http://localhost:8081/healthz
```

### Production Checklist

- [ ] Change `JWT_SIGNING_KEY` to secure random value
- [ ] Set strong `DB_PASSWORD`
- [ ] Set `APP_ENV=production`
- [ ] Set `FACE_SKIP=false` (with real face service)
- [ ] Configure SSL/TLS certificates
- [ ] Set up log aggregation
- [ ] Configure backups for PostgreSQL
- [ ] Set up monitoring alerts

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENV` | `development` | Environment (development/production) |
| `HTTP_PORT` | `8081` | HTTP server port |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `JWT_SIGNING_KEY` | - | **Required**: JWT signing secret |
| `JWT_ISSUER` | `attendance-engine` | JWT issuer claim |
| `ACCESS_TTL` | `15m` | Access token lifetime |
| `REFRESH_TTL` | `24h` | Refresh token lifetime |
| `FACE_SERVICE_URL` | `http://localhost:8000` | Face recognition service |
| `FACE_SKIP` | `true` | Skip face verification (dev only) |
| `QUEUE_BACKEND` | `redis` | Queue backend (redis/memory) |
| `RATE_LIMIT_PER_MIN` | `120` | Requests per minute per IP |

## Project Structure

```
.
├── cmd/
│   ├── api/           # HTTP API server
│   └── worker/        # Background worker
├── internal/
│   ├── attendance/    # Core business logic
│   ├── auth/          # JWT authentication
│   ├── config/        # Configuration
│   ├── faceclient/    # Face service client
│   ├── httpmiddleware/# Rate limiting, etc.
│   ├── queue/         # Redis/memory queue
│   └── store/         # Database & Redis
├── migrations/        # SQL migrations
├── web/               # Frontend assets
├── deploy/            # Deployment configs
├── Dockerfile         # Multi-stage build
├── docker-compose.prod.yml  # Production stack
└── Makefile          # Build commands
```

## Development

```bash
# Run tests
make test

# Build binaries
make build

# Lint code
make lint

# Clean build artifacts
make clean
```

## Face Recognition Service

The system expects a face service at `/embed` endpoint:

```json
// POST /embed
// Request
{"image_url": "https://..."}

// Response
{"embedding": [0.1, 0.2, ...], "score": 0.95}
```

For production, integrate with:
- Custom FastAPI + ONNX Runtime service
- AWS Rekognition
- Azure Face API
- Google Cloud Vision

## License

MIT License - see LICENSE file for details.
