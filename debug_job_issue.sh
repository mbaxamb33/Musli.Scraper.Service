#!/bin/bash

# debug_job_issue.sh - Debug why the job isn't being found by the API
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Debugging Job Issue${NC}"
echo "======================"

# Check what jobs exist in database
echo -e "\n${YELLOW}1. Jobs in Database:${NC}"
docker-compose exec -T postgres psql -U root -d musli_scraper -c "
SELECT id, url, status, progress, created_at 
FROM scraping_jobs 
ORDER BY created_at DESC 
LIMIT 5;"

# Check the specific job we created
JOB_ID="druidai-test-1748243980"
echo -e "\n${YELLOW}2. Specific Job Details:${NC}"
echo "Looking for job: $JOB_ID"

docker-compose exec -T postgres psql -U root -d musli_scraper -c "
SELECT id, url, status, progress, error, created_at, updated_at
FROM scraping_jobs 
WHERE id = '$JOB_ID';"

# Check what the API returns
echo -e "\n${YELLOW}3. API Response for Job:${NC}"
curl -v "http://localhost:8081/api/jobs/$JOB_ID" 2>&1

# Test the jobs listing API
echo -e "\n${YELLOW}4. Jobs Listing API:${NC}"
curl -s "http://localhost:8081/api/jobs" | jq '.jobs[] | {id: .id, status: .status}'

# Check if workers are processing anything
echo -e "\n${YELLOW}5. Worker Status:${NC}"
curl -s "http://localhost:8081/api/workers/status" | jq '{
  is_running: .is_running,
  active_workers: .active_workers,
  total_jobs: .total_jobs,
  workers: .workers[] | {id: .id, is_active: .is_active, jobs_processed: .jobs_processed}
}'

# Try to create a job using the working jobs API directly via database
echo -e "\n${YELLOW}6. Creating Simple Test Job:${NC}"
SIMPLE_JOB_ID="simple-test-$(date +%s)"

docker-compose exec -T postgres psql -U root -d musli_scraper -c "
INSERT INTO scraping_jobs (id, url, status, options, metadata, created_at, updated_at) 
VALUES (
    '$SIMPLE_JOB_ID',
    'https://httpbin.org/html',
    'pending',
    '{\"wait_for_js\": false, \"timeout\": \"30s\"}',
    '{\"test\": \"simple_debug\"}',
    NOW(),
    NOW()
);"

echo "Created simple job: $SIMPLE_JOB_ID"

# Wait a moment and check if it gets processed
echo -e "\n${YELLOW}7. Checking Simple Job Processing:${NC}"
for i in {1..10}; do
    echo -n "[$i/10] "
    STATUS=$(docker-compose exec -T postgres psql -U root -d musli_scraper -tAc "SELECT status FROM scraping_jobs WHERE id='$SIMPLE_JOB_ID';" 2>/dev/null)
    echo "Status: $STATUS"
    
    if [ "$STATUS" != "pending" ]; then
        echo "Job status changed! Checking details..."
        docker-compose exec -T postgres psql -U root -d musli_scraper -c "
        SELECT id, status, progress, error, results IS NOT NULL as has_results
        FROM scraping_jobs 
        WHERE id = '$SIMPLE_JOB_ID';"
        break
    fi
    sleep 2
done

echo -e "\n${BLUE}💡 Analysis:${NC}"
echo "============"
echo "1. Check if jobs are being created in the database correctly"
echo "2. Check if the API routing is working for individual job lookups"
echo "3. Check if workers are picking up jobs from the database"
echo "4. Check if there are any errors in the service logs"