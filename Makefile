# Musli.Scraper.Service Makefile

# Database Configuration
DB_USER=root
DB_PASSWORD=secret
DB_HOST=localhost
DB_PORT=5433
DB_NAME=musli_scraper
CONTAINER_NAME=postgres_scraper

# Database URL for migrations and connections
DATABASE_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# PostgreSQL Docker Container (using different port to avoid conflicts)
postgres:
	docker run --name $(CONTAINER_NAME) -p $(DB_PORT):5432 \
		-e POSTGRES_USER=$(DB_USER) \
		-e POSTGRES_PASSWORD=$(DB_PASSWORD) \
		-d postgres:17-alpine

# Create database
createdb:
	docker exec -it $(CONTAINER_NAME) createdb --username=$(DB_USER) --owner=$(DB_USER) $(DB_NAME)

# Drop database
dropdb:
	docker exec -it $(CONTAINER_NAME) dropdb $(DB_NAME)

# Stop PostgreSQL container
stoppostgres:
	docker stop $(CONTAINER_NAME) || true
	docker rm $(CONTAINER_NAME) || true

# Remove PostgreSQL container
rmpostgres:
	docker stop $(CONTAINER_NAME) || true
	docker rm $(CONTAINER_NAME) || true

# Setup: Create container and database
setup: postgres
	@echo "Waiting for PostgreSQL to start..."
	@sleep 5
	@make createdb
	@echo "Database setup complete!"

# Migrate up
migrateup:
	migrate -path db/migration/ -database "$(DATABASE_URL)" -verbose up

# Migrate down
migratedown:
	migrate -path db/migration/ -database "$(DATABASE_URL)" -verbose down

# Create a new migration file
createmigration:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir db/migration/ -seq $$name

# Generate SQLC code
sqlc:
	sqlc generate

# Build the service
build:
	go build -o bin/scraper-service cmd/server/main.go

# Run the service
server:
	go run cmd/server/main.go

# Run with live reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Clean up
clean:
	rm -f bin/scraper-service
	rm -f coverage.out

# Docker build
docker-build:
	docker build -t musli-scraper-service -f docker/Dockerfile .

# Docker run
docker-run:
	docker run -p 8081:8081 --env-file .env musli-scraper-service

# Full setup for development
dev-setup: setup migrateup sqlc
	@echo "Development environment ready!"

# Reset database (careful!)
reset-db: dropdb createdb migrateup
	@echo "Database reset complete!"

.PHONY: postgres createdb dropdb stoppostgres rmpostgres setup migrateup migratedown createmigration sqlc build server dev test test-coverage fmt vet lint clean docker-build docker-run dev-setup reset-db