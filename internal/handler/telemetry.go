package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"simtelemetry-hub/internal/repository"
	"simtelemetry-hub/internal/service"
)

type TelemetryHandler struct {
	svc  service.TelemetryService
	repo repository.TelemetryRepository
}

func NewTelemetryHandler(svc service.TelemetryService, repo repository.TelemetryRepository) *TelemetryHandler {
	return &TelemetryHandler{
		svc:  svc,
		repo: repo,
	}
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// IngestTelemetry handles POST /api/v1/telemetry
// Validates payload, submits job to Worker Pool, and returns 202 Accepted instantly
func (h *TelemetryHandler) IngestTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload repository.TelemetryPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}

	if err := h.svc.ProcessTelemetry(payload); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Status:  "processing",
		Message: "telemetry payload accepted for asynchronous processing",
	})
}

// GetLeaderboard handles GET /api/v1/leaderboard?track={track_name}&limit=10
func (h *TelemetryHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	trackName := r.URL.Query().Get("track")
	if trackName == "" {
		h.writeError(w, http.StatusBadRequest, "query parameter 'track' is required")
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	leaderboard, err := h.svc.GetLeaderboard(r.Context(), trackName, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to retrieve leaderboard: "+err.Error())
		return
	}

	if leaderboard == nil {
		leaderboard = []repository.LeaderboardEntry{}
	}

	_ = json.NewEncoder(w).Encode(APIResponse{
		Status: "success",
		Data:   leaderboard,
	})
}

// HealthCheck handles GET /api/v1/health
func (h *TelemetryHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(APIResponse{
			Status: "error",
			Error:  "database connection failed: " + err.Error(),
		})
		return
	}

	pendingJobs, workerCount := h.svc.GetQueueStatus()

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Status:  "healthy",
		Message: "SimTelemetry Hub service is fully operational",
		Data: map[string]interface{}{
			"database":     "connected",
			"worker_pool":  workerCount,
			"pending_jobs": pendingJobs,
		},
	})
}

func (h *TelemetryHandler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Status: "error",
		Error:  message,
	})
}
