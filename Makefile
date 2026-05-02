.PHONY: up down build run test lint fmt logs

up:
	docker compose up --build -d

down:
	docker compose down

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o snip ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./... -v -race

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

logs:
	docker compose logs -f api
