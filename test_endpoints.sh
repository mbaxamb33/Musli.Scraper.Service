#!/bin/bash

# working_test.sh - Working test script with proper JSON handling

BASE_URL="http://localhost:8081"

echo "🧪 Testing Musli Scraper Service - Working Version"
echo "================================================="

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test 1: Basic endpoints
echo -e "${YELLOW}1. Testing basic endpoints...${NC}"

curl -s "$BASE_URL/health" > /dev/null && echo -e "   ${GREEN}✅ Health check${NC}" || echo -e "   ${RED}❌ Health check${NC}"
curl -s "$BASE_URL/api/metrics" > /dev/null && echo -e "   ${GREEN}✅ Metrics${NC}" || echo -e "   ${RED}❌ Metrics${NC}"
curl -s "$BASE_URL/api/workers/status" > /dev/null && echo -e "   ${GREEN}✅ Worker status${NC}" || echo -e "   ${RED}❌ Worker status${NC}"

# Test 2: Create simple job using heredoc to avoid shell escaping issues
echo -e "\n${YELLOW}2. Creating simple job...${NC}"

RESPONSE=$(curl -s -X POST "$BASE_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d @- << 'EOF'
{
  "url": "https://httpbin.org/html",
  "options": {
    "wait_for_js": false
  }
}
EOF
)

if echo "$RESPONSE" | grep -q '"id"'; then
    echo -e "   ${GREEN}✅ Simple job created${NC}"
    JOB_ID=$(echo "$RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    echo "   Job ID: $JOB_ID"
else
    echo -e "   ${RED}❌ Simple job failed${NC}"
    echo "   Response: $RESPONSE"
fi

# Test 3: Create job with options
echo -e "\n${YELLOW}3. Creating job with options...${NC}"

RESPONSE2=$(curl -s -X POST "$BASE_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d @- << 'EOF'
{
  "url": "https://example.com",
  "priority": 8,
  "options": {
    "wait_for_js": true,
    "timeout": "45s",
    "scroll_to_bottom": true
  },
  "metadata": {
    "test": true,
    "source": "working_test"
  }
}
EOF
)

if echo "$RESPONSE2" | grep -q '"id"'; then
    echo -e "   ${GREEN}✅ Complex job created${NC}"
    JOB_ID2=$(echo "$RESPONSE2" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    echo "   Job ID: $JOB_ID2"
else
    echo -e "   ${RED}❌ Complex job failed${NC}"
    echo "   Response: $RESPONSE2"
fi

# Test 4: Check job status
if [ ! -z "$JOB_ID" ]; then
    echo -e "\n${YELLOW}4. Checking job status...${NC}"
    STATUS=$(curl -s "$BASE_URL/api/jobs/$JOB_ID" | jq -r '.status')
    echo "   Job status: $STATUS"
fi

# Test 5: List all jobs
echo -e "\n${YELLOW}5. Listing all jobs...${NC}"
TOTAL_JOBS=$(curl -s "$BASE_URL/api/jobs" | jq -r '.total_count')
echo "   Total jobs in system: $TOTAL_JOBS"

# Test 6: Check current metrics
echo -e "\n${YELLOW}6. Current metrics...${NC}"
curl -s "$BASE_URL/api/metrics" | jq '{total_jobs: .total_jobs, pending: .pending_jobs, processing: .processing_jobs, completed: .completed_jobs}'

echo -e "\n${GREEN}🎉 Testing completed!${NC}"