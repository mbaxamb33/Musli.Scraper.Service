// internal/config/config.go
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

// Config holds all configuration for the scraper service
type Config struct {
	// Server Configuration
	Port         string        `env:"PORT" envDefault:"8081"`
	Environment  string        `env:"ENVIRONMENT" envDefault:"development"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s"`

	// Database Configuration (for job storage only)
	JobDBType string `env:"JOB_DB_TYPE" envDefault:"postgres"` // sqlite, postgres
	JobDBURL  string `env:"JOB_DB_URL" envDefault:"postgresql://root:secret@localhost:5433/musli_scraper?sslmode=disable"`

	// RabbitMQ Configuration
	RabbitMQURL     string `env:"RABBITMQ_URL" envDefault:"amqp://guest:guest@localhost:5672/"`
	QueueName       string `env:"QUEUE_NAME" envDefault:"scraping_jobs"`
	DeadLetterQueue string `env:"DEAD_LETTER_QUEUE" envDefault:"scraping_jobs_dlq"`
	ExchangeName    string `env:"EXCHANGE_NAME" envDefault:"scraping_exchange"`
	QueueMaxRetries int    `env:"QUEUE_MAX_RETRIES" envDefault:"3"`

	// Queue Configuration
	QueueDurable    bool `env:"QUEUE_DURABLE" envDefault:"true"`
	QueueAutoDelete bool `env:"QUEUE_AUTO_DELETE" envDefault:"false"`
	QueueExclusive  bool `env:"QUEUE_EXCLUSIVE" envDefault:"false"`

	// Browser/Rod Configuration
	BrowserHeadless       bool          `env:"BROWSER_HEADLESS" envDefault:"true"`
	BrowserTimeout        time.Duration `env:"BROWSER_TIMEOUT" envDefault:"60s"`
	BrowserUserAgent      string        `env:"BROWSER_USER_AGENT" envDefault:"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"`
	BrowserViewportWidth  int           `env:"BROWSER_VIEWPORT_WIDTH" envDefault:"1920"`
	BrowserViewportHeight int           `env:"BROWSER_VIEWPORT_HEIGHT" envDefault:"1080"`
	BrowserIncognito      bool          `env:"BROWSER_INCOGNITO" envDefault:"true"`

	// Scraping Configuration
	MaxConcurrency int           `env:"MAX_CONCURRENCY" envDefault:"5"`
	RequestDelay   time.Duration `env:"REQUEST_DELAY" envDefault:"1s"`
	MaxRetries     int           `env:"MAX_RETRIES" envDefault:"3"`
	ContentTimeout time.Duration `env:"CONTENT_TIMEOUT" envDefault:"30s"`
	MaxPageSize    int64         `env:"MAX_PAGE_SIZE" envDefault:"10485760"` // 10MB
	PageLoadWait   time.Duration `env:"PAGE_LOAD_WAIT" envDefault:"3s"`

	// Worker Configuration
	WorkerCount           int           `env:"WORKER_COUNT" envDefault:"3"`
	JobTimeout            time.Duration `env:"JOB_TIMEOUT" envDefault:"300s"` // 5 minutes
	JobRetryDelay         time.Duration `env:"JOB_RETRY_DELAY" envDefault:"30s"`
	MaxJobRetries         int           `env:"MAX_JOB_RETRIES" envDefault:"3"`
	WorkerShutdownTimeout time.Duration `env:"WORKER_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	// Content Processing Configuration
	MinContentLength           int     `env:"MIN_CONTENT_LENGTH" envDefault:"50"`
	MaxContentLength           int     `env:"MAX_CONTENT_LENGTH" envDefault:"50000"`
	MinWordsPerModule          int     `env:"MIN_WORDS_PER_MODULE" envDefault:"10"`
	ContentSimilarityThreshold float64 `env:"CONTENT_SIMILARITY_THRESHOLD" envDefault:"0.8"`
	MaxNestingLevel            int     `env:"MAX_NESTING_LEVEL" envDefault:"6"`

	// Feature Flags
	EnableSemanticTagging   bool `env:"ENABLE_SEMANTIC_TAGGING" envDefault:"true"`
	EnableKeyTermExtraction bool `env:"ENABLE_KEY_TERM_EXTRACTION" envDefault:"true"`
	EnableCrossReferences   bool `env:"ENABLE_CROSS_REFERENCES" envDefault:"false"`
	EnableVisualAnalysis    bool `env:"ENABLE_VISUAL_ANALYSIS" envDefault:"true"`
	EnableScreenshots       bool `env:"ENABLE_SCREENSHOTS" envDefault:"false"`

	// API Integration
	MainAPIURL      string        `env:"MAIN_API_URL" envDefault:"http://localhost:8080"`
	APIKey          string        `env:"API_KEY"`
	CallbackRetries int           `env:"CALLBACK_RETRIES" envDefault:"3"`
	CallbackTimeout time.Duration `env:"CALLBACK_TIMEOUT" envDefault:"10s"`

	// Logging
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// Health & Monitoring
	HealthCheckInterval time.Duration `env:"HEALTH_CHECK_INTERVAL" envDefault:"30s"`
	MetricsEnabled      bool          `env:"METRICS_ENABLED" envDefault:"true"`
	MetricsPath         string        `env:"METRICS_PATH" envDefault:"/metrics"`

	// Cleanup & Maintenance
	JobCleanupInterval time.Duration `env:"JOB_CLEANUP_INTERVAL" envDefault:"1h"`
	JobRetentionPeriod time.Duration `env:"JOB_RETENTION_PERIOD" envDefault:"168h"` // 7 days
	EnableAutoCleanup  bool          `env:"ENABLE_AUTO_CLEANUP" envDefault:"true"`

	// Rate Limiting
	EnableRateLimiting bool          `env:"ENABLE_RATE_LIMITING" envDefault:"true"`
	RateLimit          int           `env:"RATE_LIMIT" envDefault:"100"` // requests per minute
	RateLimitWindow    time.Duration `env:"RATE_LIMIT_WINDOW" envDefault:"1m"`

	// Security
	EnableCORS     bool   `env:"ENABLE_CORS" envDefault:"true"`
	AllowedOrigins string `env:"ALLOWED_ORIGINS" envDefault:"*"`
	EnableAuth     bool   `env:"ENABLE_AUTH" envDefault:"false"`
	AuthSecret     string `env:"AUTH_SECRET"`
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT cannot be empty")
	}

	if c.JobDBURL == "" {
		return fmt.Errorf("JOB_DB_URL cannot be empty")
	}

	if c.RabbitMQURL == "" {
		return fmt.Errorf("RABBITMQ_URL cannot be empty")
	}

	if c.QueueName == "" {
		return fmt.Errorf("QUEUE_NAME cannot be empty")
	}

	if c.ExchangeName == "" {
		return fmt.Errorf("EXCHANGE_NAME cannot be empty")
	}

	if c.MaxConcurrency < 1 {
		return fmt.Errorf("MAX_CONCURRENCY must be at least 1")
	}

	if c.WorkerCount < 1 {
		return fmt.Errorf("WORKER_COUNT must be at least 1")
	}

	if c.MinContentLength < 0 {
		return fmt.Errorf("MIN_CONTENT_LENGTH cannot be negative")
	}

	if c.MaxContentLength < c.MinContentLength {
		return fmt.Errorf("MAX_CONTENT_LENGTH must be greater than MIN_CONTENT_LENGTH")
	}

	if c.ContentSimilarityThreshold < 0 || c.ContentSimilarityThreshold > 1 {
		return fmt.Errorf("CONTENT_SIMILARITY_THRESHOLD must be between 0 and 1")
	}

	if c.MaxNestingLevel < 1 || c.MaxNestingLevel > 10 {
		return fmt.Errorf("MAX_NESTING_LEVEL must be between 1 and 10")
	}

	if c.QueueMaxRetries < 0 {
		return fmt.Errorf("QUEUE_MAX_RETRIES cannot be negative")
	}

	if c.MaxJobRetries < 0 {
		return fmt.Errorf("MAX_JOB_RETRIES cannot be negative")
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}

// GetDSN returns the database connection string based on the database type
func (c *Config) GetDSN() string {
	switch c.JobDBType {
	case "postgres":
		return c.JobDBURL
	case "sqlite":
		return c.JobDBURL
	default:
		return c.JobDBURL
	}
}

// GetRabbitMQConfig returns RabbitMQ configuration for easier access
func (c *Config) GetRabbitMQConfig() (url, queueName, dlqName, exchangeName string, maxRetries int) {
	return c.RabbitMQURL, c.QueueName, c.DeadLetterQueue, c.ExchangeName, c.QueueMaxRetries
}

// GetWorkerConfig returns worker configuration for easier access
func (c *Config) GetWorkerConfig() (count int, timeout time.Duration, maxRetries int, shutdownTimeout time.Duration) {
	return c.WorkerCount, c.JobTimeout, c.MaxJobRetries, c.WorkerShutdownTimeout
}

// GetBrowserConfig returns browser configuration for easier access
func (c *Config) GetBrowserConfig() (headless bool, timeout time.Duration, userAgent string, width, height int) {
	return c.BrowserHeadless, c.BrowserTimeout, c.BrowserUserAgent, c.BrowserViewportWidth, c.BrowserViewportHeight
}
