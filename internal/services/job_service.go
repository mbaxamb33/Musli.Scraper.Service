// internal/services/job_service.go
package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/mbaxamb3/nusli/scraper-service/db/sqlc"
	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/internal/scraper"
	"github.com/mbaxamb3/nusli/scraper-service/internal/storage"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

// JobService handles scraping job operations
type JobService struct {
	db      *storage.Database
	scraper *scraper.Engine
	config  *config.Config
	logger  *zap.Logger
}

// NewJobService creates a new job service instance
func NewJobService(db *storage.Database, scraperEngine *scraper.Engine, cfg *config.Config, logger *zap.Logger) *JobService {
	return &JobService{
		db:      db,
		scraper: scraperEngine,
		config:  cfg,
		logger:  logger,
	}
}

// CreateScrapingJob creates a new scraping job
func (js *JobService) CreateScrapingJob(ctx context.Context, req models.ScrapingJobRequest) (*models.ScrapingJob, error) {
	// Generate unique job ID
	jobID := uuid.New().String()

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

	js.logger.Info("Created scraping job",
		zap.String("job_id", jobID),
		zap.String("url", req.URL))

	return job, nil
}

// GetScrapingJob retrieves a scraping job by ID
func (js *JobService) GetScrapingJob(ctx context.Context, jobID string) (*models.ScrapingJob, error) {
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

	return response, nil
}

// ProcessJob processes a scraping job
func (js *JobService) ProcessJob(ctx context.Context, jobID string) error {
	// Get job from database
	dbJob, err := js.db.GetQueries().GetScrapingJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Check if job is in pending status
	if dbJob.Status != db.JobStatusPending {
		return fmt.Errorf("job is not in pending status: %s", dbJob.Status)
	}

	// Start the job
	_, err = js.db.GetQueries().StartJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to start job: %w", err)
	}

	js.logger.Info("Started processing job", zap.String("job_id", jobID), zap.String("url", dbJob.Url))

	// Process in background
	go js.processJobAsync(context.Background(), jobID, dbJob)

	return nil
}

// processJobAsync processes a job asynchronously
func (js *JobService) processJobAsync(ctx context.Context, jobID string, dbJob db.ScrapingJobs) {
	defer func() {
		if r := recover(); r != nil {
			js.logger.Error("Job processing panicked",
				zap.String("job_id", jobID),
				zap.Any("panic", r))

			// Mark job as failed
			js.failJob(ctx, jobID, fmt.Sprintf("Processing panicked: %v", r))
		}
	}()

	// Parse options
	var options models.ScrapingOptions
	if err := json.Unmarshal(dbJob.Options, &options); err != nil {
		js.logger.Error("Failed to parse job options",
			zap.String("job_id", jobID),
			zap.Error(err))
		js.failJob(ctx, jobID, fmt.Sprintf("Failed to parse options: %v", err))
		return
	}

	// Update progress
	js.updateJobProgress(ctx, jobID, 10)

	// Perform scraping
	results, err := js.scraper.ScrapePage(ctx, dbJob.Url, options)
	if err != nil {
		js.logger.Error("Failed to scrape page",
			zap.String("job_id", jobID),
			zap.String("url", dbJob.Url),
			zap.Error(err))
		js.failJob(ctx, jobID, err.Error())
		return
	}

	// Update progress
	js.updateJobProgress(ctx, jobID, 90)

	// Convert results to JSON
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		js.logger.Error("Failed to marshal results",
			zap.String("job_id", jobID),
			zap.Error(err))
		js.failJob(ctx, jobID, fmt.Sprintf("Failed to marshal results: %v", err))
		return
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
		return
	}

	js.logger.Info("Job completed successfully",
		zap.String("job_id", jobID),
		zap.String("url", dbJob.Url),
		zap.Int("modules_extracted", len(results.ModulePairs)))

	// Send callback if configured
	if dbJob.CallbackUrl.Valid && dbJob.CallbackUrl.String != "" {
		js.sendCallback(ctx, jobID, dbJob.CallbackUrl.String, results)
	}
}

// GetJobsByStatus retrieves jobs by status
func (js *JobService) GetJobsByStatus(ctx context.Context, status models.JobStatus, limit, offset int32) ([]models.ScrapingJob, error) {
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

	return jobs, nil
}

// CancelJob cancels a scraping job
func (js *JobService) CancelJob(ctx context.Context, jobID string) error {
	_, err := js.db.GetQueries().CancelJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	js.logger.Info("Job canceled", zap.String("job_id", jobID))
	return nil
}

// GetJobMetrics retrieves job processing metrics
func (js *JobService) GetJobMetrics(ctx context.Context) (*models.JobMetrics, error) {
	dbMetrics, err := js.db.GetQueries().GetJobMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get job metrics: %w", err)
	}

	// Convert to model
	metrics := &models.JobMetrics{
		TotalJobs:      dbMetrics.TotalJobs,
		PendingJobs:    dbMetrics.PendingJobs,
		ProcessingJobs: dbMetrics.ProcessingJobs,
		CompletedJobs:  dbMetrics.CompletedJobs,
		FailedJobs:     dbMetrics.FailedJobs,
	}

	// Convert success rate
	if dbMetrics.SuccessRate.Valid {
		successRateFloat, _ := dbMetrics.SuccessRate.Float64Value()
		metrics.SuccessRate = successRateFloat.Float64
	}

	// Convert average processing time
	if dbMetrics.AvgProcessingTimeSeconds > 0 {
		metrics.AvgProcessingTime = time.Duration(dbMetrics.AvgProcessingTimeSeconds * float64(time.Second))
	}

	return metrics, nil
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
	}
}

func (js *JobService) failJob(ctx context.Context, jobID, errorMsg string) {
	_, err := js.db.GetQueries().FailJob(ctx, db.FailJobParams{
		ID:    jobID,
		Error: pgtype.Text{String: errorMsg, Valid: true},
	})
	if err != nil {
		js.logger.Error("Failed to mark job as failed",
			zap.String("job_id", jobID),
			zap.Error(err))
	}
}

func (js *JobService) sendCallback(ctx context.Context, jobID, callbackURL string, results *models.ScrapingResults) {
	// TODO: Implement HTTP callback
	js.logger.Info("Callback would be sent",
		zap.String("job_id", jobID),
		zap.String("callback_url", callbackURL))
}

// Conversion helpers

func (js *JobService) convertDBJobToModel(dbJob db.ScrapingJobs) (*models.ScrapingJob, error) {
	job := &models.ScrapingJob{
		ID:        dbJob.ID,
		URL:       dbJob.Url,
		Status:    js.convertDBStatusToModel(dbJob.Status),
		CreatedAt: dbJob.CreatedAt,
		UpdatedAt: dbJob.UpdatedAt,
		StartedAt: dbJob.StartedAt,
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

	if dbJob.CompletedAt != nil && *dbJob.CompletedAt != nil {
		job.CompletedAt = *dbJob.CompletedAt
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
		ID:        dbJob.ID,
		Status:    js.convertDBStatusToModel(dbJob.Status),
		CreatedAt: dbJob.CreatedAt,
		UpdatedAt: dbJob.UpdatedAt,
		StartedAt: dbJob.StartedAt,
	}

	if dbJob.Progress.Valid {
		job.Progress = int(dbJob.Progress.Int32)
	}

	if dbJob.CompletedAt != nil && *dbJob.CompletedAt != nil {
		job.CompletedAt = *dbJob.CompletedAt
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
