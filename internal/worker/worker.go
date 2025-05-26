// internal/worker/worker.go - Fixed to properly call JobService
package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/internal/queue"
	"github.com/mbaxamb3/nusli/scraper-service/internal/services"
	"go.uber.org/zap"
)

// RabbitMQWorkerPool manages multiple RabbitMQ workers
type RabbitMQWorkerPool struct {
	workers          []*RabbitMQWorker
	queueManager     *queue.RabbitMQManager
	jobService       *services.JobService
	config           *config.Config
	logger           *zap.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	healthChecker    *HealthChecker
	metricsCollector *WorkerMetrics

	// Worker pool state
	isRunning bool
	startTime time.Time
	mu        sync.RWMutex
}

// RabbitMQWorker represents a single worker that processes jobs from RabbitMQ
type RabbitMQWorker struct {
	id           int
	queueManager *queue.RabbitMQManager
	jobService   *services.JobService
	config       *config.Config
	logger       *zap.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           *sync.WaitGroup

	// Worker state
	isActive            bool
	lastJobTime         time.Time
	jobsProcessed       int64
	jobsSucceeded       int64
	jobsFailed          int64
	totalProcessingTime time.Duration
	mu                  sync.RWMutex
}

// WorkerMetrics collects metrics from all workers
type WorkerMetrics struct {
	mu                  sync.RWMutex
	totalJobs           int64
	successfulJobs      int64
	failedJobs          int64
	totalProcessingTime time.Duration
	activeWorkers       int
	startTime           time.Time
}

