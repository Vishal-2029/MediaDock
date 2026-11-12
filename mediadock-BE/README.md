# MediaDock Backend

Production-oriented Go Fiber modular monolith backend for **MediaDock** — an authorized multi-platform media retrieval, processing, conversion, and download management platform.

---

## 1. Architecture Overview

MediaDock uses a **modular monolith** architecture with strict separation between HTTP handlers, business services, persistence repositories, platform provider integrations, and background workers:

- **HTTP Layer (`internal/server`, `internal/middleware`, `*/handler.go`)**: Go Fiber v2 REST API under `/api/v1` with CORS, Redis-backed rate limiting, JWT Bearer authentication, request validation, structured JSON logging (`slog`), and sanitized error responses.
- **Business Services (`internal/modules/*`)**:
  - `auth`: Registration, login, JWT access + refresh token rotation, Redis-backed token revocation on logout.
  - `users`: User settings management (`theme`, `default_format`).
  - `providers`: Common `Provider` contract and URL-based `Registry`.
  - `media`: URL inspection, provider resolution, authorization verification, metadata persistence.
  - `downloader`: Async download job orchestration, progress tracking, secure file delivery, expiration cleanup.
  - `processing`: Safe FFmpeg/FFprobe wrapper (MP4/MOV/MKV/WebM conversion, MP3/AAC/WAV/FLAC/OGG audio extraction, resolution scaling, compression, thumbnail generation). Preserves original creator attribution (`-map_metadata 0`).
  - `conversion`: Multipart user-uploaded media conversion with magic-byte & size/duration limits.
  - `history`: Download audit history, paginated listing, and retention policy cleanup.
  - `storage`: Local filesystem storage (with HMAC signed URLs and path-traversal protection) and S3-compatible production storage (`aws-sdk-go-v2` with presigned URLs).
- **Platform Integrations (`internal/platform/*`)**:
  - `instagram`: Official Meta Instagram Graph API integration (`OAuth` Bearer token verification + `/me/media` lookup) plus optional official `/instagram_oembed` preview support when `INSTAGRAM_APP_ID` and `INSTAGRAM_APP_SECRET` are configured. No HTML scraping or cookie harvesting.
  - `external`: Optional commercially licensed third-party media resolution API adapter configured via `EXTERNAL_MEDIA_API_BASE` and `EXTERNAL_MEDIA_API_KEY` (with SSRF + size checks).
  - `generic`: Authorized direct media URLs with explicit `owner_attested=true` confirmation and strict SSRF + MIME checks.
  - `youtube`, `facebook`: Future-ready compliant stubs returning `501 Not Implemented` with official export/upload alternatives.
- **Background Worker (`internal/queue`)**: Redis-backed job queue (`RPUSH`/`BLPOP`) with configurable concurrency, per-job timeouts, retry backoff, dead-letter queue (`mediadock:jobs:dlq`), and periodic expiration/retention cleanup.

---

## 2. Folder Structure

```text
mediadock-BE/
├── cmd/api/main.go
├── internal/
│   ├── config/
│   ├── server/
│   ├── middleware/
│   ├── modules/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── media/
│   │   ├── downloader/
│   │   ├── providers/
│   │   ├── processing/
│   │   ├── conversion/
│   │   ├── history/
│   │   └── storage/
│   ├── platform/
│   │   ├── instagram/
│   │   ├── youtube/
│   │   ├── facebook/
│   │   └── generic/
│   ├── repository/
│   ├── database/
│   ├── cache/
│   ├── queue/
│   └── utils/
├── migrations/
├── tests/
├── go.mod
└── Dockerfile
```

---

## 3. Key Dependencies & Rationale

| Dependency | Purpose |
|---|---|
| `github.com/gofiber/fiber/v2` | High-performance Express-style HTTP framework and routing |
| `gorm.io/gorm` + `gorm.io/driver/postgres` | Type-safe PostgreSQL ORM, transactions, and pooling |
| `github.com/redis/go-redis/v9` | Redis client for JWT revocation, rate limiting, and job queueing |
| `github.com/golang-jwt/jwt/v5` | Access and refresh JWT signing & verification |
| `golang.org/x/crypto/bcrypt` | Adaptive password hashing |
| `github.com/go-playground/validator/v10` | Declarative struct request validation |
| `github.com/google/uuid` | UUID v4 identifiers across entities and jobs |
| `github.com/aws/aws-sdk-go-v2` | S3-compatible object storage and presigned download URLs |
| `github.com/joho/godotenv` | `.env` file loading in local development |
| `github.com/alicebob/miniredis/v2` + `github.com/glebarez/sqlite` | Pure-Go in-memory Redis and SQLite for fast automated tests |

---

## 4. Content & Platform Compliance

