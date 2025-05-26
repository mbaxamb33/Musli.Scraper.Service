// internal/handlers/job_handler.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/mbaxamb3/nusli/scraper-service/internal/services"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

type JobHandler struct {
	jobService *services.JobService
	logger     *zap.Logger
}

func NewJobHandler(jobService *services.JobService, logger *zap.Logger) *JobHandler {
	return &JobHandler{
		jobService: jobService,
		logger:     logger,
	}
}

// CreateJob handles POST /api/jobs
func (h *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.ScrapingJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid request body", zap.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Basic validation
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.CreateScrapingJob(r.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create job", zap.Error(err))
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// GetJob handles GET /api/jobs/{id}
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractJobID(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.GetScrapingJob(r.Context(), jobID)
	if err != nil {
		if err.Error() == "job not found: "+jobID {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get job", zap.Error(err))
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// ListJobs handles GET /api/jobs
func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse pagination parameters
	limit, offset := h.parsePaginationParams(r)

	// Check for status filter
	statusParam := r.URL.Query().Get("status")
	if statusParam != "" {
		status := models.JobStatus(statusParam)
		jobs, err := h.jobService.GetJobsByStatus(r.Context(), status, limit, offset)
		if err != nil {
			h.logger.Error("Failed to get jobs by status", zap.Error(err))
			http.Error(w, "Failed to get jobs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jobs":   jobs,
			"status": statusParam,
		})
		return
	}

	// Get all jobs with pagination
	response, err := h.jobService.ListScrapingJobs(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list jobs", zap.Error(err))
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CancelJob handles DELETE /api/jobs/{id}
func (h *JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractJobID(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	err := h.jobService.CancelJob(r.Context(), jobID)
	if err != nil {
		h.logger.Error("Failed to cancel job", zap.String("job_id", jobID), zap.Error(err))
		http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Job canceled successfully",
		"job_id":  jobID,
	})
}

// GetMetrics handles GET /api/metrics
func (h *JobHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics, err := h.jobService.GetJobMetrics(r.Context())
	if err != nil {
		h.logger.Error("Failed to get metrics", zap.Error(err))
		http.Error(w, "Failed to get metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// ProcessJob handles POST /api/jobs/{id}/process
func (h *JobHandler) ProcessJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractJobID(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	err := h.jobService.ProcessJob(r.Context(), jobID)
	if err != nil {
		h.logger.Error("Failed to process job", zap.String("job_id", jobID), zap.Error(err))
		http.Error(w, "Failed to process job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Job processing started",
		"job_id":  jobID,
	})
}

// Helper functions

func (h *JobHandler) parsePaginationParams(r *http.Request) (limit, offset int32) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit = 20 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	offset = 0 // default
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	return limit, offset
}

func extractJobID(path string) string {
	// Handle paths like /api/jobs/{id} or /api/jobs/{id}/process
	// Remove leading slash and split by slash
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	// Expected format: ["api", "jobs", "{id}"] or ["api", "jobs", "{id}", "process"]
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "jobs" {
		jobID := parts[2]
		// Make sure we have a valid job ID (not empty)
		if jobID != "" {
			return jobID
		}
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
