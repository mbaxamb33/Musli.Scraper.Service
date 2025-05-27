// internal/services/job_service.go
package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/mbaxamb3/nusli/scraper-service/db/sqlc"
	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/internal/queue"
	"github.com/mbaxamb3/nusli/scraper-service/internal/scraper"
	"github.com/mbaxamb3/nusli/scraper-service/internal/storage"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

// JobService handles scraping job operations
type JobService struct {
	db           *storage.Database
	scraper      *scraper.Engine
	config       *config.Config
	logger       *zap.Logger
	queueManager *queue.RabbitMQManager
}

// NewJobService creates a new job service instance
func NewJobService(db *storage.Database, scraperEngine *scraper.Engine, cfg *config.Config, logger *zap.Logger, queueManager *queue.RabbitMQManager) *JobService {
	return &JobService{
		db:           db,
		scraper:      scraperEngine,
		config:       cfg,
		logger:       logger,
		queueManager: queueManager,
	}
}

// CreateScrapingJob creates a new scraping job and publishes it to the queue
func (js *JobService) CreateScrapingJob(ctx context.Context, req models.ScrapingJobRequest) (*models.ScrapingJob, error) {
	// Generate unique job ID
	jobID := uuid.New().String()

	js.logger.Info("Creating scraping job",
		zap.String("job_id", jobID),
		zap.String("url", req.URL),
		zap.Int32("datasource_id", req.DatasourceID))

	// Convert options to JSON
	optionsJSON, err := json.Marshal(req.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Prepare database parameters
	params := db.CreateScrapingJobParams{
		ID:       jobID,
		Url:      req.URL,
		Status:   db.JobStatusPending,
		Options:  optionsJSON,
		Metadata: metadataJSON,
	}

	// Set optional datasource ID
	if req.DatasourceID != 0 {
		params.DatasourceID = pgtype.Int4{Int32: req.DatasourceID, Valid: true}
	}

	// Set optional callback URL
	if req.CallbackURL != "" {
		params.CallbackUrl = pgtype.Text{String: req.CallbackURL, Valid: true}
	}

	// Create job in database
	dbJob, err := js.db.GetQueries().CreateScrapingJob(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create job in database: %w", err)
	}

	// Convert to model
	job, err := js.convertDBJobToModel(dbJob)
	if err != nil {
		return nil, fmt.Errorf("failed to convert job: %w", err)
	}

	// Publish job to RabbitMQ queue
	if js.queueManager != nil {
		priority := 5 // Default priority
		if req.Priority > 0 && req.Priority <= 9 {
			priority = req.Priority
		}

		jobMsg := queue.JobMessage{
			JobID:     jobID,
			URL:       req.URL,
			Priority:  priority,
			CreatedAt: time.Now(),
		}

		if err := js.queueManager.PublishJob(ctx, jobMsg); err != nil {
			js.logger.Error("Failed to publish job to queue",
				zap.String("job_id", jobID),
				zap.Error(err))

			// Mark job as failed if we can't publish to queue
			js.failJob(ctx, jobID, fmt.Sprintf("Failed to publish to queue: %v", err))
			return nil, fmt.Errorf("failed to publish job to queue: %w", err)
		}

		js.logger.Info("Job published to queue successfully",
			zap.String("job_id", jobID),
			zap.String("url", req.URL),
			zap.Int("priority", priority))
	} else {
		js.logger.Warn("Queue manager not available, job created but not queued",
			zap.String("job_id", jobID))
	}

	js.logger.Info("Created scraping job successfully",
		zap.String("job_id", jobID),
		zap.String("url", req.URL))

	return job, nil
}

// ProcessJobFromQueue processes a job from the queue (called by RabbitMQ workers)
func (js *JobService) ProcessJobFromQueue(ctx context.Context, jobID string) error {
	js.logger.Info("Processing job from queue", zap.String("job_id", jobID))

	// Get job from database
	dbJob, err := js.db.GetQueries().GetScrapingJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Check if job is in pending status
	if dbJob.Status != db.JobStatusPending {
		js.logger.Warn("Job is not in pending status",
			zap.String("job_id", jobID),
			zap.String("status", string(dbJob.Status)))
		return fmt.Errorf("job is not in pending status: %s", dbJob.Status)
	}

	// Start the job
	_, err = js.db.GetQueries().StartJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to start job: %w", err)
	}

	js.logger.Info("Job marked as processing",
		zap.String("job_id", jobID),
		zap.String("url", dbJob.Url))

	// Process the job synchronously
	return js.executeJob(ctx, jobID, dbJob)
}

