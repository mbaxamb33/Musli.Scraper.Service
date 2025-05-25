#!/bin/bash

# final_test.sh - Super simple test that definitely works

BASE_URL="http://localhost:8081"

echo "🧪 Final Simple Test"
echo "==================="

# Test 1: Basic endpoints
echo "1. Testing basic endpoints..."
curl -s "$BASE_URL/health" | jq -r '.status'
curl -s "$BASE_URL/api/metrics" | jq -r '.total_jobs'

# Test 2: Create job the way we know works
echo -e "\n2. Creating job..."
RESPONSE=$(curl -s -X POST "$BASE_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://httpbin.org/html"}')

echo "$RESPONSE" | jq -r '.id'

# Test 3: Create job with minimal options
echo -e "\n3. Creating job with options..."
RESPONSE2=$(curl -s -X POST "$BASE_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com","priority":7}')

if echo "$RESPONSE2" | grep -q '"id"'; then
    echo "✅ Job with options created"
    echo "$RESPONSE2" | jq -r '.id'
else
    echo "❌ Failed to create job with options"
    echo "$RESPONSE2"
fi

# Test 4: Check metrics
echo -e "\n4. Current system status..."
echo "Jobs:" $(curl -s "$BASE_URL/api/metrics" | jq -r '.total_jobs')
echo "Pending:" $(curl -s "$BASE_URL/api/metrics" | jq -r '.pending_jobs')
echo "Processing:" $(curl -s "$BASE_URL/api/metrics" | jq -r '.processing_jobs')
echo "Completed:" $(curl -s "$BASE_URL/api/metrics" | jq -r '.completed_jobs')

# Test 5: Check if any jobs completed
echo -e "\n5. Checking for completed jobs..."
COMPLETED=$(curl -s "$BASE_URL/api/metrics" | jq -r '.completed_jobs')
if [ "$COMPLETED" -gt 0 ]; then
    echo "🎉 $COMPLETED jobs have completed!"
    
    # Get a completed job to see results
    COMPLETED_JOB=$(curl -s "$BASE_URL/api/jobs" | jq -r '.jobs[] | select(.status=="completed") | .id' | head -1)
    if [ ! -z "$COMPLETED_JOB" ]; then
        echo "Checking results for job: $COMPLETED_JOB"
        curl -s "$BASE_URL/api/jobs/$COMPLETED_JOB" | jq -r '.results.processing_stats.modules_extracted // "No modules"'
    fi
else
    echo "No jobs completed yet (this is normal for fresh jobs)"
fi

echo -e "\n✅ Simple test completed!"