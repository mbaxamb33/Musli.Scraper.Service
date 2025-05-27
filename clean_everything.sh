#!/bin/bash
# clean_everything.sh - Complete cleanup of Musli Scraper Service

set -e

echo "🧹 COMPLETE CLEANUP - This will delete EVERYTHING!"
echo "=================================================="
echo "This will delete:"
echo "  - All Docker containers"
echo "  - All Docker volumes (databases)"
echo "  - All Docker networks"
echo "  - Generated code"
echo "  - Migration state"
echo ""
read -p "Are you sure? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Cleanup cancelled."
    exit 0
fi

echo ""
echo "🛑 Stopping all containers..."
docker-compose down -v --remove-orphans 2>/dev/null || true

echo "🗑️  Removing all project containers..."
docker ps -a | grep "musli" | awk '{print $1}' | xargs -r docker rm -f 2>/dev/null || true

echo "🗑️  Removing all project volumes..."
docker volume ls | grep "musli" | awk '{print $2}' | xargs -r docker volume rm -f 2>/dev/null || true

echo "🗑️  Removing all project networks..."
docker network ls | grep "musli" | awk '{print $2}' | xargs -r docker network rm 2>/dev/null || true

echo "🗑️  Cleaning up unused Docker resources..."
docker system prune -f --volumes 2>/dev/null || true

echo "📁 Removing generated files..."
rm -rf db/sqlc/*.go 2>/dev/null || true
rm -rf bin/ 2>/dev/null || true
rm -f sqlc.yaml.backup 2>/dev/null || true

echo "📁 Removing migration files (keeping originals)..."
# Remove any extra migration files we created during fixes
rm -f db/migration/000002_fix_job_status_type.up.sql 2>/dev/null || true
rm -f db/migration/000002_fix_job_status_type.down.sql 2>/dev/null || true

echo "📁 Removing any temporary fix files..."
rm -f internal/services/job_service_fix.go 2>/dev/null || true

echo "✅ Complete cleanup done!"
echo ""
echo "The environment is now completely clean."
echo "You can now run a fresh setup with:"
echo "  ./fresh_start.sh"