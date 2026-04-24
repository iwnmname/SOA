package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/online-cinema/aggregator/internal/repository"
	"github.com/online-cinema/aggregator/internal/service"
)

type Handler struct {
	svc        *service.Service
	clickhouse *repository.ClickHouseRepo
	postgres   *repository.PostgresRepo
	logger     *zap.Logger
}

func NewHandler(
	svc *service.Service,
	clickhouse *repository.ClickHouseRepo,
	postgres *repository.PostgresRepo,
	logger *zap.Logger,
) *Handler {
	return &Handler{svc: svc, clickhouse: clickhouse, postgres: postgres, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/aggregate/run", h.run)
	mux.HandleFunc("/export/run", h.export)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.clickhouse.Ping(ctx); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": "clickhouse: " + err.Error()})
		return
	}
	if err := h.postgres.Ping(ctx); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": "postgres: " + err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	metricDate := time.Now().UTC()
	if rawDate := r.URL.Query().Get("date"); rawDate != "" {
		parsed, err := time.Parse("2006-01-02", rawDate)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date format, expected YYYY-MM-DD"})
			return
		}
		metricDate = parsed.UTC()
	}

	result, err := h.svc.RunCycleForDate(r.Context(), metricDate)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, service.ErrAlreadyRunning) {
			code = http.StatusConflict
		}
		h.logger.Warn("manual aggregation failed", zap.Error(err))
		respondJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	metricDate := time.Now().UTC().AddDate(0, 0, -1)
	if rawDate := r.URL.Query().Get("date"); rawDate != "" {
		parsed, err := time.Parse("2006-01-02", rawDate)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date format, expected YYYY-MM-DD"})
			return
		}
		metricDate = parsed.UTC()
	}

	result, err := h.svc.RunExportForDate(r.Context(), metricDate)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, service.ErrExportAlreadyRunning) {
			code = http.StatusConflict
		}
		h.logger.Warn("manual export failed", zap.Error(err))
		respondJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
