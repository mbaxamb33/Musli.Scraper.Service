#!/bin/bash

# quick_debug_test.sh - Debug the API endpoints
set -e

BASE_URL="http://localhost:8081"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Quick API Debug${NC}"
echo "=================="

# Test 1: Health endpoint
echo -e "\n${YELLOW}1. Testing Health Endpoint${NC}"
echo "GET $BASE_URL/health"
echo "Response:"
if HEALTH_RESPONSE=$(curl -s "$BASE_URL/health" 2>/dev/null); then
    echo "$HEALTH_RESPONSE"
    echo -e "${GREEN}✅ Health endpoint accessible${NC}"
else
    echo -e "${RED}❌ Health endpoint failed${NC}"
    echo "Is the service running? Try: make dev-run"
    exit 1
fi

# Test 2: Metrics endpoint (raw)
echo -e "\n${YELLOW}2. Testing Metrics Endpoint (Raw Response)${NC}"
echo "GET $BASE_URL/api/metrics"
echo "Response:"
if METRICS_RESPONSE=$(curl -s "$BASE_URL/api/metrics" 2>/dev/null); then
    echo "$METRICS_RESPONSE"
    
    # Try to parse as JSON
    echo -e "\n${YELLOW}Parsing as JSON:${NC}"
    if echo "$METRICS_RESPONSE" | jq . 2>/dev/null; then
        echo -e "${GREEN}✅ Valid JSON response${NC}"
    else
        echo -e "${RED}❌ Invalid JSON response${NC}"
        echo "Raw response bytes:"
        echo "$METRICS_RESPONSE" | hexdump -C | head -5
    fi
else
    echo -e "${RED}❌ Metrics endpoint failed${NC}"
fi

# Test 3: Workers endpoint
echo -e "\n${YELLOW}3. Testing Workers Endpoint${NC}"
echo "GET $BASE_URL/api/workers/status"
echo "Response:"
if WORKERS_RESPONSE=$(curl -s "$BASE_URL/api/workers/status" 2>/dev/null); then
    echo "$WORKERS_RESPONSE"
    
    if echo "$WORKERS_RESPONSE" | jq . 2>/dev/null; then
        echo -e "${GREEN}✅ Valid JSON response${NC}"
    else
        echo -e "${RED}❌ Invalid JSON response${NC}"
    fi
else
    echo -e "${RED}❌ Workers endpoint failed${NC}"
fi

# Test 4: Jobs listing endpoint
echo -e "\n${YELLOW}4. Testing Jobs Endpoint${NC}"
echo "GET $BASE_URL/api/jobs"
echo "Response:"
if JOBS_RESPONSE=$(curl -s "$BASE_URL/api/jobs" 2>/dev/null); then
    echo "$JOBS_RESPONSE"
    
    if echo "$JOBS_RESPONSE" | jq . 2>/dev/null; then
        echo -e "${GREEN}✅ Valid JSON response${NC}"
    else
        echo -e "${RED}❌ Invalid JSON response${NC}"
    fi
else
    echo -e "${RED}❌ Jobs endpoint failed${NC}"
fi

# Test 5: Create a simple job
echo -e "\n${YELLOW}5. Testing Job Creation${NC}"
echo "POST $BASE_URL/api/jobs"
JOB_DATA='{
    "url": "https://example.com",
    "options": {
        "wait_for_js": false,
        "timeout": "30s"
    },
    "metadata": {
        "test": "quick_debug"
    }
}'

echo "Request body: $JOB_DATA"
echo "Response:"
if CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/api/jobs" \
    -H "Content-Type: application/json" \
    -d "$JOB_DATA" 2>/dev/null); then
    echo "$CREATE_RESPONSE"
    
    if echo "$CREATE_RESPONSE" | jq . 2>/dev/null; then
        echo -e "${GREEN}✅ Job creation successful${NC}"
        
        # Extract job ID if possible
        if JOB_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id' 2>/dev/null); then
            if [ "$JOB_ID" != "null" ] && [ -n "$JOB_ID" ]; then
                echo -e "${BLUE}Created job: $JOB_ID${NC}"
                
                # Check job status
                echo -e "\n${YELLOW}6. Checking Job Status${NC}"
                sleep 2
                if JOB_STATUS_RESPONSE=$(curl -s "$BASE_URL/api/jobs/$JOB_ID" 2>/dev/null); then
                    echo "$JOB_STATUS_RESPONSE" | jq .
                fi
            fi
        fi
    else
        echo -e "${RED}❌ Invalid JSON response${NC}"
    fi
else
    echo -e "${RED}❌ Job creation failed${NC}"
fi

# Test 6: Check service logs (if available)
echo -e "\n${YELLOW}7. Service Information${NC}"
echo "Service should be running on: $BASE_URL"
echo "Check service logs in the terminal where you ran 'make dev-run'"

echo -e "\n${BLUE}💡 Debugging Tips:${NC}"
echo "- If JSON parsing fails, the endpoint might be returning HTML error pages"
echo "- Check the service logs for any error messages"
echo "- Ensure the service is fully started before running tests"
echo "- Try: curl -v $BASE_URL/api/metrics to see HTTP headers"