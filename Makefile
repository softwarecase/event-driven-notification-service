.PHONY: build run-api run-worker test lint migrate-up migrate-down docker-up docker-down clean

# Build
build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

build-api:
	go build -o bin/api ./cmd/api

build-worker:
	go build -o bin/worker ./cmd/worker

# Run
run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

# Test
test:
	go test -v -race -count=1 ./...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Database migrations
MIGRATE=migrate -path internal/adapter/repository/postgres/migrations -database "$(DATABASE_URL)"

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

migrate-create:
	migrate create -ext sql -dir internal/adapter/repository/postgres/migrations -seq $(name)

# Docker
docker-up:
	docker-compose -f deployments/docker-compose.yml up -d

docker-down:
	docker-compose -f deployments/docker-compose.yml down

docker-build:
	docker-compose -f deployments/docker-compose.yml build

# Clean
clean:
	rm -rf bin/ coverage.out coverage.html
