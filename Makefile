.PHONY: deps docker-up docker-down migrate-up migrate-down run-gateway test lint clean

# ── Dependencies ──────────────────────────────────────────────
deps:
	go mod tidy

# ── Docker ────────────────────────────────────────────────────
docker-up:
	docker compose up -d
	@echo "Waiting for PostgreSQL to accept connections..."
	@until docker compose exec -T postgres pg_isready -U rideshare > /dev/null 2>&1; do \
		sleep 1; \
	done
	@echo "✅ All services ready."

docker-down:
	docker compose down

docker-clean:
	docker compose down -v

# ── Migrations ────────────────────────────────────────────────
DB_URL ?= postgres://rideshare:rideshare_secret@localhost:5433/rideshare?sslmode=disable

migrate-up:
	@echo "Running migrations..."
	@for f in $$(ls migrations/*.up.sql | sort); do \
		echo "  → $$f"; \
		psql "$(DB_URL)" -f $$f; \
	done

migrate-down:
	@echo "Rolling back migrations..."
	@for f in $$(ls migrations/*.down.sql | sort -r); do \
		echo "  → $$f"; \
		psql "$(DB_URL)" -f $$f; \
	done

# ── Run Services ──────────────────────────────────────────────
run-gateway:
	go run ./cmd/gateway

run-authsvc:
	go run ./cmd/authsvc

run-usersvc:
	go run ./cmd/usersvc

run-ridesvc:
	go run ./cmd/ridesvc

run-locationsvc:
	go run ./cmd/locationsvc

run-paymentsvc:
	go run ./cmd/paymentsvc

run-notificationsvc:
	go run ./cmd/notificationsvc

# ── Testing ───────────────────────────────────────────────────
test:
	go test ./... -v -race -count=1

test-cover:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# ── Linting ───────────────────────────────────────────────────
lint:
	golangci-lint run ./...

# ── Clean ─────────────────────────────────────────────────────
clean:
	go clean
	rm -f coverage.out coverage.html
