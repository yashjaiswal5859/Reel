.PHONY: build test run-api run-worker up down tidy

tidy:
	go mod tidy

build:
	go build -o bin/api.exe ./cmd/api
	go build -o bin/worker.exe ./cmd/worker

test:
	go test -v -race ./...

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

up:
	docker compose up -d

down:
	docker compose down