// executeJob performs the actual scraping work
func (js *JobService) executeJob(ctx context.Context, jobID string, dbJob db.ScrapingJobs) error {
	startTime := time.Now()

	// Parse options
	var options models.ScrapingOptions
	if len(dbJob.Options) > 0 {
		if err := json.Unmarshal(dbJob.Options, &options); err != nil {
			js.logger.Error("Failed to parse job options",
				zap.String("job_id", jobID),
				zap.Error(err))
			return js.failJob(ctx, jobID, fmt.Sprintf("Failed to parse options: %v", err))
		}
	}

	// Set default timeout if not specified
	if options.Timeout == 0 {
		options.Timeout = js.config.BrowserTimeout
	}

	// Update progress
	js.updateJobProgress(ctx, jobID, 10)

	// Add processing timeout
	processingCtx, cancel := context.WithTimeout(ctx, js.config.JobTimeout)
	defer cancel()

	// Perform scraping
	results, err := js.scraper.ScrapePage(processingCtx, dbJob.Url, options)
	if err != nil {
		js.logger.Error("Failed to scrape page",
			zap.String("job_id", jobID),
			zap.String("url", dbJob.Url),
			zap.Duration("processing_time", time.Since(startTime)),
			zap.Error(err))
		return js.failJob(ctx, jobID, err.Error())
	}

	// Update progress
	js.updateJobProgress(ctx, jobID, 90)

	// Convert results to JSON
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		js.logger.Error("Failed to marshal results",
			zap.String("job_id", jobID),
			zap.Error(err))
		return js.failJob(ctx, jobID, fmt.Sprintf("Failed to marshal results: %v", err))
	}

	// Complete the job
	_, err = js.db.GetQueries().CompleteJob(ctx, db.CompleteJobParams{
		ID:      jobID,
		Results: resultsJSON,
	})
	if err != nil {
		js.logger.Error("Failed to complete job",
			zap.String("job_id", jobID),
			zap.Error(err))
		return err
	}

	processingTime := time.Since(startTime)

	js.logger.Info("Job completed successfully",
		zap.String("job_id", jobID),
		zap.String("url", dbJob.Url),
		zap.Int("modules_extracted", len(results.ModulePairs)),
		zap.Duration("processing_time", processingTime),
		zap.Int("content_length", results.ProcessingStats.ContentLength))

	// Send callback if configured
	if dbJob.CallbackUrl.Valid && dbJob.CallbackUrl.String != "" {
		go js.sendCallback(context.Background(), jobID, dbJob.CallbackUrl.String, results)
	}

	return nil
}

// ProcessJob is deprecated - use ProcessJobFromQueue instead
// This method is kept for API compatibility but immediately returns
func (js *JobService) ProcessJob(ctx context.Context, jobID string) error {
	js.logger.Info("ProcessJob called - job will be processed by queue workers",
		zap.String("job_id", jobID))

	// Just verify the job exists and is in pending state
	dbJob, err := js.db.GetQueries().GetScrapingJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	if dbJob.Status != db.JobStatusPending {
		return fmt.Errorf("job is not in pending status: %s", dbJob.Status)
	}

	// Job will be processed by RabbitMQ workers
	return nil
}

