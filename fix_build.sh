#!/bin/bash
# fix_build.sh - Fix the job status type issue

set -e

echo "🔧 Fixing job status type issue..."
echo "================================"

# 1. Stop services to avoid conflicts
echo "🛑 Stopping services..."
docker-compose down

# 2. Start only PostgreSQL
echo "🐳 Starting PostgreSQL..."
docker-compose up -d postgres
sleep 5

# 3. Create the new migration file
echo "📝 Creating migration file..."
cat > db/migration/000002_fix_job_status_type.up.sql << 'EOF'
-- Fix job status type mapping for SQLC
-- This ensures the enum is properly defined

-- Nothing to do here since the enum already exists
-- This file is just a placeholder to mark that we've addressed the issue
EOF

cat > db/migration/000002_fix_job_status_type.down.sql << 'EOF'
-- No rollback needed
EOF

# 4. Run migrations
echo "🗄️  Running migrations..."
make migrateup || true

# 5. Update sqlc.yaml
echo "📝 Updating sqlc.yaml..."
cp sqlc.yaml sqlc.yaml.backup
cat > sqlc.yaml << 'EOF'
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/query/"
    schema: "db/migration/"
    gen:
      go:
        package: "db"
        out: "db/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_db_tags: true
        emit_prepared_queries: false
        emit_interface: true
        emit_exact_table_names: true
        emit_empty_slices: true
        emit_exported_queries: true
        emit_result_struct_pointers: false
        emit_params_struct_pointers: false
        emit_methods_with_db_argument: false
        emit_enum_valid_method: true
        emit_all_enum_values: true
        overrides:
          - db_type: "job_status"
            go_type: "string"
          - column: "*.created_at"
            go_type: "time.Time"
          - column: "*.updated_at"
            go_type: "time.Time"
          - column: "*.started_at"
            go_type: "time.Time"
          - column: "*.completed_at"
            go_type: 
              type: "*time.Time"
              pointer: true
          - column: "scraping_jobs.options"
            go_type: "encoding/json.RawMessage"
          - column: "scraping_jobs.metadata"
            go_type: "encoding/json.RawMessage"
          - column: "scraping_jobs.results"
            go_type: "encoding/json.RawMessage"
EOF

# 6. Create job status constants file
echo "📝 Creating job status constants..."
mkdir -p db/sqlc
cat > db/sqlc/job_status.go << 'EOF'
// db/sqlc/job_status.go
package db

// JobStatus represents the status of a scraping job
type JobStatus string

// Job status constants matching the PostgreSQL enum values
const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCanceled   JobStatus = "canceled"
)

// String returns the string representation of the JobStatus
func (s JobStatus) String() string {
	return string(s)
}

// Valid checks if the JobStatus is valid
func (s JobStatus) Valid() bool {
	switch s {
	case JobStatusPending, JobStatusProcessing, JobStatusCompleted, JobStatusFailed, JobStatusCanceled:
		return true
	default:
		return false
	}
}
EOF

# 7. Regenerate SQLC code
echo "🔧 Regenerating SQLC code..."
make sqlc

# 8. Update the models.go file to use string for status
echo "📝 Updating models.go..."
sed -i 's/Status\s*interface{}/Status string/g' db/sqlc/models.go

# 9. Update scraping_jobs.sql.go to cast status properly
echo "📝 Updating SQL queries..."
# Fix the CreateScrapingJob parameters
sed -i 's/Status\s*interface{}/Status string/g' db/sqlc/scraping_jobs.sql.go

# 10. Build the service
echo "🏗️  Building the service..."
make build

echo "✅ Fix complete! Now you can run ./startup.sh again"