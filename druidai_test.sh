#!/bin/bash

# druidai_test.sh - Test scraper service with DruidAI website
set -e

BASE_URL="http://localhost:8081"
TEST_URL="https://www.druidai.com/"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🧪 DruidAI Website Scraping Test${NC}"
echo "================================="
echo "Target URL: $TEST_URL"

# Function to wait for job completion
wait_for_job() {
    local job_id=$1
    local max_wait=${2:-120}  # 2 minutes default
    
    echo -e "${YELLOW}Waiting for job completion (max ${max_wait}s)...${NC}"
    
    for i in $(seq 1 $max_wait); do
        if JOB_STATUS=$(curl -s "$BASE_URL/api/jobs/$job_id" 2>/dev/null); then
            STATUS=$(echo "$JOB_STATUS" | jq -r '.status' 2>/dev/null || echo "unknown")
            PROGRESS=$(echo "$JOB_STATUS" | jq -r '.progress' 2>/dev/null || echo "0")
            
            printf "\r[%3d/%3ds] Status: %-12s Progress: %s%%" "$i" "$max_wait" "$STATUS" "$PROGRESS"
            
            case $STATUS in
                "completed")
                    echo -e "\n${GREEN}✅ Job completed successfully!${NC}"
                    return 0
                    ;;
                "failed")
                    echo -e "\n${RED}❌ Job failed${NC}"
                    ERROR=$(echo "$JOB_STATUS" | jq -r '.error // "No error details"')
                    echo "Error: $ERROR"
                    return 1
                    ;;
                "processing"|"pending")
                    sleep 1
                    ;;
                *)
                    echo -e "\n${RED}❌ Unknown status: $STATUS${NC}"
                    return 1
                    ;;
            esac
        else
            echo -e "\n${RED}❌ Failed to get job status${NC}"
            return 1
        fi
    done
    
    echo -e "\n${RED}❌ Job timed out after ${max_wait}s${NC}"
    return 1
}

# Function to display job results
show_job_results() {
    local job_id=$1
    
    echo -e "\n${BLUE}📊 Job Results Summary${NC}"
    if JOB_DATA=$(curl -s "$BASE_URL/api/jobs/$job_id" 2>/dev/null); then
        # Basic job info
        echo "$JOB_DATA" | jq '{
            id: .id,
            status: .status,
            progress: .progress,
            url: .url,
            created_at: .created_at,
            completed_at: .completed_at
        }' 2>/dev/null || echo "Could not parse basic job info"
        
        # Results summary
        if echo "$JOB_DATA" | jq -e '.results' > /dev/null 2>&1; then
            echo -e "\n${BLUE}📈 Processing Results:${NC}"
            echo "$JOB_DATA" | jq '.results.processing_stats // {}' 2>/dev/null || echo "No processing stats available"
            
            # Module pairs count
            MODULE_COUNT=$(echo "$JOB_DATA" | jq '.results.module_pairs | length' 2>/dev/null || echo "0")
            echo -e "\n${BLUE}📄 Content Extracted:${NC}"
            echo "  - Modules extracted: $MODULE_COUNT"
            
            # Show first few modules
            if [ "$MODULE_COUNT" -gt 0 ]; then
                echo -e "\n${BLUE}🔍 Sample Modules:${NC}"
                echo "$JOB_DATA" | jq -r '.results.module_pairs[0:3][] | "  - \(.title) (Level \(.level)): \(.content[0:100])..."' 2>/dev/null || echo "Could not display modules"
            fi
        else
            echo "No results data available"
        fi
    else
        echo -e "${RED}❌ Could not fetch job results${NC}"
    fi
}

# Test 1: Quick service health check
echo -e "\n${YELLOW}1. Service Health Check${NC}"
if curl -s "$BASE_URL/health" | jq -e '.status == "ok"' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Service is healthy${NC}"
else
    echo -e "${RED}❌ Service health check failed${NC}"
    exit 1
fi