// GetScrapingJob retrieves a scraping job by ID
func (js *JobService) GetScrapingJob(ctx context.Context, jobID string) (*models.ScrapingJob, error) {
	js.logger.Debug("Getting scraping job", zap.String("job_id", jobID))

	dbJob, err := js.db.GetQueries().GetScrapingJob(ctx, jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found: %s", jobID)
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return js.convertDBJobToModel(dbJob)
}

// ListScrapingJobs retrieves a paginated list of scraping jobs
func (js *JobService) ListScrapingJobs(ctx context.Context, limit, offset int32) (*models.JobListResponse, error) {
	js.logger.Debug("Listing scraping jobs",
		zap.Int32("limit", limit),
		zap.Int32("offset", offset))

	// Get jobs
	dbJobs, err := js.db.GetQueries().ListScrapingJobs(ctx, db.ListScrapingJobsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Get total count
	totalCount, err := js.db.GetQueries().CountTotalJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count jobs: %w", err)
	}

	// Convert to response models
	var jobs []models.ScrapingJobResponse
	for _, dbJob := range dbJobs {
		job, err := js.convertDBJobToResponse(dbJob)
		if err != nil {
			js.logger.Warn("Failed to convert job", zap.String("job_id", dbJob.ID), zap.Error(err))
			continue
		}
		jobs = append(jobs, *job)
	}

	response := &models.JobListResponse{
		Jobs:       jobs,
		TotalCount: int(totalCount),
		Page:       int(offset/limit) + 1,
		PageSize:   int(limit),
		HasMore:    int64(offset+limit) < totalCount,
	}

	js.logger.Debug("Listed scraping jobs successfully",
		zap.Int("job_count", len(jobs)),
		zap.Int64("total_count", totalCount))

	return response, nil
}

// GetJobsByStatus retrieves jobs by status
func (js *JobService) GetJobsByStatus(ctx context.Context, status models.JobStatus, limit, offset int32) ([]models.ScrapingJob, error) {
	js.logger.Debug("Getting jobs by status",
		zap.String("status", string(status)),
		zap.Int32("limit", limit),
		zap.Int32("offset", offset))

	// Convert model status to DB status
	dbStatus, err := js.convertModelStatusToDB(status)
	if err != nil {
		return nil, fmt.Errorf("invalid status: %w", err)
	}

	dbJobs, err := js.db.GetQueries().ListScrapingJobsByStatus(ctx, db.ListScrapingJobsByStatusParams{
		Status: dbStatus,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs by status: %w", err)
	}

	var jobs []models.ScrapingJob
	for _, dbJob := range dbJobs {
		job, err := js.convertDBJobToModel(dbJob)
		if err != nil {
			js.logger.Warn("Failed to convert job", zap.String("job_id", dbJob.ID), zap.Error(err))
			continue
		}
		jobs = append(jobs, *job)
	}

	js.logger.Debug("Retrieved jobs by status",
		zap.String("status", string(status)),
		zap.Int("count", len(jobs)))

	return jobs, nil
}

// CancelJob cancels a scraping job
func (js *JobService) CancelJob(ctx context.Context, jobID string) error {
	js.logger.Info("Canceling job", zap.String("job_id", jobID))

	_, err := js.db.GetQueries().CancelJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	js.logger.Info("Job canceled successfully", zap.String("job_id", jobID))
	return nil
}

// RetryJob retries a failed job
func (js *JobService) RetryJob(ctx context.Context, jobID string) error {
	js.logger.Info("Retrying job", zap.String("job_id", jobID))

	// Get current job to check retry count
	dbJob, err := js.db.GetQueries().GetScrapingJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Check if job can be retried
	currentRetryCount := int32(0)
	if dbJob.RetryCount.Valid {
		currentRetryCount = dbJob.RetryCount.Int32
	}

	maxRetries := int32(js.config.MaxJobRetries)
	if currentRetryCount >= maxRetries {
		return fmt.Errorf("job has exceeded maximum retry count (%d)", maxRetries)
	}

	// Reset job to pending and increment retry count
	_, err = js.db.GetQueries().RetryJob(ctx, db.RetryJobParams{
		ID:         jobID,
		RetryCount: pgtype.Int4{Int32: maxRetries, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to retry job: %w", err)
	}

	// Republish to queue if queue manager is available
	if js.queueManager != nil {
		jobMsg := queue.JobMessage{
			JobID:     jobID,
			URL:       dbJob.Url,
			Priority:  5, // Default priority for retries
			Retry:     int(currentRetryCount + 1),
			CreatedAt: time.Now(),
		}

		if err := js.queueManager.PublishJob(ctx, jobMsg); err != nil {
			js.logger.Error("Failed to republish retry job to queue",
				zap.String("job_id", jobID),
				zap.Error(err))
			return fmt.Errorf("failed to republish job to queue: %w", err)
		}
	}

	js.logger.Info("Job retry initiated successfully",
		zap.String("job_id", jobID),
		zap.Int32("retry_count", currentRetryCount+1))

	return nil
}

// GetJobMetrics retrieves job processing metrics
func (js *JobService) GetJobMetrics(ctx context.Context) (*models.JobMetrics, error) {
	js.logger.Debug("Getting job metrics")

	dbMetrics, err := js.db.GetQueries().GetJobMetrics(ctx)
	if err != nil {
		js.logger.Error("Failed to get job metrics from database", zap.Error(err))
		return nil, fmt.Errorf("failed to get job metrics: %w", err)
	}

	// Convert to model
	metrics := &models.JobMetrics{
		TotalJobs:         dbMetrics.TotalJobs,
		PendingJobs:       dbMetrics.PendingJobs,
		ProcessingJobs:    dbMetrics.ProcessingJobs,
		CompletedJobs:     dbMetrics.CompletedJobs,
		FailedJobs:        dbMetrics.FailedJobs,
		SuccessRate:       0.0, // Default value
		AvgProcessingTime: 0,   // Default value
	}

	// Safely convert success rate
	if dbMetrics.SuccessRate.Valid {
		if successRateFloat, err := dbMetrics.SuccessRate.Float64Value(); err == nil {
			metrics.SuccessRate = successRateFloat.Float64
		} else {
			js.logger.Warn("Failed to convert success rate", zap.Error(err))
			metrics.SuccessRate = 0.0
		}
	}

	// Safely convert average processing time
	if dbMetrics.AvgProcessingTimeSeconds > 0 {
		metrics.AvgProcessingTime = time.Duration(dbMetrics.AvgProcessingTimeSeconds * float64(time.Second))
	}

	js.logger.Debug("Retrieved job metrics successfully",
		zap.Int64("total_jobs", metrics.TotalJobs),
		zap.Int64("pending_jobs", metrics.PendingJobs),
		zap.Int64("processing_jobs", metrics.ProcessingJobs),
		zap.Float64("success_rate", metrics.SuccessRate))

	return metrics, nil
}

// GetQueueStatus returns information about the job queue
func (js *JobService) GetQueueStatus(ctx context.Context) (*models.QueueStatus, error) {
	if js.queueManager == nil {
		return &models.QueueStatus{
			Available: false,
			Message:   "Queue manager not initialized",
		}, nil
	}

	queueSize, err := js.queueManager.GetQueueInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue info: %w", err)
	}

	// Get active jobs count from database
	activeJobs, err := js.db.GetQueries().GetActiveJobsCount(ctx)
	if err != nil {
		js.logger.Warn("Failed to get active jobs count", zap.Error(err))
		activeJobs = 0
	}

	return &models.QueueStatus{
		Available:    true,
		QueueSize:    queueSize,
		ActiveJobs:   int(activeJobs),
		HealthStatus: "healthy",
	}, nil
}

// CleanupOldJobs removes old completed/failed jobs
func (js *JobService) CleanupOldJobs(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-js.config.JobRetentionPeriod)

	js.logger.Info("Starting job cleanup",
		zap.Time("cutoff", cutoff),
		zap.Duration("retention_period", js.config.JobRetentionPeriod))

	// Convert time.Time to pgtype.Timestamp
	pgCutoff := pgtype.Timestamp{
		Time:  cutoff,
		Valid: true,
	}

	err := js.db.GetQueries().CleanupOldJobs(ctx, pgCutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup failed: %w", err)
	}

	// Get count of deleted jobs (this is a simplified approach)
	// In production, you might want to modify the SQL query to return the count
	js.logger.Info("Job cleanup completed")

	return 0, nil // TODO: Return actual count of deleted jobs
}

// Helper methods

func (js *JobService) updateJobProgress(ctx context.Context, jobID string, progress int) {
	_, err := js.db.GetQueries().UpdateJobProgress(ctx, db.UpdateJobProgressParams{
		ID:       jobID,
		Progress: pgtype.Int4{Int32: int32(progress), Valid: true},
	})
	if err != nil {
		js.logger.Warn("Failed to update job progress",
			zap.String("job_id", jobID),
			zap.Int("progress", progress),
			zap.Error(err))
	} else {
		js.logger.Debug("Updated job progress",
			zap.String("job_id", jobID),
			zap.Int("progress", progress))
	}
}

func (js *JobService) failJob(ctx context.Context, jobID, errorMsg string) error {
	js.logger.Error("Marking job as failed",
		zap.String("job_id", jobID),
		zap.String("error", errorMsg))

	_, err := js.db.GetQueries().FailJob(ctx, db.FailJobParams{
		ID:    jobID,
		Error: pgtype.Text{String: errorMsg, Valid: true},
	})
	if err != nil {
		js.logger.Error("Failed to mark job as failed",
			zap.String("job_id", jobID),
			zap.Error(err))
	}
	return err
}

func (js *JobService) sendCallback(ctx context.Context, jobID, callbackURL string, results *models.ScrapingResults) {
	js.logger.Info("Sending callback",
		zap.String("job_id", jobID),
		zap.String("callback_url", callbackURL))

	// Create completion time
	completedAt := time.Now()

	// Create callback payload
	payload := models.CallbackPayload{
		JobID:       jobID,
		Status:      models.JobStatusCompleted,
		Progress:    100,
		Results:     results,
		CompletedAt: &completedAt,
	}

	// Implement actual HTTP callback
	go js.sendHTTPCallback(ctx, callbackURL, payload)
}

// sendHTTPCallback sends an HTTP POST request to the callback URL
func (js *JobService) sendHTTPCallback(ctx context.Context, callbackURL string, payload models.CallbackPayload) {
	maxRetries := js.config.CallbackRetries
	timeout := js.config.CallbackTimeout

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create context with timeout
		callbackCtx, cancel := context.WithTimeout(ctx, timeout)

		// Marshal payload
		jsonData, err := json.Marshal(payload)
		if err != nil {
			js.logger.Error("Failed to marshal callback payload",
				zap.String("job_id", payload.JobID),
				zap.String("callback_url", callbackURL),
				zap.Error(err))
			cancel()
			return
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(callbackCtx, "POST", callbackURL, bytes.NewBuffer(jsonData))
		if err != nil {
			js.logger.Error("Failed to create callback request",
				zap.String("job_id", payload.JobID),
				zap.String("callback_url", callbackURL),
				zap.Error(err))
			cancel()
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Musli-Scraper-Service/1.0")

		// Send request
		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)

		if err != nil {
			js.logger.Warn("Callback request failed",
				zap.String("job_id", payload.JobID),
				zap.String("callback_url", callbackURL),
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", maxRetries),
				zap.Error(err))

			cancel()
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * time.Second) // Exponential backoff
				continue
			}
			return
		}

		// Check response status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			js.logger.Info("Callback sent successfully",
				zap.String("job_id", payload.JobID),
				zap.String("callback_url", callbackURL),
				zap.Int("status_code", resp.StatusCode))
			resp.Body.Close()
			cancel()
			return
		}

		js.logger.Warn("Callback returned non-2xx status",
			zap.String("job_id", payload.JobID),
			zap.String("callback_url", callbackURL),
			zap.Int("status_code", resp.StatusCode),
			zap.Int("attempt", attempt+1))

		resp.Body.Close()
		cancel()

		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * time.Second) // Exponential backoff
		}
	}

	js.logger.Error("Failed to send callback after all retries",
		zap.String("job_id", payload.JobID),
		zap.String("callback_url", callbackURL),
		zap.Int("max_retries", maxRetries))
}

