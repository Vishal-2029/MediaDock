# Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mediadock-api ./cmd/api

# Stage 2: Minimal production runtime with FFmpeg
FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ffmpeg ca-certificates tzdata \
    && addgroup -S mediadock \
    && adduser -S -G mediadock mediadock \
    && mkdir -p /app/storage_data \
    && chown -R mediadock:mediadock /app

COPY --from=builder /out/mediadock-api /app/mediadock-api

USER mediadock

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/v1/health || exit 1

ENTRYPOINT ["/app/mediadock-api"]

