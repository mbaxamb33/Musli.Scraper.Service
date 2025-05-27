#!/bin/bash
# check_processes.sh - Check for lingering processes

echo "🔍 Checking for lingering processes..."
echo "======================================"

# Check for PostgreSQL processes
echo "📊 PostgreSQL processes:"
ps aux | grep -E "(postgres|psql)" | grep -v grep || echo "No PostgreSQL processes found"

# Check for Go processes
echo -e "\n📊 Go processes:"
ps aux | grep -E "(go run|scraper-service)" | grep -v grep || echo "No Go processes found"

# Check for Docker processes
echo -e "\n📊 Docker containers:"
docker ps -a | grep -E "(musli|postgres|rabbitmq)" || echo "No related containers found"

# Check what's using port 5433 (PostgreSQL)
echo -e "\n🔌 Port 5433 (PostgreSQL):"
sudo lsof -i :5433 2>/dev/null || netstat -tlnp 2>/dev/null | grep :5433 || echo "Port 5433 is free"

# Check what's using port 8081 (Service)
echo -e "\n🔌 Port 8081 (Service):"
sudo lsof -i :8081 2>/dev/null || netstat -tlnp 2>/dev/null | grep :8081 || echo "Port 8081 is free"

# Check what's using port 5672 (RabbitMQ)
echo -e "\n🔌 Port 5672 (RabbitMQ):"
sudo lsof -i :5672 2>/dev/null || netstat -tlnp 2>/dev/null | grep :5672 || echo "Port 5672 is free"

# Check for migrate processes
echo -e "\n📊 Migration processes:"
ps aux | grep migrate | grep -v grep || echo "No migration processes found"

echo -e "\n💡 To kill processes:"
echo "  - Kill by PID: kill -9 <PID>"
echo "  - Kill all postgres: pkill -f postgres"
echo "  - Kill all go: pkill -f 'go run'"
echo "  - Kill scraper: pkill -f scraper-service"