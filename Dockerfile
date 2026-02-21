# ── Stage 1: Build ────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

# Which microservice to build (e.g., gateway, auth, user, ride, location, payment)
ARG SERVICE_NAME=gateway

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/app ./cmd/${SERVICE_NAME}

# ── Stage 2: Runtime ──────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Non-root user
RUN addgroup -S app && adduser -S app -G app
USER app

WORKDIR /app
COPY --from=builder /bin/app /app/server

EXPOSE 8080 50051

ENTRYPOINT ["/app/server"]
