-- db/query/scraping_jobs.sql
-- SQLC queries for scraping jobs management

-- name: CreateScrapingJob :one
INSERT INTO scraping_jobs (
    id, url, datasource_id, callback_url, options, metadata, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetScrapingJob :one
SELECT * FROM scraping_jobs WHERE id = $1;

-- name: GetScrapingJobByURL :one
SELECT * FROM scraping_jobs WHERE url = $1 ORDER BY created_at DESC LIMIT 1;

-- name: ListScrapingJobs :many
SELECT * FROM scraping_jobs 
ORDER BY created_at DESC 
LIMIT $1 OFFSET $2;

-- name: ListScrapingJobsByStatus :many
SELECT * FROM scraping_jobs 
WHERE status = $1 
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- name: ListPendingJobs :many
SELECT * FROM scraping_jobs 
WHERE status = 'pending' 
ORDER BY created_at ASC 
LIMIT $1;

-- name: ListProcessingJobs :many
SELECT * FROM scraping_jobs 
WHERE status = 'processing' 
ORDER BY started_at ASC;

-- name: UpdateJobStatus :one
UPDATE scraping_jobs 
SET status = $2, progress = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 
RETURNING *;

-- name: UpdateJobProgress :one
UPDATE scraping_jobs 
SET progress = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 
RETURNING *;

-- name: StartJob :one
UPDATE scraping_jobs 
SET status = 'processing', started_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 
RETURNING *;

-- name: CompleteJob :one
UPDATE scraping_jobs 
SET status = 'completed', progress = 100, completed_at = CURRENT_TIMESTAMP, 
    results = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 
RETURNING *;

-- name: FailJob :one
UPDATE scraping_jobs 
SET status = 'failed', error = $2, completed_at = CURRENT_TIMESTAMP,
    retry_count = retry_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 
RETURNING *;

-- name: CancelJob :one
UPDATE scraping_jobs 
SET status = 'canceled', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 
RETURNING *;

-- name: RetryJob :one
UPDATE scraping_jobs 
SET status = 'pending', error = NULL, progress = 0,
    retry_count = retry_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND retry_count < $2
RETURNING *;

-- name: DeleteScrapingJob :exec
DELETE FROM scraping_jobs WHERE id = $1;

-- name: CleanupOldJobs :exec
DELETE FROM scraping_jobs 
WHERE created_at < $1 
AND status IN ('completed', 'failed', 'canceled');

-- name: GetJobsByDatasourceID :many
SELECT * FROM scraping_jobs 
WHERE datasource_id = $1 
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- name: CountJobsByStatus :one
SELECT COUNT(*) FROM scraping_jobs WHERE status = $1;

-- name: CountTotalJobs :one
SELECT COUNT(*) FROM scraping_jobs;

-- name: GetJobMetrics :one
SELECT 
    total_jobs,
    pending_jobs,
    processing_jobs,
    completed_jobs,
    failed_jobs,
    canceled_jobs,
    success_rate,
    avg_processing_time_seconds
FROM job_metrics;

-- name: GetRecentJobs :many
SELECT * FROM scraping_jobs 
WHERE created_at >= $1 
ORDER BY created_at DESC 
LIMIT $2;

-- name: GetJobsForCleanup :many
SELECT id, created_at, status FROM scraping_jobs 
WHERE created_at < $1 
AND status IN ('completed', 'failed', 'canceled')
LIMIT $2;

-- name: GetActiveJobsCount :one
SELECT COUNT(*) FROM scraping_jobs 
WHERE status IN ('pending', 'processing');

-- name: GetJobsByURLPattern :many
SELECT * FROM scraping_jobs 
WHERE url ILIKE $1 
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- name: UpdateJobResults :one
UPDATE scraping_jobs 
SET results = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 
RETURNING *;

-- name: GetStaleProcessingJobs :many
SELECT * FROM scraping_jobs 
WHERE status = 'processing' 
AND started_at < $1
ORDER BY started_at ASC;