// HealthChecker monitors worker pool health
type HealthChecker struct {
	pool          *RabbitMQWorkerPool
	checkInterval time.Duration
	logger        *zap.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// WorkerStatus represents the status of a worker
type WorkerStatus struct {
	ID                int           `json:"id"`
	IsActive          bool          `json:"is_active"`
	LastJobTime       time.Time     `json:"last_job_time"`
	JobsProcessed     int64         `json:"jobs_processed"`
	JobsSucceeded     int64         `json:"jobs_succeeded"`
	JobsFailed        int64         `json:"jobs_failed"`
	SuccessRate       float64       `json:"success_rate"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
}

// PoolStatus represents the overall status of the worker pool
type PoolStatus struct {
	IsRunning          bool           `json:"is_running"`
	WorkerCount        int            `json:"worker_count"`
	ActiveWorkers      int            `json:"active_workers"`
	StartTime          time.Time      `json:"start_time"`
	Uptime             time.Duration  `json:"uptime"`
	TotalJobs          int64          `json:"total_jobs"`
	SuccessfulJobs     int64          `json:"successful_jobs"`
	FailedJobs         int64          `json:"failed_jobs"`
	OverallSuccessRate float64        `json:"overall_success_rate"`
	AvgProcessingTime  time.Duration  `json:"avg_processing_time"`
	Workers            []WorkerStatus `json:"workers"`
	QueueSize          int            `json:"queue_size,omitempty"`
	MemoryUsage        uint64         `json:"memory_usage_bytes"`
	GoroutineCount     int            `json:"goroutine_count"`
}

// NewRabbitMQWorkerPool creates a new RabbitMQ worker pool
func NewRabbitMQWorkerPool(jobService *services.JobService, queueManager *queue.RabbitMQManager,
	cfg *config.Config, logger *zap.Logger) *RabbitMQWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &RabbitMQWorkerPool{
		workers:      make([]*RabbitMQWorker, 0, cfg.WorkerCount),
		queueManager: queueManager,
		jobService:   jobService,
		config:       cfg,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		metricsCollector: &WorkerMetrics{
			startTime: time.Now(),
		},
	}

	// Initialize health checker
	pool.healthChecker = NewHealthChecker(pool, 30*time.Second, logger)

	return pool
}

// Start starts the RabbitMQ worker pool
func (rwp *RabbitMQWorkerPool) Start() error {
	rwp.mu.Lock()
	defer rwp.mu.Unlock()

	if rwp.isRunning {
		return fmt.Errorf("worker pool is already running")
	}

	rwp.logger.Info("Starting RabbitMQ worker pool",
		zap.Int("worker_count", rwp.config.WorkerCount),
		zap.String("queue_name", rwp.config.QueueName))

	rwp.startTime = time.Now()
	rwp.isRunning = true

	// Create and start workers
	for i := 0; i < rwp.config.WorkerCount; i++ {
		worker := rwp.createWorker(i)
		rwp.workers = append(rwp.workers, worker)
		rwp.wg.Add(1)
		go worker.run()
	}

	// Start health checker
	go rwp.healthChecker.Start()

	// Start metrics collection
	go rwp.collectMetrics()

	rwp.logger.Info("RabbitMQ worker pool started successfully",
		zap.Int("active_workers", len(rwp.workers)))

	return nil
}

// Stop stops the RabbitMQ worker pool gracefully
func (rwp *RabbitMQWorkerPool) Stop() error {
	rwp.mu.Lock()
	defer rwp.mu.Unlock()

	if !rwp.isRunning {
		return fmt.Errorf("worker pool is not running")
	}

	rwp.logger.Info("Stopping RabbitMQ worker pool...",
		zap.Int("worker_count", len(rwp.workers)))

	// Stop health checker first
	rwp.healthChecker.Stop()

	// Cancel context to signal workers to stop
	rwp.cancel()

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		rwp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		rwp.logger.Info("All RabbitMQ workers stopped gracefully")
	case <-time.After(rwp.config.WorkerShutdownTimeout):
		rwp.logger.Warn("RabbitMQ worker shutdown timeout exceeded",
			zap.Duration("timeout", rwp.config.WorkerShutdownTimeout))
	}

	rwp.isRunning = false
	rwp.logger.Info("RabbitMQ worker pool stopped")

	return nil
}

// GetStatus returns the current status of the worker pool
func (rwp *RabbitMQWorkerPool) GetStatus() *PoolStatus {
	rwp.mu.RLock()
	defer rwp.mu.RUnlock()

	status := &PoolStatus{
		IsRunning:      rwp.isRunning,
		WorkerCount:    len(rwp.workers),
		StartTime:      rwp.startTime,
		GoroutineCount: runtime.NumGoroutine(),
	}

	if rwp.isRunning {
		status.Uptime = time.Since(rwp.startTime)
	}

	// Collect metrics
	rwp.metricsCollector.mu.RLock()
	status.TotalJobs = rwp.metricsCollector.totalJobs
	status.SuccessfulJobs = rwp.metricsCollector.successfulJobs
	status.FailedJobs = rwp.metricsCollector.failedJobs
	status.ActiveWorkers = rwp.metricsCollector.activeWorkers
	if rwp.metricsCollector.totalJobs > 0 {
		status.OverallSuccessRate = float64(rwp.metricsCollector.successfulJobs) / float64(rwp.metricsCollector.totalJobs) * 100
	}
	if rwp.metricsCollector.successfulJobs > 0 {
		status.AvgProcessingTime = rwp.metricsCollector.totalProcessingTime / time.Duration(rwp.metricsCollector.successfulJobs)
	}
	rwp.metricsCollector.mu.RUnlock()

	// Get individual worker statuses
	status.Workers = make([]WorkerStatus, len(rwp.workers))
	for i, worker := range rwp.workers {
		status.Workers[i] = worker.getStatus()
	}

	// Get queue size if available
	if queueSize, err := rwp.queueManager.GetQueueInfo(); err == nil {
		status.QueueSize = queueSize
	}

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	status.MemoryUsage = memStats.Alloc

	return status
}

// createWorker creates a new RabbitMQ worker
func (rwp *RabbitMQWorkerPool) createWorker(id int) *RabbitMQWorker {
	ctx, cancel := context.WithCancel(rwp.ctx)

	return &RabbitMQWorker{
		id:           id,
		queueManager: rwp.queueManager,
		jobService:   rwp.jobService,
		config:       rwp.config,
		logger:       rwp.logger.With(zap.Int("worker_id", id)),
		ctx:          ctx,
		cancel:       cancel,
		wg:           &rwp.wg,
		lastJobTime:  time.Now(),
	}
}

// collectMetrics periodically collects metrics from all workers
func (rwp *RabbitMQWorkerPool) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rwp.ctx.Done():
			return
		case <-ticker.C:
			rwp.updateMetrics()
		}
	}
}

// updateMetrics updates the pool metrics by aggregating worker metrics
func (rwp *RabbitMQWorkerPool) updateMetrics() {
	rwp.metricsCollector.mu.Lock()
	defer rwp.metricsCollector.mu.Unlock()

	var totalJobs, successfulJobs, failedJobs int64
	var totalProcessingTime time.Duration
	var activeWorkers int

	for _, worker := range rwp.workers {
		worker.mu.RLock()
		totalJobs += worker.jobsProcessed
		successfulJobs += worker.jobsSucceeded
		failedJobs += worker.jobsFailed
		totalProcessingTime += worker.totalProcessingTime
		if worker.isActive {
			activeWorkers++
		}
		worker.mu.RUnlock()
	}

	rwp.metricsCollector.totalJobs = totalJobs
	rwp.metricsCollector.successfulJobs = successfulJobs
	rwp.metricsCollector.failedJobs = failedJobs
	rwp.metricsCollector.totalProcessingTime = totalProcessingTime
	rwp.metricsCollector.activeWorkers = activeWorkers
}

// RabbitMQWorker methods

// run is the main RabbitMQ worker loop
func (rw *RabbitMQWorker) run() {
	defer rw.wg.Done()
	defer rw.cancel()

	rw.logger.Info("RabbitMQ worker started")
	rw.setActive(true)

	// Create job handler
	handler := func(job queue.JobMessage) error {
		return rw.processJob(job)
	}

	// Start consuming jobs
	err := rw.queueManager.ConsumeJobs(rw.ctx, handler)
	if err != nil && err != context.Canceled {
		rw.logger.Error("Worker stopped due to error", zap.Error(err))
	}

	rw.setActive(false)
	rw.logger.Info("RabbitMQ worker stopped")
}

// processJob processes a single job from the queue
func (rw *RabbitMQWorker) processJob(job queue.JobMessage) error {
	startTime := time.Now()

	rw.logger.Info("Processing job from queue",
		zap.String("job_id", job.JobID),
		zap.String("url", job.URL),
		zap.Int("retry", job.Retry),
		zap.Int("priority", job.Priority))

	rw.updateJobStats(startTime, false, false) // Mark as started

	// Process the job using the job service's new method
	err := rw.jobService.ProcessJobFromQueue(rw.ctx, job.JobID)

	processingTime := time.Since(startTime)

	if err != nil {
		rw.logger.Error("Failed to process job from queue",
			zap.String("job_id", job.JobID),
			zap.String("url", job.URL),
			zap.Duration("processing_time", processingTime),
			zap.Error(err))

		rw.updateJobStats(startTime, true, false) // Mark as failed
		return fmt.Errorf("job processing failed: %w", err)
	}

	rw.logger.Info("Successfully processed job from queue",
		zap.String("job_id", job.JobID),
		zap.String("url", job.URL),
		zap.Duration("processing_time", processingTime))

	rw.updateJobStats(startTime, true, true) // Mark as completed successfully
	return nil
}

// updateJobStats updates the worker's job processing statistics
func (rw *RabbitMQWorker) updateJobStats(startTime time.Time, completed, succeeded bool) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if completed {
		rw.jobsProcessed++
		rw.totalProcessingTime += time.Since(startTime)

		if succeeded {
			rw.jobsSucceeded++
		} else {
			rw.jobsFailed++
		}
	}

	rw.lastJobTime = time.Now()
}

// setActive sets the worker's active status
func (rw *RabbitMQWorker) setActive(active bool) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.isActive = active
}

// getStatus returns the current status of the worker
func (rw *RabbitMQWorker) getStatus() WorkerStatus {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	status := WorkerStatus{
		ID:            rw.id,
		IsActive:      rw.isActive,
		LastJobTime:   rw.lastJobTime,
		JobsProcessed: rw.jobsProcessed,
		JobsSucceeded: rw.jobsSucceeded,
		JobsFailed:    rw.jobsFailed,
	}

	// Calculate success rate
	if rw.jobsProcessed > 0 {
		status.SuccessRate = float64(rw.jobsSucceeded) / float64(rw.jobsProcessed) * 100
	}

	// Calculate average processing time
	if rw.jobsSucceeded > 0 {
		status.AvgProcessingTime = rw.totalProcessingTime / time.Duration(rw.jobsSucceeded)
	}

	return status
}

// HealthChecker methods

// NewHealthChecker creates a new health checker for the worker pool
func NewHealthChecker(pool *RabbitMQWorkerPool, checkInterval time.Duration, logger *zap.Logger) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthChecker{
		pool:          pool,
		checkInterval: checkInterval,
		logger:        logger.With(zap.String("component", "health_checker")),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the health checker
func (hc *HealthChecker) Start() {
	hc.wg.Add(1)
	defer hc.wg.Done()

	hc.logger.Info("Health checker started", zap.Duration("check_interval", hc.checkInterval))

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			hc.logger.Info("Health checker stopping")
			return
		case <-ticker.C:
			hc.performHealthCheck()
		}
	}
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.logger.Info("Stopping health checker...")
	hc.cancel()
	hc.wg.Wait()
	hc.logger.Info("Health checker stopped")
}

// performHealthCheck performs a health check on the worker pool
func (hc *HealthChecker) performHealthCheck() {
	status := hc.pool.GetStatus()

	// Check if any workers are stuck (haven't processed a job in a long time)
	stuckWorkers := 0
	for _, worker := range status.Workers {
		if time.Since(worker.LastJobTime) > 5*time.Minute && worker.IsActive {
			stuckWorkers++
		}
	}

	// Log health status
	if stuckWorkers > 0 {
		hc.logger.Warn("Health check detected stuck workers",
			zap.Int("stuck_workers", stuckWorkers),
			zap.Int("total_workers", status.WorkerCount))
	}

	// Log overall metrics
	hc.logger.Debug("Worker pool health check",
		zap.Int("active_workers", status.ActiveWorkers),
		zap.Int("total_workers", status.WorkerCount),
		zap.Int64("total_jobs", status.TotalJobs),
		zap.Float64("success_rate", status.OverallSuccessRate),
		zap.Int("queue_size", status.QueueSize),
		zap.Uint64("memory_usage_mb", status.MemoryUsage/1024/1024),
		zap.Int("goroutines", status.GoroutineCount))

	// Check memory usage and log warning if too high
	memoryUsageMB := status.MemoryUsage / 1024 / 1024
	if memoryUsageMB > 500 {
		hc.logger.Warn("High memory usage detected",
			zap.Uint64("memory_usage_mb", memoryUsageMB))
	}

	// Check goroutine count
	if status.GoroutineCount > 100 {
		hc.logger.Warn("High goroutine count detected",
			zap.Int("goroutine_count", status.GoroutineCount))
	}
}

// Shutdown performs a graceful shutdown of the worker pool
func (rwp *RabbitMQWorkerPool) Shutdown(ctx context.Context) error {
	rwp.logger.Info("Initiating graceful shutdown of worker pool")

	// Create a channel to signal completion
	done := make(chan error, 1)

	go func() {
		done <- rwp.Stop()
	}()

	// Wait for shutdown or context timeout
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		rwp.logger.Warn("Shutdown timeout exceeded, forcing stop")
		rwp.cancel() // Force cancel all workers
		return ctx.Err()
	}
}
