-- db/migration/000001_init_scraping_jobs.up.sql
-- Job database schema for Musli.Scraper.Service
-- This is a lightweight database for job management only

-- Create enum for job status if it doesn't already exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'job_status') THEN
        CREATE TYPE job_status AS ENUM ('pending', 'processing', 'completed', 'failed', 'canceled');
    END IF;
END
$$;


-- Main scraping jobs table
CREATE TABLE scraping_jobs (
    id VARCHAR(36) PRIMARY KEY,
    url TEXT NOT NULL,
    datasource_id INTEGER,
    callback_url TEXT,
    options JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    status job_status NOT NULL DEFAULT 'pending',
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    error TEXT,
    retry_count INTEGER DEFAULT 0,
    results JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_scraping_jobs_status ON scraping_jobs(status);
CREATE INDEX idx_scraping_jobs_created_at ON scraping_jobs(created_at);
CREATE INDEX idx_scraping_jobs_updated_at ON scraping_jobs(updated_at);
CREATE INDEX idx_scraping_jobs_datasource_id ON scraping_jobs(datasource_id) WHERE datasource_id IS NOT NULL;
CREATE INDEX idx_scraping_jobs_url_hash ON scraping_jobs(MD5(url));
CREATE INDEX idx_scraping_jobs_status_created ON scraping_jobs(status, created_at);

-- Partial index for active jobs
CREATE INDEX idx_scraping_jobs_active ON scraping_jobs(created_at) 
WHERE status IN ('pending', 'processing');

-- Job metrics view for monitoring
CREATE VIEW job_metrics AS
SELECT 
    COUNT(*) as total_jobs,
    COUNT(*) FILTER (WHERE status = 'pending') as pending_jobs,
    COUNT(*) FILTER (WHERE status = 'processing') as processing_jobs,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_jobs,
    COUNT(*) FILTER (WHERE status = 'failed') as failed_jobs,
    COUNT(*) FILTER (WHERE status = 'canceled') as canceled_jobs,
    ROUND(
        COUNT(*) FILTER (WHERE status = 'completed')::DECIMAL / 
        NULLIF(COUNT(*) FILTER (WHERE status IN ('completed', 'failed')), 0) * 100, 2
    ) as success_rate,
    AVG(
        EXTRACT(EPOCH FROM (completed_at - started_at))
    ) FILTER (WHERE status = 'completed' AND completed_at IS NOT NULL) as avg_processing_time_seconds
FROM scraping_jobs;

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at
CREATE TRIGGER update_scraping_jobs_updated_at 
    BEFORE UPDATE ON scraping_jobs 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Function to cleanup old jobs
CREATE OR REPLACE FUNCTION cleanup_old_jobs(retention_hours INTEGER DEFAULT 168)
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM scraping_jobs 
    WHERE created_at < NOW() - INTERVAL '1 hour' * retention_hours
    AND status IN ('completed', 'failed', 'canceled');
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Create a sample job for testing (optional)
INSERT INTO scraping_jobs (id, url, status, options, metadata) 
VALUES (
    'test-job-' || EXTRACT(EPOCH FROM NOW())::TEXT,
    'https://example.com',
    'pending',
    '{"wait_for_js": true, "timeout": "60s"}',
    '{"test": true}'
) ON CONFLICT DO NOTHING;