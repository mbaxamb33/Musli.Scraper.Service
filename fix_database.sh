#!/bin/bash

# fix_database.sh - Reset and properly initialize the database
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔧 Fixing Database Migration Issues${NC}"
echo "=================================="

# Step 1: Stop and clean up Docker containers
echo -e "\n${YELLOW}1. Cleaning up existing containers...${NC}"
make docker-down || true
docker volume rm musli_postgres_data musli_rabbitmq_data musli_redis_data 2>/dev/null || true
echo -e "${GREEN}✅ Containers stopped and volumes removed${NC}"

# Step 2: Start fresh containers
echo -e "\n${YELLOW}2. Starting fresh containers...${NC}"
make docker-up
echo -e "${GREEN}✅ Docker containers started${NC}"

# Step 3: Wait for PostgreSQL to be ready
echo -e "\n${YELLOW}3. Waiting for PostgreSQL to be ready...${NC}"
for i in {1..30}; do
    if docker-compose exec -T postgres pg_isready -U root -d postgres >/dev/null 2>&1; then
        echo -e "${GREEN}✅ PostgreSQL is ready${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e "${RED}❌ PostgreSQL failed to start${NC}"
        exit 1
    fi
done

# Step 4: Drop and recreate database completely
echo -e "\n${YELLOW}4. Recreating database from scratch...${NC}"
docker-compose exec -T postgres psql -U root -d postgres -c "DROP DATABASE IF EXISTS musli_scraper;" 2>/dev/null || true
docker-compose exec -T postgres psql -U root -d postgres -c "CREATE DATABASE musli_scraper;" 2>/dev/null
echo -e "${GREEN}✅ Database recreated${NC}"

# Step 5: Run migrations
echo -e "\n${YELLOW}5. Running migrations...${NC}"
if make migrateup; then
    echo -e "${GREEN}✅ Database migrations completed${NC}"
else
    echo -e "${RED}❌ Migration failed${NC}"
    echo "Let's try to fix the migration..."
    
    # Force reset migrations
    echo -e "${YELLOW}Forcing migration reset...${NC}"
    migrate -path db/migration/ -database "postgresql://root:secret@localhost:5433/musli_scraper?sslmode=disable" force 0 || true
    
    # Try again
    make migrateup
fi

# Step 6: Verify database setup
echo -e "\n${YELLOW}6. Verifying database setup...${NC}"
TABLE_COUNT=$(docker-compose exec -T postgres psql -U root -d musli_scraper -tAc "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null || echo "0")

if [ "$TABLE_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✅ Database setup verified - $TABLE_COUNT tables created${NC}"
    
    # Show table list
    echo -e "\n${BLUE}Tables created:${NC}"
    docker-compose exec -T postgres psql -U root -d musli_scraper -c "\dt" | grep -E "(scraping_jobs|job_metrics)" || echo "Tables exist but might have different names"
    
else
    echo -e "${RED}❌ Database setup failed${NC}"
    exit 1
fi

# Step 7: Test database connectivity
echo -e "\n${YELLOW}7. Testing database connectivity...${NC}"
TEST_RESULT=$(docker-compose exec -T postgres psql -U root -d musli_scraper -tAc "SELECT COUNT(*) FROM scraping_jobs;" 2>/dev/null || echo "ERROR")

if [ "$TEST_RESULT" = "ERROR" ]; then
    echo -e "${RED}❌ Cannot query scraping_jobs table${NC}"
    exit 1
else
    echo -e "${GREEN}✅ Database is working - scraping_jobs table has $TEST_RESULT rows${NC}"
fi

# Step 8: Check RabbitMQ
echo -e "\n${YELLOW}8. Checking RabbitMQ...${NC}"
sleep 5  # Give RabbitMQ time to start
if curl -s http://localhost:15672 >/dev/null; then
    echo -e "${GREEN}✅ RabbitMQ is accessible${NC}"
else
    echo -e "${YELLOW}⚠️  RabbitMQ might still be starting...${NC}"
fi

echo -e "\n${GREEN}🎉 Database setup completed successfully!${NC}"
echo -e "\n${BLUE}Next steps:${NC}"
echo "1. Start the service: make dev-run"
echo "2. Test the service: ./comprehensive_test.sh"
echo "3. Check RabbitMQ UI: http://localhost:15672 (guest/guest)"

echo -e "\n${BLUE}Database connection details:${NC}"
echo "Host: localhost:5433"
echo "Database: musli_scraper"
echo "User: root"
echo "Password: secret"