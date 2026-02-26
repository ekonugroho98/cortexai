# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Cache dependency layer before copying full source
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Build binary with stripped debug info for minimal size
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o cortexai ./cmd/cortexai

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19

# Security: non-root user
RUN addgroup -g 1001 cortexai && \
    adduser -D -u 1001 -G cortexai cortexai

WORKDIR /app

# FIX #3: ca-certificates for GCP/Anthropic HTTPS, wget for HEALTHCHECK
RUN apk add --no-cache ca-certificates tzdata wget

# Copy binary only — results in ~15MB image
COPY --from=builder /build/cortexai /app/cortexai

USER cortexai

EXPOSE 8000

# FIX #3: wget is now installed so healthcheck works
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:8000/health || exit 1

ENTRYPOINT ["/app/cortexai"]
