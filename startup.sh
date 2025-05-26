#!/bin/bash
# startup.sh - Complete startup script for Musli Scraper Service

set -e

echo "🚀 Starting Musli Scraper Service Setup"
echo "======================================"

# 1. Add missing Go dependency
echo "📦 Installing missing RabbitMQ dependency..."
go get github.com/rabbitmq/amqp091-go@v1.10.0
go mod tidy

# 2. Start infrastructure services
echo "🐳 Starting Docker services (PostgreSQL, RabbitMQ)..."
make docker-up

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 15

# 3. Run database migrations
echo "🗄️  Running database migrations..."
make migrateup

# 4. Generate SQLC code (if needed)
echo "🔧 Generating SQLC code..."
make sqlc

# 5. Build the service
echo "🏗️  Building the service..."
make build

# 6. Start the service
echo "✅ Starting the scraper service..."
echo ""
echo "Service will be available at:"
echo "  - API: http://localhost:8081"
echo "  - RabbitMQ UI: http://localhost:15672 (guest/guest)"
echo "  - Health: http://localhost:8081/health"
echo ""

# Run the service
./bin/scraper-service