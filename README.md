# MediaDock

> **Your Media. One Simple Destination.**
>
> An authorized multi-platform media retrieval, inspection, processing, conversion, and download management platform.

---

## 1. Project Overview

**MediaDock** is a full-stack web platform built to inspect, retrieve, process, convert, and manage user-owned and explicitly authorized media from a single clean workspace. Rather than functioning as an unauthorized scraper, MediaDock is engineered from the ground up around **platform compliance, creator attribution preservation, security hardening, and clean modular architecture**.

- **Backend (`mediadock-BE`)**: Go Fiber v2 modular monolith backed by PostgreSQL (GORM), Redis (caching, token revocation, rate limiting, and background job queueing), FFmpeg/FFprobe media processing, and Local / S3-compatible object storage.
- **Frontend (`mediadock-FE`)**: React 19, TypeScript, Vite, Tailwind CSS, TanStack Query, Zustand, React Hook Form + Zod, and Framer Motion with a dark-first SaaS interface and persisted light/dark theme switching.
- **Orchestration (`docker-compose.yml` & `Makefile`)**: Single-command container stack and unified developer workflow for PostgreSQL 16, Redis 7, the Go API + FFmpeg worker (`mediadock-api`), and the Nginx-served React web application (`mediadock-web`).

---

## 2. Content & Platform Compliance Model

MediaDock strictly distinguishes authorized integrations from unsupported or restricted platforms:

1. **Authorized Access Only**:
   - **Instagram** (`Available`): Uses the official Meta Instagram Graph API with per-request user OAuth Bearer tokens (`/me`, `/me/media`) to retrieve media owned or authorized by the caller.
   - **Direct User-Owned Media** (`Available`): Supports direct HTTP/HTTPS media URLs with explicit `owner_attested: true` confirmation alongside SSRF and MIME/magic-byte verification.
   - **YouTube & Facebook** (`Planned`): Kept as compliant architectural placeholders returning `501 Not Implemented` with guidance toward official data exports (e.g., Google Takeout / Meta Accounts Center) or direct file uploads into MediaDock's converter.
2. **Attribution Integrity**:
   - Processing and format conversion preserve source metadata (`-map_metadata 0`) and never remove third-party watermarks or copyright attribution.
   - MediaDock never stamps its own watermark onto downloaded or converted media.
3. **Zero Circumvention**:
   - No HTML scraping, credential harvesting, session-cookie theft, private-media bypassing, or DRM circumvention.

---

## 3. Repository Structure

```text
MediaDock/
├── mediadock-BE/              # Go Fiber modular monolith REST API & FFmpeg worker
│   ├── cmd/api/main.go        # Application bootstrap, migrations, worker pool, cleanup ticker
│   ├── docs/                  # OpenAPI 3.0 specification (served at GET /api/v1/openapi.yaml)
│   ├── internal/
│   │   ├── config/            # Environment configuration & validation
│   │   ├── server/            # Fiber app setup & /api/v1 route registration
│   │   ├── middleware/        # JWT/Cookie auth, CORS, Redis rate limit, slog request logging
│   │   ├── modules/           # auth, users, media, downloader, providers, processing, conversion, history, storage
│   │   ├── platform/          # instagram (Graph API), generic (owner-attested), youtube & facebook (planned)
│   │   ├── repository/        # GORM models and repository abstractions
│   │   ├── database/          # PostgreSQL connection & embedded SQL migrations
│   │   ├── cache/             # Redis wrapper
│   │   ├── queue/             # Concurrent Redis worker queue with retries & DLQ
│   │   └── utils/             # SSRF guard, magic-byte file validator, error/response envelopes
│   ├── migrations/            # Versioned SQL up/down schemas
│   ├── tests/                 # Security, provider, cookie auth, queue, and API integration tests
│   ├── Dockerfile             # Multi-stage Go + Alpine + FFmpeg runtime image
│   └── README.md
├── mediadock-FE/              # React + TypeScript + Vite SPA
│   ├── public/                # Brand SVG logo, favicon, sitemap, robots, placeholders
│   ├── src/
│   │   ├── components/        # Common UI primitives, forms, layout, media cards, navigation
│   │   ├── features/          # Media download, preview, history, conversion, auth hooks
│   │   ├── pages/             # Home, Downloader, History, MyMedia, Converters, Settings, Login, Register
│   │   ├── routes/            # React Router setup & ProtectedRoute guard
│   │   ├── services/          # Typed Axios services (cookie-based sessions + 401 refresh)
│   │   ├── store/             # Zustand state stores (auth, theme, toasts)
│   │   └── utils/             # Platform URL detection, byte/duration formatting
│   ├── Dockerfile             # Multi-stage Vite build + Nginx static server
│   ├── nginx.conf
│   └── README.md
├── docker-compose.yml         # PostgreSQL, Redis, mediadock-api, mediadock-web stack
├── Makefile                   # Unified build, test, lint, dev, and Docker workflows
├── .env.example               # Shared environment template
└── .gitignore
```

---

## 4. Core Capabilities & Architecture

