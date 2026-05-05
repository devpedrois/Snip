.PHONY: up down build run test test-race test-integration coverage lint fmt logs migrate-up migrate-down

up:
	docker compose up --build -d

down:
	docker compose down

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o snip ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./... -v

test-race:
	CGO_ENABLED=1 go test ./... -v -race

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

logs:
	docker compose logs -f api

test-integration:
	go test ./tests/integration/... -v -tags=integration

coverage:
	go test ./internal/... -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML report: coverage.html"

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down