func (js *JobService) convertDBJobToModel(dbJob db.ScrapingJobs) (*models.ScrapingJob, error) {
	job := &models.ScrapingJob{
		ID:     dbJob.ID,
		URL:    dbJob.Url,
		Status: js.convertDBStatusToModel(dbJob.Status),
	}

	// Handle timestamp fields
	if dbJob.CreatedAt.Valid {
		job.CreatedAt = dbJob.CreatedAt.Time
	}
	if dbJob.UpdatedAt.Valid {
		job.UpdatedAt = dbJob.UpdatedAt.Time
	}
	if dbJob.StartedAt.Valid {
		job.StartedAt = dbJob.StartedAt.Time
	}

	// Handle optional fields
	if dbJob.DatasourceID.Valid {
		job.DatasourceID = &dbJob.DatasourceID.Int32
	}

	if dbJob.CallbackUrl.Valid {
		job.CallbackURL = &dbJob.CallbackUrl.String
	}

	if dbJob.Progress.Valid {
		job.Progress = int(dbJob.Progress.Int32)
	}

	if dbJob.CompletedAt.Valid {
		job.CompletedAt = &dbJob.CompletedAt.Time
	}

	if dbJob.Error.Valid {
		job.Error = &dbJob.Error.String
	}

	if dbJob.RetryCount.Valid {
		job.RetryCount = int(dbJob.RetryCount.Int32)
	}

	// Parse JSON fields
	if len(dbJob.Options) > 0 {
		if err := json.Unmarshal(dbJob.Options, &job.Options); err != nil {
			return nil, fmt.Errorf("failed to unmarshal options: %w", err)
		}
	}

	if len(dbJob.Metadata) > 0 {
		if err := json.Unmarshal(dbJob.Metadata, &job.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	if len(dbJob.Results) > 0 {
		var results models.ScrapingResults
		if err := json.Unmarshal(dbJob.Results, &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results: %w", err)
		}
		job.Results = &results
	}

	return job, nil
}

func (js *JobService) convertDBJobToResponse(dbJob db.ScrapingJobs) (*models.ScrapingJobResponse, error) {
	job := &models.ScrapingJobResponse{
		ID:     dbJob.ID,
		Status: js.convertDBStatusToModel(dbJob.Status),
	}

	// Handle timestamp fields
	if dbJob.CreatedAt.Valid {
		job.CreatedAt = dbJob.CreatedAt.Time
	}
	if dbJob.UpdatedAt.Valid {
		job.UpdatedAt = dbJob.UpdatedAt.Time
	}
	if dbJob.StartedAt.Valid {
		job.StartedAt = dbJob.StartedAt.Time
	}

	if dbJob.Progress.Valid {
		job.Progress = int(dbJob.Progress.Int32)
	}

	if dbJob.CompletedAt.Valid {
		job.CompletedAt = &dbJob.CompletedAt.Time
	}

	if dbJob.Error.Valid {
		job.Error = &dbJob.Error.String
	}

	if len(dbJob.Results) > 0 {
		var results models.ScrapingResults
		if err := json.Unmarshal(dbJob.Results, &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results: %w", err)
		}
		job.Results = &results
	}

	return job, nil
}

func (js *JobService) convertDBStatusToModel(dbStatus db.JobStatus) models.JobStatus {
	switch dbStatus {
	case db.JobStatusPending:
		return models.JobStatusPending
	case db.JobStatusProcessing:
		return models.JobStatusProcessing
	case db.JobStatusCompleted:
		return models.JobStatusCompleted
	case db.JobStatusFailed:
		return models.JobStatusFailed
	case db.JobStatusCanceled:
		return models.JobStatusCanceled
	default:
		return models.JobStatusPending
	}
}

func (js *JobService) convertModelStatusToDB(modelStatus models.JobStatus) (db.JobStatus, error) {
	switch modelStatus {
	case models.JobStatusPending:
		return db.JobStatusPending, nil
	case models.JobStatusProcessing:
		return db.JobStatusProcessing, nil
	case models.JobStatusCompleted:
		return db.JobStatusCompleted, nil
	case models.JobStatusFailed:
		return db.JobStatusFailed, nil
	case models.JobStatusCanceled:
		return db.JobStatusCanceled, nil
	default:
		return "", fmt.Errorf("unknown job status: %s", modelStatus)
	}
}
