#!/bin/bash

# comprehensive_test.sh - Complete testing script for Musli Scraper Service
set -e

BASE_URL="http://localhost:8081"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🧪 Musli Scraper Service - Comprehensive Test${NC}"
echo "=============================================="

# Function to check if a service is running
check_service() {
    local service_name=$1
    local url=$2
    local max_attempts=${3:-30}
    
    echo -e "${YELLOW}Checking $service_name...${NC}"
    
    for i in $(seq 1 $max_attempts); do
        if curl -s "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ $service_name is ready${NC}"
            return 0
        fi
        echo -n "."
        sleep 1
    done
    
    echo -e "${RED}❌ $service_name is not ready after ${max_attempts}s${NC}"
    return 1
}

# Function to check Docker services
check_docker_services() {
    echo -e "${YELLOW}Checking Docker services...${NC}"
    
    # Check if docker-compose services are running
    if ! docker-compose ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting Docker services...${NC}"
        make docker-up
        sleep 10
    fi
    
    # Check PostgreSQL
    if ! docker-compose exec -T postgres pg_isready -U root -d musli_scraper; then
        echo -e "${RED}❌ PostgreSQL not ready${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ PostgreSQL is ready${NC}"
    
    # Check RabbitMQ
    if ! curl -s http://localhost:15672 > /dev/null; then
        echo -e "${RED}❌ RabbitMQ not ready${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ RabbitMQ is ready${NC}"
}

# Function to run database migrations
setup_database() {
    echo -e "${YELLOW}Setting up database...${NC}"
    
    if make migrateup 2>/dev/null; then
        echo -e "${GREEN}✅ Database migrations applied${NC}"
    else
        echo -e "${RED}❌ Database migration failed${NC}"
        return 1
    fi
}

# Function to start the service if not running
start_service() {
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${YELLOW}Starting scraper service...${NC}"
        
        # Start service in background
        make dev-run > service.log 2>&1 &
        SERVICE_PID=$!
        echo $SERVICE_PID > service.pid
        
        echo -e "${BLUE}Service started with PID: $SERVICE_PID${NC}"
        echo -e "${BLUE}Logs: tail -f service.log${NC}"
        
        # Wait for service to be ready
        check_service "Scraper Service" "$BASE_URL/health" 60
    else
        echo -e "${GREEN}✅ Scraper service is already running${NC}"
    fi
}

# Function to test basic endpoints
test_basic_endpoints() {
    echo -e "\n${YELLOW}🔍 Testing Basic Endpoints${NC}"
    
    # Health check
    if curl -s "$BASE_URL/health" | jq -e '.status == "ok"' > /dev/null; then
        echo -e "${GREEN}✅ Health check${NC}"
    else
        echo -e "${RED}❌ Health check failed${NC}"
        curl -s "$BASE_URL/health" | jq .
        return 1
    fi
    
    # Metrics
    if curl -s "$BASE_URL/api/metrics" | jq -e '.total_jobs >= 0' > /dev/null; then
        echo -e "${GREEN}✅ Metrics endpoint${NC}"
    else
        echo -e "${RED}❌ Metrics endpoint failed${NC}"
        curl -s "$BASE_URL/api/metrics"
        return 1
    fi
    
    # Worker status
    if curl -s "$BASE_URL/api/workers/status" | jq -e '.is_running == true' > /dev/null; then
        echo -e "${GREEN}✅ Worker status${NC}"
    else
        echo -e "${RED}❌ Worker status failed${NC}"
        curl -s "$BASE_URL/api/workers/status" | jq .
        return 1
    fi
}

# Function to test job creation and processing
test_job_processing() {
    echo -e "\n${YELLOW}🔍 Testing Job Processing${NC}"
    
    # Test 1: Simple HTML page
    echo -e "${BLUE}Creating simple job...${NC}"
    
    RESPONSE=$(curl -s -X POST "$BASE_URL/api/jobs" \
        -H "Content-Type: application/json" \
        -d '{
            "url": "https://httpbin.org/html",
            "options": {
                "wait_for_js": false,
                "timeout": "30s"
            }
        }')
    
    if echo "$RESPONSE" | jq -e '.id' > /dev/null; then
        JOB_ID=$(echo "$RESPONSE" | jq -r '.id')
        echo -e "${GREEN}✅ Simple job created: $JOB_ID${NC}"
        
        # Wait for job completion
        echo -e "${BLUE}Waiting for job completion...${NC}"
        for i in {1..30}; do
            STATUS=$(curl -s "$BASE_URL/api/jobs/$JOB_ID" | jq -r '.status')
            echo -n "[$i/30] Status: $STATUS "
            
            case $STATUS in
                "completed")
                    echo -e "${GREEN}✅ Job completed successfully${NC}"
                    # Show results summary
                    curl -s "$BASE_URL/api/jobs/$JOB_ID" | jq '{
                        status: .status,
                        modules_extracted: (.results.processing_stats.modules_extracted // 0),
                        processing_time: .results.processing_stats.processing_time
                    }'
                    break
                    ;;
                "failed")
                    echo -e "${RED}❌ Job failed${NC}"
                    curl -s "$BASE_URL/api/jobs/$JOB_ID" | jq '.error'
                    return 1
                    ;;
                "processing"|"pending")
                    echo "- waiting..."
                    sleep 2
                    ;;
                *)
                    echo -e "${RED}❌ Unknown status: $STATUS${NC}"
                    return 1
                    ;;
            esac
        done
    else
        echo -e "${RED}❌ Failed to create simple job${NC}"
        echo "$RESPONSE" | jq .
        return 1
    fi
    
    # Test 2: Job with options
    echo -e "\n${BLUE}Creating job with options...${NC}"
    
    RESPONSE2=$(curl -s -X POST "$BASE_URL/api/jobs" \
        -H "Content-Type: application/json" \
        -d '{
            "url": "https://example.com",
            "priority": 7,
            "options": {
                "wait_for_js": true,
                "timeout": "20s",
                "scroll_to_bottom": false
            },
            "metadata": {
                "test": "comprehensive_test",
                "timestamp": "'$(date -Iseconds)'"
            }
        }')
    
    if echo "$RESPONSE2" | jq -e '.id' > /dev/null; then
        JOB_ID2=$(echo "$RESPONSE2" | jq -r '.id')
        echo -e "${GREEN}✅ Complex job created: $JOB_ID2${NC}"
    else
        echo -e "${RED}❌ Failed to create complex job${NC}"
        echo "$RESPONSE2" | jq .
    fi
}

