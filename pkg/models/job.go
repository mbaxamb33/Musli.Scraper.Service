// pkg/models/job.go
package models

import (
	"time"
)

// ScrapingJobRequest represents a request to scrape content
type ScrapingJobRequest struct {
	URL          string                 `json:"url" binding:"required"`
	DatasourceID int32                  `json:"datasource_id,omitempty"`
	CallbackURL  string                 `json:"callback_url,omitempty"`
	Priority     int                    `json:"priority,omitempty"` // 0-9, higher = more urgent
	Options      ScrapingOptions        `json:"options,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ScrapingOptions contains scraping configuration
type ScrapingOptions struct {
	WaitForJS       bool          `json:"wait_for_js"`
	Timeout         time.Duration `json:"timeout"`
	MaxDepth        int           `json:"max_depth"`
	Depth           int           `json:"depth"` // Current crawl depth
	FollowLinks     bool          `json:"follow_links"`
	RespectRobots   bool          `json:"respect_robots"`
	UserAgent       string        `json:"user_agent,omitempty"`
	WaitForSelector string        `json:"wait_for_selector,omitempty"`
	ScrollToBottom  bool          `json:"scroll_to_bottom"`
	Screenshot      bool          `json:"screenshot"`

	// Crawling options
	MaxPages        int      `json:"max_pages,omitempty"`        // Maximum pages to crawl
	SameDomainOnly  bool     `json:"same_domain_only"`           // Only crawl within same domain
	IncludePatterns []string `json:"include_patterns,omitempty"` // URL patterns to include
	ExcludePatterns []string `json:"exclude_patterns,omitempty"` // URL patterns to exclude
}

// ScrapingJob represents a scraping job in the database
type ScrapingJob struct {
	ID           string                 `json:"id" db:"id"`
	URL          string                 `json:"url" db:"url"`
	DatasourceID *int32                 `json:"datasource_id,omitempty" db:"datasource_id"`
	CallbackURL  *string                `json:"callback_url,omitempty" db:"callback_url"`
	Options      ScrapingOptions        `json:"options" db:"options"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	Status       JobStatus              `json:"status" db:"status"`
	Progress     int                    `json:"progress" db:"progress"`
	StartedAt    time.Time              `json:"started_at" db:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	Error        *string                `json:"error,omitempty" db:"error"`
	RetryCount   int                    `json:"retry_count" db:"retry_count"`
	Results      *ScrapingResults       `json:"results,omitempty" db:"results"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

// ScrapingJobResponse represents the response for a scraping job
type ScrapingJobResponse struct {
	ID          string           `json:"id"`
	Status      JobStatus        `json:"status"`
	Progress    int              `json:"progress"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	Error       *string          `json:"error,omitempty"`
	Results     *ScrapingResults `json:"results,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// JobListResponse represents a paginated list of jobs
type JobListResponse struct {
	Jobs       []ScrapingJobResponse `json:"jobs"`
	TotalCount int                   `json:"total_count"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	HasMore    bool                  `json:"has_more"`
}

// JobStatusUpdate represents a status update for a job
type JobStatusUpdate struct {
	Status   JobStatus `json:"status"`
	Progress int       `json:"progress"`
	Error    *string   `json:"error,omitempty"`
}

// CallbackPayload represents the payload sent to callback URLs
type CallbackPayload struct {
	JobID       string           `json:"job_id"`
	Status      JobStatus        `json:"status"`
	Progress    int              `json:"progress"`
	Results     *ScrapingResults `json:"results,omitempty"`
	Error       *string          `json:"error,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
}

// JobMetrics represents metrics about job processing
type JobMetrics struct {
	TotalJobs         int64         `json:"total_jobs"`
	PendingJobs       int64         `json:"pending_jobs"`
	ProcessingJobs    int64         `json:"processing_jobs"`
	CompletedJobs     int64         `json:"completed_jobs"`
	FailedJobs        int64         `json:"failed_jobs"`
	SuccessRate       float64       `json:"success_rate"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
}

// QueueStatus represents the status of the job queue
type QueueStatus struct {
	Available    bool   `json:"available"`
	QueueSize    int    `json:"queue_size,omitempty"`
	ActiveJobs   int    `json:"active_jobs,omitempty"`
	HealthStatus string `json:"health_status,omitempty"`
	Message      string `json:"message,omitempty"`
}
