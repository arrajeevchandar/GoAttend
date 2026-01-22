.PHONY: help dev prod build test clean migrate docker-build docker-up docker-down

# Default target
help:
	@echo "Attendance Engine - Available Commands:"
	@echo ""
	@echo "Development:"
	@echo "  make dev           - Start development environment (Docker + local API)"
	@echo "  make run-api       - Run API server locally"
	@echo "  make run-worker    - Run worker locally"
	@echo "  make test          - Run all tests"
	@echo ""
	@echo "Database:"
	@echo "  make migrate       - Apply database migrations"
	@echo "  make migrate-down  - Rollback database migrations"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build  - Build Docker images"
	@echo "  make docker-up     - Start all services with Docker Compose"
	@echo "  make docker-down   - Stop all services"
	@echo "  make docker-logs   - View container logs"
	@echo ""
	@echo "Production:"
	@echo "  make prod          - Start production environment"
	@echo "  make prod-build    - Build production images"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make lint          - Run linters"
	@echo "  make gen-key       - Generate secure JWT key"

# Development
dev:
	docker compose -f deploy/docker-compose.yml up -d
	@echo "Waiting for services..."
	@sleep 3
	@echo "Applying migrations..."
	docker cp migrations/0001_init.up.sql deploy-postgres-1:/tmp/init.sql
	docker exec -e PGPASSWORD=attendance deploy-postgres-1 psql -U attendance -d attendance -f /tmp/init.sql || true
	@echo ""
	@echo "Development environment ready!"
	@echo "  API:    http://localhost:8081"
	@echo "  DB:     localhost:5433"
	@echo "  Redis:  localhost:6379"
	@echo ""
	@echo "Run 'make run-api' in another terminal to start the API"

run-api:
	FACE_SKIP=true go run ./cmd/api

run-worker:
	go run ./cmd/worker

test:
	go test -v -race -cover ./...

# Database
migrate:
	docker cp migrations/0001_init.up.sql deploy-postgres-1:/tmp/init.sql
	docker exec -e PGPASSWORD=attendance deploy-postgres-1 psql -U attendance -d attendance -f /tmp/init.sql

migrate-down:
	docker cp migrations/0001_init.down.sql deploy-postgres-1:/tmp/down.sql
	docker exec -e PGPASSWORD=attendance deploy-postgres-1 psql -U attendance -d attendance -f /tmp/down.sql

# Docker Development
docker-build:
	docker compose -f deploy/docker-compose.yml build

docker-up:
	docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

docker-logs:
	docker compose -f deploy/docker-compose.yml logs -f

# Production
prod:
	docker compose -f docker-compose.prod.yml up -d

prod-build:
	docker compose -f docker-compose.prod.yml build

prod-down:
	docker compose -f docker-compose.prod.yml down

prod-logs:
	docker compose -f docker-compose.prod.yml logs -f

# With Nginx
prod-nginx:
	docker compose -f docker-compose.prod.yml --profile with-nginx up -d

# Utilities
clean:
	rm -rf bin/
	go clean -cache

lint:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "Install golangci-lint for full linting"

gen-key:
	@echo "Generated JWT Key:"
	@openssl rand -base64 32

# Build binaries
build:
	CGO_ENABLED=0 go build -ldflags='-w -s' -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -ldflags='-w -s' -o bin/worker ./cmd/worker
	@echo "Binaries built in bin/"
