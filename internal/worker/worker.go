// internal/worker/worker.go
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/internal/services"
	"go.uber.org/zap"
)

type Worker struct {
	id         int
	jobService *services.JobService
	config     *config.Config
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         *sync.WaitGroup
}

type WorkerPool struct {
	workers    []*Worker
	jobService *services.JobService
	config     *config.Config
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(jobService *services.JobService, cfg *config.Config, logger *zap.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workers:    make([]*Worker, 0, cfg.WorkerCount),
		jobService: jobService,
		config:     cfg,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	wp.logger.Info("Starting worker pool", zap.Int("worker_count", wp.config.WorkerCount))

	for i := 0; i < wp.config.WorkerCount; i++ {
		worker := wp.createWorker(i)
		wp.workers = append(wp.workers, worker)
		wp.wg.Add(1)
		go worker.run()
	}

	wp.logger.Info("Worker pool started successfully")
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop() {
	wp.logger.Info("Stopping worker pool...")

	// Cancel context to signal workers to stop
	wp.cancel()

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wp.logger.Info("All workers stopped gracefully")
	case <-time.After(wp.config.WorkerShutdownTimeout):
		wp.logger.Warn("Worker shutdown timeout exceeded")
	}
}

// createWorker creates a new worker instance
func (wp *WorkerPool) createWorker(id int) *Worker {
	ctx, cancel := context.WithCancel(wp.ctx)

	return &Worker{
		id:         id,
		jobService: wp.jobService,
		config:     wp.config,
		logger:     wp.logger.With(zap.Int("worker_id", id)),
		ctx:        ctx,
		cancel:     cancel,
		wg:         &wp.wg,
	}
}

// run is the main worker loop
func (w *Worker) run() {
	defer w.wg.Done()
	defer w.cancel()

	w.logger.Info("Worker started")

	ticker := time.NewTicker(5 * time.Second) // Check for jobs every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("Worker stopping")
			return
		case <-ticker.C:
			if err := w.processNextJob(); err != nil {
				w.logger.Error("Error processing job", zap.Error(err))
			}
		}
	}
}

// processNextJob finds and processes the next pending job
func (w *Worker) processNextJob() error {
	// Get pending jobs (limit 1 for this worker)
	jobs, err := w.jobService.GetJobsByStatus(w.ctx, "pending", 1, 0)
	if err != nil {
		return err
	}

	if len(jobs) == 0 {
		// No pending jobs
		return nil
	}

	job := jobs[0]
	w.logger.Info("Processing job", zap.String("job_id", job.ID), zap.String("url", job.URL))

	// Process the job
	if err := w.jobService.ProcessJob(w.ctx, job.ID); err != nil {
		w.logger.Error("Failed to process job",
			zap.String("job_id", job.ID),
			zap.Error(err))
		return err
	}

	return nil
}

// JobQueue represents a simple in-memory job queue
type JobQueue struct {
	jobs   chan string
	logger *zap.Logger
}

// NewJobQueue creates a new job queue
func NewJobQueue(size int, logger *zap.Logger) *JobQueue {
	return &JobQueue{
		jobs:   make(chan string, size),
		logger: logger,
	}
}

// Enqueue adds a job to the queue
func (jq *JobQueue) Enqueue(jobID string) error {
	select {
	case jq.jobs <- jobID:
		jq.logger.Debug("Job enqueued", zap.String("job_id", jobID))
		return nil
	default:
		return ErrQueueFull
	}
}

// Dequeue removes a job from the queue
func (jq *JobQueue) Dequeue(ctx context.Context) (string, error) {
	select {
	case jobID := <-jq.jobs:
		return jobID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Size returns the current queue size
func (jq *JobQueue) Size() int {
	return len(jq.jobs)
}

// Enhanced Worker with Queue Support
type QueueWorker struct {
	id         int
	queue      *JobQueue
	jobService *services.JobService
	config     *config.Config
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         *sync.WaitGroup
}

type QueueWorkerPool struct {
	workers    []*QueueWorker
	queue      *JobQueue
	jobService *services.JobService
	config     *config.Config
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewQueueWorkerPool creates a worker pool with job queue
func NewQueueWorkerPool(jobService *services.JobService, cfg *config.Config, logger *zap.Logger) *QueueWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	queue := NewJobQueue(1000, logger) // Queue can hold 1000 jobs

	return &QueueWorkerPool{
		workers:    make([]*QueueWorker, 0, cfg.WorkerCount),
		queue:      queue,
		jobService: jobService,
		config:     cfg,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the queue-based worker pool
func (qwp *QueueWorkerPool) Start() {
	qwp.logger.Info("Starting queue worker pool", zap.Int("worker_count", qwp.config.WorkerCount))

	for i := 0; i < qwp.config.WorkerCount; i++ {
		worker := qwp.createQueueWorker(i)
		qwp.workers = append(qwp.workers, worker)
		qwp.wg.Add(1)
		go worker.run()
	}

	qwp.logger.Info("Queue worker pool started successfully")
}

// Stop stops the queue worker pool
func (qwp *QueueWorkerPool) Stop() {
	qwp.logger.Info("Stopping queue worker pool...")
	qwp.cancel()

	done := make(chan struct{})
	go func() {
		qwp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		qwp.logger.Info("All queue workers stopped gracefully")
	case <-time.After(qwp.config.WorkerShutdownTimeout):
		qwp.logger.Warn("Queue worker shutdown timeout exceeded")
	}
}

// EnqueueJob adds a job to the processing queue
func (qwp *QueueWorkerPool) EnqueueJob(jobID string) error {
	return qwp.queue.Enqueue(jobID)
}

// GetQueueSize returns current queue size
func (qwp *QueueWorkerPool) GetQueueSize() int {
	return qwp.queue.Size()
}

// createQueueWorker creates a new queue worker
func (qwp *QueueWorkerPool) createQueueWorker(id int) *QueueWorker {
	ctx, cancel := context.WithCancel(qwp.ctx)

	return &QueueWorker{
		id:         id,
		queue:      qwp.queue,
		jobService: qwp.jobService,
		config:     qwp.config,
		logger:     qwp.logger.With(zap.Int("worker_id", id)),
		ctx:        ctx,
		cancel:     cancel,
		wg:         &qwp.wg,
	}
}

// run is the main queue worker loop
func (qw *QueueWorker) run() {
	defer qw.wg.Done()
	defer qw.cancel()

	qw.logger.Info("Queue worker started")

	for {
		select {
		case <-qw.ctx.Done():
			qw.logger.Info("Queue worker stopping")
			return
		default:
			if err := qw.processQueuedJob(); err != nil {
				qw.logger.Error("Error processing queued job", zap.Error(err))
				// Brief pause on error to avoid tight error loops
				time.Sleep(time.Second)
			}
		}
	}
}

// processQueuedJob processes a job from the queue
func (qw *QueueWorker) processQueuedJob() error {
	// Wait for job with timeout
	ctx, cancel := context.WithTimeout(qw.ctx, 30*time.Second)
	defer cancel()

	jobID, err := qw.queue.Dequeue(ctx)
	if err != nil {
		if err == context.DeadlineExceeded {
			// Timeout is normal, not an error
			return nil
		}
		return err
	}

	qw.logger.Info("Processing queued job", zap.String("job_id", jobID))

	// Process the job
	if err := qw.jobService.ProcessJob(qw.ctx, jobID); err != nil {
		qw.logger.Error("Failed to process queued job",
			zap.String("job_id", jobID),
			zap.Error(err))
		return err
	}

	return nil
}

// Custom errors
var (
	ErrQueueFull = fmt.Errorf("job queue is full")
)