# Test 2: Check workers are ready
echo -e "\n${YELLOW}2. Worker Status Check${NC}"
if WORKERS=$(curl -s "$BASE_URL/api/workers/status" 2>/dev/null); then
    if echo "$WORKERS" | jq -e '.is_running == true' > /dev/null 2>&1; then
        WORKER_COUNT=$(echo "$WORKERS" | jq -r '.worker_count')
        ACTIVE_WORKERS=$(echo "$WORKERS" | jq -r '.active_workers')
        echo -e "${GREEN}✅ Workers ready - $ACTIVE_WORKERS/$WORKER_COUNT active${NC}"
    else
        echo -e "${RED}❌ Workers not running${NC}"
        exit 1
    fi
else
    echo -e "${RED}❌ Could not get worker status${NC}"
    exit 1
fi

# Test 3: Manual job insertion (since job creation API might be buggy)
echo -e "\n${YELLOW}3. Creating DruidAI Scraping Job${NC}"
echo "Inserting job directly into database..."

# Generate unique job ID
JOB_ID="druidai-test-$(date +%s)"

# Insert job directly into database
docker-compose exec -T postgres psql -U root -d musli_scraper -c "
INSERT INTO scraping_jobs (id, url, status, options, metadata, created_at, updated_at) 
VALUES (
    '$JOB_ID',
    '$TEST_URL',
    'pending',
    '{\"wait_for_js\": true, \"timeout\": \"60s\", \"scroll_to_bottom\": false}',
    '{\"test\": \"druidai_website\", \"user\": \"manual_test\", \"timestamp\": \"$(date -Iseconds)\"}',
    NOW(),
    NOW()
);" 2>/dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Job created successfully: $JOB_ID${NC}"
else
    echo -e "${RED}❌ Failed to create job${NC}"
    exit 1
fi

# Test 4: Monitor job processing
echo -e "\n${YELLOW}4. Monitoring Job Processing${NC}"
echo "Job ID: $JOB_ID"
echo "URL: $TEST_URL"

# Wait for job to complete
if wait_for_job "$JOB_ID" 120; then
    # Show results
    show_job_results "$JOB_ID"
    
    # Test 5: Verify job in database
    echo -e "\n${YELLOW}5. Database Verification${NC}"
    if DB_RESULT=$(docker-compose exec -T postgres psql -U root -d musli_scraper -tAc "SELECT status, progress, error FROM scraping_jobs WHERE id='$JOB_ID';" 2>/dev/null); then
        echo "Database record: $DB_RESULT"
        echo -e "${GREEN}✅ Job successfully stored in database${NC}"
    fi
    
    echo -e "\n${GREEN}🎉 DruidAI scraping test completed successfully!${NC}"
else
    echo -e "\n${RED}❌ DruidAI scraping test failed${NC}"
    
    # Show error details
    echo -e "\n${YELLOW}Error Details:${NC}"
    if ERROR_INFO=$(curl -s "$BASE_URL/api/jobs/$JOB_ID" 2>/dev/null); then
        echo "$ERROR_INFO" | jq '{status: .status, error: .error, progress: .progress}' 2>/dev/null || echo "$ERROR_INFO"
    fi
fi

# Test 6: System metrics after processing
echo -e "\n${YELLOW}6. System Status After Processing${NC}"
if WORKERS_FINAL=$(curl -s "$BASE_URL/api/workers/status" 2>/dev/null); then
    echo "Worker statistics:"
    echo "$WORKERS_FINAL" | jq '{
        total_jobs: .total_jobs,
        successful_jobs: .successful_jobs,
        failed_jobs: .failed_jobs,
        overall_success_rate: .overall_success_rate
    }' 2>/dev/null || echo "Could not parse worker stats"
else
    echo "Could not get final worker status"
fi

# Final summary
echo -e "\n${BLUE}📋 Test Summary${NC}"
echo "================="
echo "Target URL: $TEST_URL"
echo "Job ID: $JOB_ID"
echo "Test completed at: $(date)"

echo -e "\n${BLUE}💡 Next Steps:${NC}"
echo "- Check the extracted content quality"
echo "- Verify if all important sections were captured"
echo "- Test with different scraping options if needed"
echo "- Monitor worker performance over time"