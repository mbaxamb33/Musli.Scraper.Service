// internal/handlers/health_handler.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mbaxamb3/nusli/scraper-service/internal/scraper"
	"github.com/mbaxamb3/nusli/scraper-service/internal/storage"
	"go.uber.org/zap"
)

type HealthHandler struct {
	db      *storage.Database
	scraper *scraper.Engine
	logger  *zap.Logger
}

func NewHealthHandler(db *storage.Database, scraperEngine *scraper.Engine, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		db:      db,
		scraper: scraperEngine,
		logger:  logger,
	}
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Service   string            `json:"service"`
	Version   string            `json:"version"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

// Health handles GET /health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status:    "ok",
		Service:   "musli-scraper-service",
		Version:   "1.0.0",
		Timestamp: "2025-01-01T00:00:00Z", // You can use time.Now().Format(time.RFC3339)
		Checks:    make(map[string]string),
	}

	// Check database
	if err := h.db.Health(r.Context()); err != nil {
		response.Status = "error"
		response.Checks["database"] = "failed: " + err.Error()
		h.logger.Error("Database health check failed", zap.Error(err))
	} else {
		response.Checks["database"] = "ok"
	}

	// Check scraper engine
	if err := h.scraper.Health(r.Context()); err != nil {
		response.Status = "error"
		response.Checks["scraper"] = "failed: " + err.Error()
		h.logger.Error("Scraper health check failed", zap.Error(err))
	} else {
		response.Checks["scraper"] = "ok"
	}

	// Set response status code
	statusCode := http.StatusOK
	if response.Status == "error" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// Ready handles GET /ready - Kubernetes readiness probe
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simple readiness check
	if err := h.db.Health(r.Context()); err != nil {
		http.Error(w, "Not ready", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

// Live handles GET /live - Kubernetes liveness probe
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}