# Function to test job listing and metrics
test_job_management() {
    echo -e "\n${YELLOW}🔍 Testing Job Management${NC}"
    
    # List jobs
    JOBS_RESPONSE=$(curl -s "$BASE_URL/api/jobs?limit=5")
    TOTAL_JOBS=$(echo "$JOBS_RESPONSE" | jq -r '.total_count')
    
    if [ "$TOTAL_JOBS" -ge 0 ] 2>/dev/null; then
        echo -e "${GREEN}✅ Job listing works - Total jobs: $TOTAL_JOBS${NC}"
    else
        echo -e "${RED}❌ Job listing failed${NC}"
        echo "$JOBS_RESPONSE" | jq .
    fi
    
    # Current metrics
    echo -e "${BLUE}Current system metrics:${NC}"
    curl -s "$BASE_URL/api/metrics" | jq '{
        total_jobs: .total_jobs,
        pending_jobs: .pending_jobs,
        processing_jobs: .processing_jobs,
        completed_jobs: .completed_jobs,
        failed_jobs: .failed_jobs
    }'
}

# Function to test service under load (optional)
test_load() {
    echo -e "\n${YELLOW}🔍 Load Testing (Creating multiple jobs)${NC}"
    
    for i in {1..5}; do
        curl -s -X POST "$BASE_URL/api/jobs" \
            -H "Content-Type: application/json" \
            -d "{
                \"url\": \"https://httpbin.org/delay/1\",
                \"options\": {
                    \"wait_for_js\": false,
                    \"timeout\": \"10s\"
                },
                \"metadata\": {
                    \"load_test\": true,
                    \"batch\": $i
                }
            }" > /dev/null &
    done
    
    wait
    echo -e "${GREEN}✅ Created 5 load test jobs${NC}"
    
    # Wait a moment and check metrics
    sleep 5
    echo -e "${BLUE}Metrics after load test:${NC}"
    curl -s "$BASE_URL/api/metrics" | jq '{
        total_jobs: .total_jobs,
        pending_jobs: .pending_jobs,
        processing_jobs: .processing_jobs
    }'
}

# Function to cleanup
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleanup${NC}"
    
    # Stop service if we started it
    if [ -f service.pid ]; then
        PID=$(cat service.pid)
        if kill $PID 2>/dev/null; then
            echo -e "${GREEN}✅ Service stopped${NC}"
        fi
        rm -f service.pid service.log
    fi
}

# Trap cleanup on exit
trap cleanup EXIT

# Main test flow
main() {
    echo -e "${BLUE}Starting comprehensive test...${NC}"
    
    # Check prerequisites
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}❌ jq is required but not installed${NC}"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}❌ docker-compose is required but not installed${NC}"
        exit 1
    fi
    
    # Step 1: Check and start infrastructure
    check_docker_services || exit 1
    
    # Step 2: Setup database
    setup_database || exit 1
    
    # Step 3: Start the service
    start_service || exit 1
    
    # Step 4: Run tests
    test_basic_endpoints || exit 1
    test_job_processing || exit 1
    test_job_management || exit 1
    
    # Optional load test
    if [ "$1" = "--load" ]; then
        test_load
    fi
    
    echo -e "\n${GREEN}🎉 All tests completed successfully!${NC}"
    
    # Show final summary
    echo -e "\n${BLUE}📊 Final Summary:${NC}"
    curl -s "$BASE_URL/api/metrics" | jq .
    
    echo -e "\n${BLUE}💡 Useful commands:${NC}"
    echo "  - View logs: tail -f service.log"
    echo "  - Check workers: curl -s $BASE_URL/api/workers/status | jq ."
    echo "  - List jobs: curl -s $BASE_URL/api/jobs | jq ."
    echo "  - RabbitMQ UI: http://localhost:15672 (guest/guest)"
}

# Run main function
main "$@"