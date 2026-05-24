SHELL := /bin/sh

.PHONY: tidy fmt vet lint test build run migrate seed gqlgen docker-build docker-up docker-down

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet ./...

lint: fmt vet

test:
	go test ./... -race -count=1

build:
	go build -o bin/server ./cmd/server
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/seed ./cmd/seed

run:
	go run ./cmd/server

migrate:
	go run ./cmd/migrate up

seed:
	go run ./cmd/seed

gqlgen:
	go run github.com/99designs/gqlgen generate

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down
