#!/bin/bash

# debug_metrics.sh - Debug the metrics endpoint issue

BASE_URL="http://localhost:8081"

echo "🔍 Debugging Metrics Issue"
echo "=========================="

echo "1. Testing metrics endpoint with verbose output:"
curl -v "$BASE_URL/api/metrics"

echo -e "\n\n2. Testing database connection via health check:"
curl -s "$BASE_URL/health" | jq '.'

echo -e "\n\n3. Checking if there are any jobs in the database:"
curl -s "$BASE_URL/api/jobs?limit=1" | jq '.'

echo -e "\n\n4. Creating a simple test job to populate metrics:"
curl -X POST "$BASE_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://httpbin.org/html","options":{"wait_for_js":false}}' \
  -v

echo -e "\n\n5. Trying metrics again after job creation:"
curl -s "$BASE_URL/api/metrics" | jq '.'