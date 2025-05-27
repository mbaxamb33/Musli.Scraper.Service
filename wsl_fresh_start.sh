#!/bin/bash
# wsl_fresh_start.sh - Fresh start specifically for WSL/Ubuntu on Windows

set -e

echo "🚀 WSL/Ubuntu Fresh Start - Musli Scraper Service"
echo "================================================="

# 1. Kill everything first
echo "🔪 Killing all related processes..."
pkill -f "go run" 2>/dev/null || true
pkill -f "scraper-service" 2>/dev/null || true
pkill -f postgres 2>/dev/null || true
pkill -f migrate 2>/dev/null || true

# 2. Clean Docker completely
echo "🐳 Cleaning Docker environment..."
docker stop $(docker ps -aq) 2>/dev/null || true
docker rm -f $(docker ps -aq) 2>/dev/null || true
docker volume rm $(docker volume ls -q) 2>/dev/null || true
docker network prune -f 2>/dev/null || true
docker system prune -af --volumes 2>/dev/null || true

# 3. Restart Docker service (WSL specific)
echo "🔄 Restarting Docker service..."
sudo service docker stop
sleep 2
sudo service docker start
sleep 3

# 4. Clean project files
echo "📁 Cleaning project files..."
rm -rf db/sqlc/*.go 2>/dev/null || true
mkdir -p db/sqlc
rm -rf bin/ 2>/dev/null || true

# 5. Create SQLC config
echo "📝 Creating sqlc.yaml..."
cat > sqlc.yaml << 'EOF'
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/query/"
    schema: "db/migration/"
    gen:
      go:
        package: "db"
        out: "db/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_db_tags: true
        emit_prepared_queries: false
        emit_interface: true
        emit_exact_table_names: true
        emit_empty_slices: true
        emit_exported_queries: true
        emit_result_struct_pointers: false
        emit_params_struct_pointers: false
        emit_methods_with_db_argument: false
        emit_enum_valid_method: true
        emit_all_enum_values: true
        overrides:
          - db_type: "job_status"
            go_type:
              type: "JobStatus"
EOF

# 6. Create a custom docker-compose for fresh start
echo "📝 Creating temporary docker-compose..."
cat > docker-compose.fresh.yaml << 'EOF'
version: '3.8'

services:
  postgres-fresh:
    image: postgres:17-alpine
    container_name: musli_postgres_fresh
    ports:
      - "5433:5432"
    environment:
      POSTGRES_USER: root
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: musli_scraper
      POSTGRES_INITDB_ARGS: "--encoding=UTF-8"
    volumes:
      - musli_postgres_fresh:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U root -d musli_scraper"]
      interval: 5s
      timeout: 5s
      retries: 10

  rabbitmq-fresh:
    image: rabbitmq:3-management
    container_name: musli_rabbitmq_fresh
    ports:
      - "5672:5672"
      - "15672:15672"
    environment:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    volumes:
      - musli_rabbitmq_fresh:/var/lib/rabbitmq

  redis-fresh:
    image: redis:7-alpine
    container_name: musli_redis_fresh
    ports:
      - "6379:6379"
    volumes:
      - musli_redis_fresh:/data

volumes:
  musli_postgres_fresh:
  musli_rabbitmq_fresh:
  musli_redis_fresh:
EOF

# 7. Start services with fresh config
echo "🐳 Starting fresh services..."
docker-compose -f docker-compose.fresh.yaml up -d

# 8. Wait for services
echo "⏳ Waiting for services to be ready..."
for i in {1..30}; do
    if docker exec musli_postgres_fresh pg_isready -U root -d musli_scraper &>/dev/null; then
        echo "✅ PostgreSQL is ready!"
        break
    fi
    echo "Waiting for PostgreSQL... ($i/30)"
    sleep 2
done

# 9. Run migrations
echo "🗄️  Running migrations..."
DATABASE_URL="postgresql://root:secret@localhost:5433/musli_scraper?sslmode=disable"
migrate -path db/migration/ -database "$DATABASE_URL" -verbose up

# 10. Generate SQLC
echo "🔧 Generating SQLC code..."
sqlc generate

# 11. Create JobStatus type
echo "📝 Creating JobStatus type..."
cat > db/sqlc/job_status_type.go << 'EOF'
package db

import (
	"database/sql/driver"
	"fmt"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCanceled   JobStatus = "canceled"
)

func (e *JobStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = JobStatus(s)
	case string:
		*e = JobStatus(s)
	default:
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type %T", src, e)
	}
	return nil
}

func (e JobStatus) Value() (driver.Value, error) {
	return string(e), nil
}

func (e JobStatus) Valid() bool {
	switch e {
	case JobStatusPending, JobStatusProcessing, JobStatusCompleted, JobStatusFailed, JobStatusCanceled:
		return true
	}
	return false
}

func (e JobStatus) String() string {
	return string(e)
}
EOF

# 12. Fix generated files
echo "📝 Fixing generated code..."
find db/sqlc -name "*.go" -type f -exec sed -i 's/Status\s\+interface{}/Status JobStatus/g' {} \;

# 13. Build
echo "🏗️  Building the service..."
go build -o bin/scraper-service cmd/server/main.go

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build successful!"
    echo ""
    echo "Services are running with fresh containers:"
    echo "  - PostgreSQL: localhost:5433"
    echo "  - RabbitMQ: localhost:5672 (UI at localhost:15672)"
    echo "  - Redis: localhost:6379"
    echo ""
    echo "Run the service with:"
    echo "  ./bin/scraper-service"
    echo ""
    echo "To switch back to normal docker-compose later:"
    echo "  docker-compose -f docker-compose.fresh.yaml down"
    echo "  docker-compose up -d"
else
    echo "❌ Build failed"
fi