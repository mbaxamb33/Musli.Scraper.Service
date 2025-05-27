// job_bridge_service.go - Bridge between database jobs and RabbitMQ workers
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/internal/queue"
	"github.com/mbaxamb3/nusli/scraper-service/internal/storage"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("🔗 Job Bridge Service - Connecting Database Jobs to Workers")
	fmt.Println("=========================================================")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize database
	database, err := storage.NewDatabase(cfg.GetDSN(), logger)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	// Initialize RabbitMQ
	queueManager, err := queue.NewRabbitMQManager(
		cfg.RabbitMQURL,
		cfg.QueueName,
		cfg.DeadLetterQueue,
		cfg.ExchangeName,
		logger,
	)
	if err != nil {
		log.Fatal("Failed to initialize RabbitMQ:", err)
	}
	defer queueManager.Close()

	fmt.Println("✅ Connected to database and RabbitMQ")

	// Run the bridge loop
	ctx := context.Background()

	for {
		// Get pending jobs from database
		jobs, err := database.GetQueries().ListPendingJobs(ctx, 10)
		if err != nil {
			logger.Error("Failed to get pending jobs", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		if len(jobs) > 0 {
			logger.Info("Found pending jobs", zap.Int("count", len(jobs)))

			for _, job := range jobs {
				// Handle timestamp conversion
				var createdAt time.Time
				if job.CreatedAt.Valid {
					createdAt = job.CreatedAt.Time
				} else {
					createdAt = time.Now()
				}

				// Create job message for RabbitMQ
				jobMsg := queue.JobMessage{
					JobID:     job.ID,
					URL:       job.Url,
					Priority:  5, // Default priority
					CreatedAt: createdAt,
				}

				// Publish to RabbitMQ
				if err := queueManager.PublishJob(ctx, jobMsg); err != nil {
					logger.Error("Failed to publish job to queue",
						zap.String("job_id", job.ID),
						zap.Error(err))
				} else {
					logger.Info("Published job to queue",
						zap.String("job_id", job.ID),
						zap.String("url", job.Url))
				}
			}
		} else {
			logger.Debug("No pending jobs found")
		}

		// Wait before checking again
		time.Sleep(10 * time.Second)
	}
}
