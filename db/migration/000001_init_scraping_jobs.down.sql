-- db/migration/000001_init_scraping_jobs.down.sql
-- Rollback migration for scraping jobs

-- Drop functions
DROP FUNCTION IF EXISTS cleanup_old_jobs(INTEGER);
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop view
DROP VIEW IF EXISTS job_metrics;

-- Drop table (this will automatically drop triggers and indexes)
DROP TABLE IF EXISTS scraping_jobs;

-- Drop enum type
DROP TYPE IF EXISTS job_status;