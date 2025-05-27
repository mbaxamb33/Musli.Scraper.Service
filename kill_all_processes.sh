#!/bin/bash
# kill_all_processes.sh - Kill all related processes and clean up

echo "🔪 Killing all related processes..."
echo "==================================="

# Kill any Go processes
echo "Killing Go processes..."
pkill -f "go run" 2>/dev/null || true
pkill -f "scraper-service" 2>/dev/null || true

# Kill any PostgreSQL processes (outside Docker)
echo "Killing PostgreSQL processes..."
pkill -f postgres 2>/dev/null || true
pkill -f psql 2>/dev/null || true

# Kill any migration processes
echo "Killing migration processes..."
pkill -f migrate 2>/dev/null || true

# Stop and remove ALL Docker containers
echo "Stopping ALL Docker containers..."
docker stop $(docker ps -aq) 2>/dev/null || true
docker rm -f $(docker ps -aq) 2>/dev/null || true

# Remove Docker volumes
echo "Removing Docker volumes..."
docker volume prune -f 2>/dev/null || true

# Clean up any WSL2 specific issues (if on Windows WSL)
echo "Cleaning WSL2 networking (if applicable)..."
# Reset WSL networking if needed
sudo service docker restart 2>/dev/null || true

# Wait for everything to settle
sleep 3

echo "✅ All processes killed and cleaned up"