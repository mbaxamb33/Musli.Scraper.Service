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



build:
	go build -o bin/scraper-service cmd/server/main.go

run: build
	./bin/scraper-service

# Development
dev-run:
	go run cmd/server/main.go

# Install missing dependencies
deps:
	go mod tidy
	go get github.com/jackc/pgx/v5
	go get github.com/jackc/pgx/v5/pgxpool

# Create directory structure
create-dirs:
	mkdir -p internal/handlers
	mkdir -p internal/services  
	mkdir -p internal/worker

# Full development setup including new structure
dev-setup-full: setup migrateup sqlc create-dirs deps
	@echo "Full development environment ready!"



# Start all services (infrastructure only)
docker-up:
	docker-compose up -d postgres rabbitmq redis

# Start all services including the scraper service
docker-up-full:
	docker-compose --profile full-stack up -d

# Stop all services
docker-down:
	docker-compose down

# Stop and remove volumes (CAUTION: This will delete all data)
docker-down-clean:
	docker-compose down -v

# View logs
docker-logs:
	docker-compose logs -f

# View logs for specific service
docker-logs-postgres:
	docker-compose logs -f postgres

docker-logs-rabbitmq:
	docker-compose logs -f rabbitmq

docker-logs-scraper:
	docker-compose logs -f scraper-service

# Build the scraper service image
docker-build:
	docker-compose build scraper-service

# Clean up Docker resources
docker-clean:
	docker-compose down -v --rmi all --remove-orphans
	docker system prune -f

# Check service health
docker-health:
	docker-compose ps

# Development setup with Docker
docker-dev-setup: docker-up
	@echo "Waiting for services to be ready..."
	@sleep 10
	@make migrateup
	@echo "Development environment ready!"

# Production deployment
docker-deploy:
	docker-compose --profile full-stack up -d --build

# Backup database
docker-backup-db:
	docker-compose exec postgres pg_dump -U root musli_scraper > backup_$(shell date +%Y%m%d_%H%M%S).sql

# Restore database (usage: make docker-restore-db BACKUP_FILE=backup.sql)
docker-restore-db:
	docker-compose exec -T postgres psql -U root -d musli_scraper < $(BACKUP_FILE)

# Monitor resource usage
docker-stats:
	docker stats $(shell docker-compose ps -q)

# Access RabbitMQ Management UI
rabbitmq-ui:
	@echo "RabbitMQ Management UI: http://localhost:15672"
	@echo "Username: guest, Password: guest"

# Access services for debugging
docker-shell-postgres:
	docker-compose exec postgres psql -U root -d musli_scraper

docker-shell-rabbitmq:
	docker-compose exec rabbitmq bash

docker-shell-scraper:
	docker-compose exec scraper-service sh

# Quick development start
dev-start: docker-up
	@echo "Starting development environment..."
	@sleep 5
	@make migrateup
	@echo "Services started. Running scraper service locally..."
	@go run cmd/server/main.go

# Quick development stop
dev-stop: docker-down
	@echo "Development environment stopped"


.PHONY: docker-up docker-down docker-logs docker-clean docker-build postgres createdb dropdb stoppostgres rmpostgres setup migrateup migratedown createmigration sqlc build server dev test test-coverage fmt vet lint clean docker-build docker-run dev-setup reset-db build run dev-run deps create-dirs dev-setup-full