MediaDock strictly enforces authorized access:
1. Only supports media **owned by the user** or for which the user holds **explicit API/owner authorization**.
2. Never removes creator attribution, copyright metadata, or third-party watermarks, and never adds a MediaDock watermark.
3. Never performs HTML scraping, session-cookie harvesting, private-media bypassing, or DRM circumvention.
4. When a platform (e.g. YouTube or Facebook, or unauthenticated Instagram) does not permit general-purpose downloading, the API returns a structured compliance error explaining the limitation and suggesting official alternatives (Meta/Google Takeout exports or direct file upload to `/api/v1/conversions`).

---

## 5. Security Controls

- **SSRF Protection (`internal/utils/ssrf.go`)**: Enforces `http`/`https`, rejects embedded URL credentials, blocks `localhost`, private CIDR ranges (`10/8`, `172.16/12`, `192.168/16`, `100.64/10`, `fc00::/7`), link-local / cloud metadata (`169.254.169.254`, `metadata.google.internal`), validates DNS IPs at TCP dial time (`net.Dialer.Control` preventing DNS rebinding), and re-validates every HTTP redirect hop.
- **File Validation (`internal/utils/files.go`)**: Verifies extension allowlist, size limits (`MAX_FILE_SIZE`), and magic-byte signatures (ISO BMFF `ftyp`, EBML, RIFF, ID3, FLAC, OGG, JPEG, PNG) to reject disguised HTML/scripts.
- **Path Safety**: `SafeJoinPath` and randomized temp filenames prevent directory traversal.
- **Auth & Rate Limiting**: Short-lived access JWTs, rotated refresh tokens stored by `jti` in Redis, Redis revocation checks on logout, and per-IP fixed-window rate limiting.

---

## 6. REST API Endpoints (`/api/v1`)

### Health
- `GET /api/v1/health` — Liveness probe (`200 OK`)
- `GET /api/v1/ready` — Readiness probe checking PostgreSQL and Redis (`200 OK` / `503 Service Unavailable`)

### Authentication
- `POST /api/v1/auth/register` — Register user + default settings (`201 Created`)
- `POST /api/v1/auth/login` — Authenticate and receive JWT pair (`200 OK`)
- `POST /api/v1/auth/refresh` — Rotate refresh token and receive new JWT pair (`200 OK`)
- `POST /api/v1/auth/logout` — Revoke access & refresh tokens (`200 OK`, Bearer auth)
- `GET /api/v1/auth/me` — Current user profile (`200 OK`, Bearer auth)

### Media Inspection
- `POST /api/v1/media/inspect` — Validate URL, verify authorization (pass `access_token` or `X-Platform-Access-Token` for Instagram Graph API, or `owner_attested: true` for direct media URLs), inspect metadata and formats (`200 OK`)
- `GET /api/v1/media/:id` — Retrieve inspected media item (`200 OK`)

### Downloads
- `POST /api/v1/downloads` — Queue authorized download + optional FFmpeg format/quality processing (`202 Accepted`)
- `GET /api/v1/downloads/:id` — Get download job details (`200 OK`)
- `GET /api/v1/downloads/:id/status` — Poll job status (`pending|processing|completed|failed|expired`) and `progress` (`0..100`)
- `GET /api/v1/downloads/:id/file` — Stream completed file (or S3 presigned URL) and log history (`200 OK`, `410 Gone` if expired)
- `DELETE /api/v1/downloads/:id` — Delete job and stored artifact (`200 OK`)

### Conversions
- `POST /api/v1/conversions` — Multipart file upload (`file`, `target_format`, `target_resolution`, `compress`, `extract_audio`) queued for FFmpeg processing (`202 Accepted`)
- `GET /api/v1/conversions/:id` — Get conversion job details (`200 OK`)
- `GET /api/v1/conversions/:id/status` — Poll conversion status & progress (`200 OK`)
- `GET /api/v1/conversions/:id/file` — Stream converted output file or S3 presigned URL (`200 OK`)
- `GET /api/v1/conversions/:id/thumbnail` — Stream generated JPEG thumbnail for video conversions (`200 OK`)

### History & Settings
- `GET /api/v1/history?page=1&per_page=20` — Paginated download history (`200 OK`)
- `GET /api/v1/history/:id` — History entry details (`200 OK`)
- `DELETE /api/v1/history/:id` — Delete history entry (`200 OK`)
- `GET /api/v1/settings` — Get user settings (`200 OK`)
- `PUT /api/v1/settings` — Update `theme` (`light|dark|system`) and `default_format` (`200 OK`)

---

## 7. Running Locally & With Docker

### Docker Compose (Recommended)
1. Copy `.env.example` to `.env` and update secrets (`JWT_SECRET`, database password).
2. Start PostgreSQL, Redis, and the MediaDock API (with FFmpeg included):
   ```bash
   docker compose up --build -d
   ```
3. Verify readiness:
   ```bash
   curl http://localhost:8080/api/v1/ready
   ```

### Local Go Development
```bash
cd mediadock-BE
go mod download
go run ./cmd/api
```

### Automated Tests
```bash
cd mediadock-BE
go test -v ./...
```

