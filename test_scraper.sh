#!/bin/bash
# test_scraper.sh - Complete testing script for Musli Scraper Service

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🧪 Testing Musli Scraper Service${NC}"
echo "================================="

# 1. Test single page scraping for macromex.com
echo -e "\n${YELLOW}Test 1: Single page scraping - macromex.com${NC}"
RESPONSE1=$(curl -s -X POST http://localhost:8081/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://www.macromex.com/",
    "options": {
      "wait_for_js": true,
      "scroll_to_bottom": true,
      "timeout": "90s"
    }
  }')

JOB_ID1=$(echo $RESPONSE1 | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "Created job: $JOB_ID1"

# 2. Test single page scraping for druidai.com
echo -e "\n${YELLOW}Test 2: Single page scraping - druidai.com${NC}"
RESPONSE2=$(curl -s -X POST http://localhost:8081/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://www.druidai.com/",
    "options": {
      "wait_for_js": true,
      "scroll_to_bottom": true,
      "timeout": "90s"
    }
  }')

JOB_ID2=$(echo $RESPONSE2 | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "Created job: $JOB_ID2"

# 3. Test multi-page crawling
echo -e "\n${YELLOW}Test 3: Multi-page crawling - macromex.com (depth 2)${NC}"
RESPONSE3=$(curl -s -X POST http://localhost:8081/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://www.macromex.com/",
    "options": {
      "wait_for_js": true,
      "depth": 2,
      "max_pages": 5,
      "same_domain_only": true,
      "timeout": "120s"
    }
  }')

JOB_ID3=$(echo $RESPONSE3 | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "Created job: $JOB_ID3"

# Wait for processing
echo -e "\n${YELLOW}Waiting for jobs to process...${NC}"
sleep 30

# Check job statuses
echo -e "\n${YELLOW}Checking job statuses:${NC}"

echo -e "\nJob 1 (macromex.com single page):"
curl -s http://localhost:8081/api/jobs/$JOB_ID1 | jq '.status, .progress'

echo -e "\nJob 2 (druidai.com single page):"
curl -s http://localhost:8081/api/jobs/$JOB_ID2 | jq '.status, .progress'

echo -e "\nJob 3 (macromex.com crawl):"
curl -s http://localhost:8081/api/jobs/$JOB_ID3 | jq '.status, .progress'

# Check metrics
echo -e "\n${YELLOW}System metrics:${NC}"
curl -s http://localhost:8081/api/metrics | jq .

# Check worker status
echo -e "\n${YELLOW}Worker status:${NC}"
curl -s http://localhost:8081/api/workers/status | jq '.worker_count, .active_workers, .total_jobs'

# Check queue status
echo -e "\n${YELLOW}Queue status:${NC}"
curl -s http://localhost:8081/api/queue/status | jq .

# Wait more if needed
echo -e "\n${YELLOW}Waiting 60 more seconds for jobs to complete...${NC}"
sleep 60

# Get final results
echo -e "\n${GREEN}Final Results:${NC}"

echo -e "\n${YELLOW}Job 1 Results (macromex.com):${NC}"
RESULT1=$(curl -s http://localhost:8081/api/jobs/$JOB_ID1)
echo $RESULT1 | jq '.status, .results.module_pairs | length' 2>/dev/null || echo "Job still processing..."

echo -e "\n${YELLOW}Job 2 Results (druidai.com):${NC}"
RESULT2=$(curl -s http://localhost:8081/api/jobs/$JOB_ID2)
echo $RESULT2 | jq '.status, .results.module_pairs | length' 2>/dev/null || echo "Job still processing..."

echo -e "\n${YELLOW}Job 3 Results (macromex.com crawl):${NC}"
RESULT3=$(curl -s http://localhost:8081/api/jobs/$JOB_ID3)
echo $RESULT3 | jq '.status, .results.crawl_results.total_pages_scraped' 2>/dev/null || echo "Job still processing..."

echo -e "\n${GREEN}Testing complete!${NC}"