# ADLTS Core Engine

[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14+-336791.svg)](https://www.postgresql.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A production-grade API engine for automated driving license testing systems. Manages test orchestration, real-time video analysis coordination, payment processing, and result reporting across multiple test centers and devices.

## Quick Start

### Prerequisites

- Go 1.24 or later
- PostgreSQL 14+
- Docker & Docker Compose (optional)

### Get Running in 3 Minutes

```bash
# Clone and configure
git clone <repository-url>
cd adlts-core-engine
cp .env.example .env

# Set required variables
export DATABASE_URL="postgres://user:pass@localhost:5432/adlts"
export JWT_SECRET="your-32-byte-random-key"

# Run migrations
psql $DATABASE_URL < migrations/001_schema.sql
psql $DATABASE_URL < migrations/002_testing_core.sql
psql $DATABASE_URL < migrations/003_create_appeal_evidence.sql

# Start the server
go run ./cmd/api

# Server listening on http://localhost:8080
curl http://localhost:8080/health

# Seed integration data
DATABASE_URL=$DATABASE_URL go run ./cmd/seed/main.go
```

Seeded login accounts:
- `super@adlts.et` / `SuperAdmin123!`
- `admin@adlts.et` / `Admin123!`
- `candidate@test.et` / `Candidate123!`
- `institute@test.et` / `Institute123!`
- `expert@test.et` / `Expert123!`
- `authority@test.et` / `Authority123!`

**Or with Docker:**

```bash
docker-compose up -d
# API available at http://localhost:8080
```

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Configuration](#configuration)
- [API Usage](#api-usage)
- [Development](#development)
- [Database](#database)
- [Deployment](#deployment)
- [Contributing](#contributing)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## Features

- **Test Execution Orchestration**: State machine-driven exam lifecycle with atomic database transactions
- **Real-Time Video Analysis**: HTTP integration with Python FastAPI computer vision service for lane detection and traffic rule compliance scoring
- **Multi-Role Access Control**: Nine entity types with fine-grained role-based authorization (super-admin, examiner, candidate, institute, expert, etc.)
- **Payment Processing**: Chapa payment provider integration with webhook support and transaction reconciliation
- **Recording Archival**: MinIO S3-compatible storage for test session recordings and frame-level analysis results
- **Automated Workflows**: Background jobs for test expiry, notification delivery, and state synchronization
- **Result Reporting**: PDF generation with AI-powered narrative summaries via Gemini API integration
- **Authentication**: JWT-based token system with bcrypt password hashing and stateless session management
- **Audit Trails**: Complete operation history with actor identity and timestamps on all mutable entities
- **Email Delivery**: SMTP integration for one-time passwords, password resets, and notifications

## Architecture

### High-Level Overview

```
┌─────────────────┐
│  Client Apps    │  (Web, Mobile)
└────────┬────────┘
         │ REST API
         │
    ┌────▼──────────────────────────┐
    │  ADLTS Core Engine (Go)        │
    │  ─────────────────────────────  │
    │  • Identity & Auth              │
    │  • Booking & Payment            │
    │  • Test Orchestration           │
    │  • Recording Coordination       │
    │  • Result Reporting             │
    │  • Appeals Management           │
    └────┬─────────────┬──────────────┘
         │             │
    ┌────▼──┐     ┌────▼─────────────┐
    │ PgSQL │     │ Python CV Service │
    │       │     │ (Lane Detection)  │
    └───────┘     └─────┬────────────┘
                        │
                   ┌────▼─────┐
                   │ ESP32-CAM │
                   │ (Device)  │
                   └───────────┘
```

### Service Modules

| Module | Purpose | Key Entities |
|--------|---------|--------------|
| **Identity** | User authentication, account management, role assignment | Users, roles, credentials |
| **Booking** | Scheduling, payment processing, confirmation workflows | Bookings, payments, invoices |
| **Testing** | Core exam lifecycle, device management, scoring | Tests, devices, sessions, results |
| **Recording** | Video archival and playback coordination | Recordings, frame analyses |
| **Appeals** | Test result challenges with evidence | Appeals, evidence, verdicts |
| **Reporting** | PDF report generation with analytics | Reports, narratives, statistics |

### Platform Components

- **Database Layer**: pgx connection pooling with row-level locking and atomic transactions
- **Security**: JWT tokens, bcrypt hashing, CORS middleware, role-based authorization
- **HTTP Router**: chi/v5 composable middleware for logging, recovery, timeout enforcement
- **File Management**: Upload validation, media streaming, byte-range request support
- **Object Storage**: MinIO S3-compatible API for recordings and analysis artifacts
- **Email**: SMTP client with asynchronous delivery and template support
- **External Services**: Chapa (payments), Gemini (AI summaries), Python CV (video analysis)

## Configuration

### Environment Variables

Required variables:

```env
DATABASE_URL=postgres://user:password@localhost:5432/adlts_db
JWT_SECRET=<32-byte-random-string>
PORT=8080
```

Optional variables with defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `INTERNAL_API_KEY` | empty | Shared secret for internal service-to-service calls |
| `SUPER_ADMIN_EMAIL` | root@adlts.et | Initial admin account email |
| `SUPER_ADMIN_PASSWORD` | empty | Initial admin password (auto-generated if empty) |
| `UPLOADS_DIR` | ../uploads | Base directory for file uploads |
| `MEDIA_MAX_SIZE_MB` | 5 | Maximum upload size in MB |
| `CHAPA_SECRET_KEY` | empty | Payment provider API key |
| `CHAPA_BASE_URL` | https://api.chapa.co/v1 | Payment provider endpoint |
| `SMTP_HOST` | empty | Email server hostname |
| `SMTP_PORT` | 587 | Email server port |
| `SMTP_USER` | empty | Email server username |
| `SMTP_PASSWORD` | empty | Email server password |
| `SMTP_FROM` | noreply@adlts.et | Sender email address |
| `MINIO_ENDPOINT` | localhost:9000 | Object storage endpoint |
| `MINIO_BUCKET` | recordings | Storage bucket name |
| `LANE_DETECTOR_URL` | http://localhost:8000 | Python CV service URL |
| `GEMINI_API_KEY` | empty | AI text generation API key |

Copy `.env.example` and customize for your environment.

## API Usage

### Authentication

All authenticated endpoints require a bearer token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/tests
```

### Core Endpoints

**Tests** — Manage exam lifecycle:

```bash
# Create test
POST /api/v1/tests
{
  "candidate_id": "uuid",
  "test_plan_id": "uuid",
  "booking_id": "uuid"
}

# Get test status
GET /api/v1/tests/:id

# Transition test state
POST /api/v1/tests/:id/begin-exam
POST /api/v1/tests/:id/end-exam
POST /api/v1/tests/:id/abort
```

**Devices** — Register and monitor hardware:

```bash
# Register device
POST /api/v1/devices
{
  "device_code": "DEVICE_001",
  "stream_url": "http://192.168.1.10/stream"
}

# Health check
GET /api/v1/devices/:id/health
```

**Bookings** — Schedule exams with payment:

```bash
# Create booking
POST /api/v1/bookings
{
  "candidate_id": "uuid",
  "test_plan_id": "uuid",
  "scheduled_at": "2026-06-15T10:00:00Z"
}

# Confirm payment
POST /api/v1/bookings/:id/confirm-payment
```

See [TESTING_CORE_API_DOCUMENTATION.md](docs/TESTING_CORE_API_DOCUMENTATION.md) for complete endpoint reference.

## Development

### Project Structure

```
adlts-core-engine/
├── cmd/api/                 # Application entry point
├── internal/
│   ├── app/                 # Service composition
│   ├── identity/            # Authentication & user management
│   ├── booking/             # Scheduling & payments
│   ├── testing/             # Core exam orchestration
│   ├── recording/           # Video archival
│   ├── appeal/              # Result disputes
│   ├── reporting/           # Report generation
│   └── platform/            # Cross-cutting concerns
│       ├── config/          # Environment binding
│       ├── db/              # Database connection
│       ├── security/        # JWT, authorization
│       ├── media/           # File serving
│       ├── minio/           # Object storage
│       └── mailer/          # Email delivery
├── migrations/              # Database schema versions
├── tests/                   # Integration tests
├── Dockerfile               # Container definition
└── go.mod                   # Dependencies
```

### Building from Source

```bash
# Install dependencies
go mod download

# Build binary
go build -o adlts-api ./cmd/api

# Run tests
go test ./...

# Build Docker image
docker build -t adlts-core-engine .
```

### Code Organization

The codebase follows standard Go project layout:

- **Domain models** in `internal/domain/` — shared entities and enums
- **Service logic** in `internal/{module}/service.go` — business rules
- **Data access** in `internal/{module}/repository.go` — database operations
- **HTTP handlers** in `internal/{module}/handler.go` — request/response mapping
- **Platform utilities** in `internal/platform/` — reusable infrastructure

### Making Changes

1. Create a feature branch: `git checkout -b feature/description`
2. Make changes and add tests
3. Run linter: `go fmt ./...` and `go vet ./...`
4. Commit with clear messages: `git commit -m "feat: add X feature"`
5. Push and open a pull request

## Database

### Schema Management

Database migrations are versioned SQL files in `migrations/`:

```bash
# Apply all migrations
for file in migrations/*.sql; do
  psql $DATABASE_URL < "$file"
done

# Or apply individually
psql $DATABASE_URL < migrations/001_schema.sql
```

### Key Tables

- `identities` — Users with roles and credentials
- `tests` — Exam records with state and scoring
- `bookings` — Schedule entries with payment status
- `devices` — Hardware registration and status
- `sessions` — Maneuver attempt records
- `frame_analyses` — Per-frame CV results
- `appeals` — Test result challenges

### Row-Level Locking

Test state transitions use database-level locking to prevent race conditions:

```go
// Atomic status transition
UPDATE tests SET status='running' WHERE id=$1 AND status='ready'
```

The `WHERE` clause acts as optimistic concurrency control — if the condition fails, the transition is rejected.

## Deployment

### Docker Compose

Start the complete stack with a single command:

```bash
docker-compose up -d
```

This starts:
- ADLTS API on port 8080
- PostgreSQL on port 5432
- MinIO object storage on port 9000

### Production Deployment

For production, use environment-specific configurations:

1. Set all required environment variables via secrets manager
2. Use a managed PostgreSQL database (AWS RDS, Azure, etc.)
3. Use a managed S3-compatible storage (AWS S3, MinIO enterprise, etc.)
4. Enable HTTPS with a reverse proxy (Nginx, Caddy)
5. Monitor logs and metrics via your platform (CloudWatch, DataDog, etc.)

### Health Checks

Kubernetes-style health endpoints:

```bash
# Liveness probe
curl http://localhost:8080/health

# Readiness (checks DB connection)
curl http://localhost:8080/api/v1/health
```

## Troubleshooting

### Common Issues

**"DATABASE_URL is required"**

Set the environment variable before running:

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/adlts"
go run ./cmd/api
```

**"connection refused" on startup**

Ensure PostgreSQL is running and accessible:

```bash
psql $DATABASE_URL -c "SELECT 1"
```

**Migration errors**

Check that all migration files exist and are in order:

```bash
ls -la migrations/
```

**JWT token expired**

Tokens expire after a set duration. Request a new token by re-authenticating:

```bash
POST /api/v1/auth/login
```

### Debug Mode

Enable detailed logging:

```bash
LOGLEVEL=debug go run ./cmd/api
```

### Performance

For high-load scenarios:

- Increase PostgreSQL connection pool: `PGBOUNCER_MAX_CLIENT_CONN=1000`
- Enable Redis caching (not yet implemented, planned for v2)
- Scale API horizontally behind a load balancer

See [TESTING_CORE_API_DOCUMENTATION.md](docs/TESTING_CORE_API_DOCUMENTATION.md) for detailed API reference and [docs/](docs/) for architecture decisions.

## Contributing

Contributions are welcome. Please follow these guidelines:

1. Fork the repository
2. Create a feature branch with clear naming: `feature/user-auth`, `fix/state-transition`
3. Write tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Keep commits atomic and messages descriptive
6. Submit a pull request with context

Code style follows Go conventions enforced by `go fmt` and `go vet`.

## License

MIT License — see [LICENSE](LICENSE) for details.

Copyright (c) 2026 ADLTS-Lab

---

**Questions?** Check the [documentation](docs/) or open an issue on GitHub.
