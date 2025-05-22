#!/bin/bash
# Install required dependencies for Musli.Scraper.Service

echo "Installing Go dependencies..."

# Core dependencies
go get github.com/gin-gonic/gin
go get github.com/go-rod/rod
go get github.com/lib/pq
go get github.com/joho/godotenv
go get github.com/caarlos0/env/v6
go get github.com/google/uuid
go get go.uber.org/zap

# SQLC and database tools
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/stdlib

# Development tools
echo "Installing development tools..."

# Install golang-migrate
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Install SQLC
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Install air for live reload (optional)
go install github.com/cosmtrek/air@latest

# Install golangci-lint for linting (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "Dependencies installed successfully!"
echo ""
echo "Make sure the following tools are in your PATH:"
echo "- migrate (for database migrations)"
echo "- sqlc (for generating Go code from SQL)"
echo ""
echo "You can also install them manually:"
echo "- migrate: https://github.com/golang-migrate/migrate/releases"
echo "- sqlc: https://github.com/sqlc-dev/sqlc/releases"