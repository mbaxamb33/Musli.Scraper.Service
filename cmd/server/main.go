// cmd/server/main.go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/internal/handlers"
	"github.com/mbaxamb3/nusli/scraper-service/internal/queue"
	"github.com/mbaxamb3/nusli/scraper-service/internal/scraper"
	"github.com/mbaxamb3/nusli/scraper-service/internal/services"
	"github.com/mbaxamb3/nusli/scraper-service/internal/storage"
	"github.com/mbaxamb3/nusli/scraper-service/internal/worker"
	"go.uber.org/zap"
)

const (
	ServiceName    = "Musli.Scraper.Service"
	ServiceVersion = "1.0.0"
)

func main() {
	// Initialize logger
	logger, err := initLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting "+ServiceName,
		zap.String("version", ServiceVersion),
		zap.String("go_version", "1.24.2"))

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.Info("Configuration loaded successfully",
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
		zap.String("db_type", cfg.JobDBType),
		zap.String("rabbitmq_url", maskCredentials(cfg.RabbitMQURL)),
		zap.Int("worker_count", cfg.WorkerCount),
		zap.Bool("browser_headless", cfg.BrowserHeadless))

	// Initialize database
	logger.Info("Connecting to database...")
	database, err := storage.NewDatabase(cfg.GetDSN(), logger)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.Close()
	logger.Info("Database connection established successfully")

	// Initialize scraper engine
	logger.Info("Initializing scraper engine...")
	scraperEngine, err := scraper.NewEngine(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize scraper engine", zap.Error(err))
	}
	defer func() {
		if err := scraperEngine.Close(); err != nil {
			logger.Error("Error closing scraper engine", zap.Error(err))
		}
	}()
	logger.Info("Scraper engine initialized successfully")

	// Initialize RabbitMQ queue manager
	logger.Info("Connecting to RabbitMQ...")
	queueManager, err := queue.NewRabbitMQManager(
		cfg.RabbitMQURL,
		cfg.QueueName,
		cfg.DeadLetterQueue,
		cfg.ExchangeName,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to initialize RabbitMQ", zap.Error(err))
	}
	defer func() {
		if err := queueManager.Close(); err != nil {
			logger.Error("Error closing RabbitMQ connection", zap.Error(err))
		}
	}()
	logger.Info("RabbitMQ connection established successfully",
		zap.String("queue_name", cfg.QueueName),
		zap.String("exchange_name", cfg.ExchangeName))

	// Initialize services
	logger.Info("Initializing services...")
	jobService := services.NewJobService(database, scraperEngine, cfg, logger, queueManager)
	logger.Info("Job service initialized successfully")

	// Initialize handlers
	jobHandler := handlers.NewJobHandler(jobService, logger)
	healthHandler := handlers.NewHealthHandler(database, scraperEngine, logger)

	// Initialize RabbitMQ worker pool
	logger.Info("Initializing worker pool...")
	workerPool := worker.NewRabbitMQWorkerPool(jobService, queueManager, cfg, logger)

	// Start worker pool
	logger.Info("Starting worker pool...")
	if err := workerPool.Start(); err != nil {
		logger.Fatal("Failed to start worker pool", zap.Error(err))
	}
	defer func() {
		logger.Info("Stopping worker pool...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.WorkerShutdownTimeout)
		defer cancel()

		if err := workerPool.Shutdown(shutdownCtx); err != nil {
			logger.Error("Error stopping worker pool", zap.Error(err))
		} else {
			logger.Info("Worker pool stopped successfully")
		}
	}()
	logger.Info("Worker pool started successfully")

	// Setup HTTP routes
	mux := setupRoutes(jobHandler, healthHandler, workerPool, logger)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start HTTP server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Starting HTTP server",
			zap.String("addr", httpServer.Addr),
			zap.Duration("read_timeout", cfg.ReadTimeout),
			zap.Duration("write_timeout", cfg.WriteTimeout))

		serverErrors <- httpServer.ListenAndServe()
	}()

	// Start background tasks
	if cfg.EnableAutoCleanup {
		go startCleanupTask(jobService, cfg, logger)
	}

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("Server error", zap.Error(err))
	case sig := <-quit:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	}

	logger.Info("Shutting down " + ServiceName + "...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Graceful shutdown of HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during HTTP server shutdown", zap.Error(err))
	} else {
		logger.Info("HTTP server stopped gracefully")
	}

	logger.Info(ServiceName + " stopped gracefully")
}