### Backend Modules (`mediadock-BE`)
- **Authentication & Sessions**: Bcrypt password hashing, short-lived JWT access tokens + rotating refresh tokens backed by Redis revocation lists, supporting both `HttpOnly` secure cookies (`mediadock_access` / `mediadock_refresh`) and `Authorization: Bearer` headers.
- **Provider Registry & Platform Isolation**: Common `Provider` contract (`ValidateURL`, `VerifyAuthorization`, `GetMetadata`, `ListFormats`, `RetrieveMedia`) decouples core orchestration from platform APIs.
- **Async Download & FFmpeg Processing**: Redis `WorkerQueue` with bounded concurrency, per-job execution timeouts, exponential retry backoff, dead-letter queue (`mediadock:jobs:dlq`), and FFmpeg transcoding/audio extraction/compression/thumbnailing.
- **Direct Media Converter**: Multipart upload conversion supporting MP4, MOV, MKV, WebM, MP3, AAC, WAV, FLAC, and OGG with strict file size, duration, extension, and magic-byte validation.
- **Storage & Retention**: Local filesystem backend (with HMAC-SHA256 signed URLs and path-traversal protection) and S3-compatible object storage (`aws-sdk-go-v2` presigned URLs), paired with automatic temporary file expiration and 90-day configurable history retention.
- **Security Controls**: SSRF protection verifying DNS resolutions at TCP dial time and across HTTP redirects, blocking loopback, private networks (`RFC 1918` / `CGNAT`), and cloud metadata endpoints (`169.254.169.254`).

### Frontend Application (`mediadock-FE`)
- **Landing & Inspection Workspace**: Hero URL inspector with live platform detection, clipboard paste/clear actions, Zod validation, and available-vs-planned platform transparency cards.
- **Interactive Media Preview & Job Tracker**: Inspect metadata (title, thumbnail, duration, file size, authorization badge), select permitted output format/quality, preview playback, and monitor live polling job progress.
- **Media Library, History & Converters**: Paginated download history (`GET /api/v1/history`), saved inspected media (`GET /api/v1/media`), upload conversion studio (`POST /api/v1/conversions`), and persisted user profile/preferences (`PUT /api/v1/auth/me`, `PUT /api/v1/settings`).

---

## 5. API Endpoints Summary (`/api/v1`)

| Area | Endpoints |
|---|---|
| **System & Docs** | `GET /health`, `GET /ready`, `GET /openapi.yaml` |
| **Authentication** | `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout`, `GET /auth/me`, `PUT /auth/me` |
| **Settings** | `GET /settings`, `PUT /settings` |
| **Media Inspection** | `GET /media`, `POST /media/inspect`, `GET /media/:id`, `DELETE /media/:id` |
| **Downloads** | `GET /downloads`, `POST /downloads`, `GET /downloads/:id`, `GET /downloads/:id/status`, `GET /downloads/:id/file`, `DELETE /downloads/:id` |
| **Conversions** | `POST /conversions`, `GET /conversions/:id`, `GET /conversions/:id/status`, `GET /conversions/:id/file` |
| **History** | `GET /history`, `GET /history/:id`, `DELETE /history/:id` |

---

## 6. Quick Start

### Option A: Docker Compose (Full Stack)

```bash
cp .env.example .env
docker-compose up -d --build
```

Services started:
- **Web UI**: `http://localhost:5173`
- **REST API**: `http://localhost:8080/api/v1/health`
- **OpenAPI Spec**: `http://localhost:8080/api/v1/openapi.yaml`
- **PostgreSQL**: `localhost:5432`
- **Redis**: `localhost:6379`

### Option B: Local Development with `Makefile`

Prerequisites: **Go 1.24+**, **Node.js 20+**, **PostgreSQL 16+**, **Redis 7+**, and **FFmpeg**.

```bash
# View all commands
make help

# Initialize environment and install dependencies
make setup-env
make deps

# Start both backend (:8080) and frontend (:5173) together in one command
make run

# Or start backend API (terminal 1) and frontend Vite server (terminal 2) separately
make dev-be
make dev-fe
```

Common `Makefile` targets:
- `make verify` — runs `lint`, `vet`, all `BE` & `FE` tests, and builds both projects.
- `make test` — runs Go tests (`mediadock-BE`) and Vitest tests (`mediadock-FE`).
- `make build` — compiles `mediadock-BE/bin/mediadock-api` and `mediadock-FE/dist`.
- `make docker-up` / `make docker-down` / `make docker-logs` — manage the Docker Compose stack.

---

## 7. Automated Testing

- **Backend (`cd mediadock-BE && go test -timeout 30s -v ./...`)**: Exercises unit & integration suites using pure-Go SQLite (`github.com/glebarez/sqlite`), in-memory Redis (`miniredis`), an `httptest` mock Instagram Graph API server, and a no-op processor mock for FFmpeg.
- **Frontend (`cd mediadock-FE && npm test`)**: Exercises utility functions, form validation, media preview cards, processing status cards, route protection guards, and homepage platform states using Vitest and React Testing Library (`jsdom`).

---

## 8. Further Documentation

- Backend architecture, migrations, and environment details: [`mediadock-BE/README.md`](./mediadock-BE/README.md)
- Frontend structure and scripts: [`mediadock-FE/README.md`](./mediadock-FE/README.md)
- Machine-readable API contract: [`mediadock-BE/docs/openapi.yaml`](./mediadock-BE/docs/openapi.yaml)