// setupRoutes configures all HTTP routes
func setupRoutes(jobHandler *handlers.JobHandler, healthHandler *handlers.HealthHandler,
	workerPool *worker.RabbitMQWorkerPool, logger *zap.Logger) *http.ServeMux {

	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/ready", healthHandler.Ready)
	mux.HandleFunc("/live", healthHandler.Live)

	// Job management endpoints
	mux.HandleFunc("/api/jobs", handleJobRoutes(jobHandler))
	mux.HandleFunc("/api/jobs/", handleJobRoutesWithID(jobHandler))
	mux.HandleFunc("/api/metrics", jobHandler.GetMetrics)

	// Worker status endpoints
	mux.HandleFunc("/api/workers/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := workerPool.GetStatus()
		w.Header().Set("Content-Type", "application/json")

		if err := writeJSON(w, status); err != nil {
			logger.Error("Failed to write worker status response", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	})

	// Queue status endpoint
	mux.HandleFunc("/api/queue/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// This would be implemented in your job service
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","message":"Queue status endpoint"}`))
	})

	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"service": ServiceName,
			"version": ServiceVersion,
			"status":  "running",
			"endpoints": map[string]string{
				"health":        "/health",
				"jobs":          "/api/jobs",
				"metrics":       "/api/metrics",
				"worker_status": "/api/workers/status",
				"queue_status":  "/api/queue/status",
			},
		}
		writeJSON(w, response)
	})

	// Add middleware for logging and CORS
	return addMiddleware(mux, logger)
}

// handleJobRoutes handles routes for /api/jobs
func handleJobRoutes(jobHandler *handlers.JobHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			jobHandler.CreateJob(w, r)
		case http.MethodGet:
			jobHandler.ListJobs(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleJobRoutesWithID handles routes for /api/jobs/{id} and /api/jobs/{id}/process
func handleJobRoutesWithID(jobHandler *handlers.JobHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Check if it's a process endpoint
		if len(path) > 15 && path[len(path)-8:] == "/process" {
			if r.Method == http.MethodPost {
				jobHandler.ProcessJob(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Regular job endpoints
		switch r.Method {
		case http.MethodGet:
			jobHandler.GetJob(w, r)
		case http.MethodDelete:
			jobHandler.CancelJob(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// addMiddleware adds logging and CORS middleware
func addMiddleware(handler http.Handler, logger *zap.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/", loggingMiddleware(corsMiddleware(handler), logger))

	return mux
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler, logger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures the status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		logger.Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
			zap.Int("status", wrapped.statusCode),
			zap.Duration("duration", duration))
	})
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// startCleanupTask starts a background task for cleaning up old jobs
func startCleanupTask(jobService *services.JobService, cfg *config.Config, logger *zap.Logger) {
	ticker := time.NewTicker(cfg.JobCleanupInterval)
	defer ticker.Stop()

	logger.Info("Started cleanup task",
		zap.Duration("interval", cfg.JobCleanupInterval),
		zap.Duration("retention_period", cfg.JobRetentionPeriod))

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

		count, err := jobService.CleanupOldJobs(ctx)
		if err != nil {
			logger.Error("Job cleanup failed", zap.Error(err))
		} else {
			logger.Info("Job cleanup completed", zap.Int64("jobs_cleaned", count))
		}

		cancel()
	}
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// maskCredentials masks sensitive information in URLs
func maskCredentials(url string) string {
	// Simple credential masking for logs
	// In production, use a more robust solution
	if len(url) > 20 {
		return url[:10] + "***" + url[len(url)-7:]
	}
	return "***"
}

// initLogger initializes the zap logger based on environment
func initLogger() (*zap.Logger, error) {
	env := os.Getenv("ENVIRONMENT")

	switch env {
	case "production", "prod":
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		return config.Build()
	case "development", "dev":
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		return config.Build()
	default:
		// Default to development for unknown environments
		return zap.NewDevelopment()
	}
}
