package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/bootstrap"
)

type cronWorker interface {
	RunOnce(context.Context) (bootstrap.WorkerCounts, error)
}

type CronDrainHandler struct {
	secret     string
	runner     cronWorker
	budget     time.Duration
	maxBatches int
}

func NewCronDrainHandler(secret string, runner cronWorker, budget time.Duration) *CronDrainHandler {
	if budget <= 0 {
		budget = 50 * time.Second
	}
	return &CronDrainHandler{secret: strings.TrimSpace(secret), runner: runner, budget: budget, maxBatches: 100}
}

func (h *CronDrainHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeCronResponse(writer, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method_not_allowed"})
		return
	}
	if h.secret == "" || h.runner == nil {
		writeCronResponse(writer, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "cron_not_configured"})
		return
	}
	expected := "Bearer " + h.secret
	provided := request.Header.Get("Authorization")
	if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeCronResponse(writer, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), h.budget)
	defer cancel()
	total := bootstrap.WorkerCounts{}
	batches := 0
	for batches < h.maxBatches && ctx.Err() == nil {
		counts, err := h.runner.RunOnce(ctx)
		batches++
		total.Add(counts)
		if err != nil {
			writeCronResponse(writer, http.StatusInternalServerError, map[string]any{"ok": false, "error": "worker_failed", "processed": total, "batches": batches})
			return
		}
		if counts.Total() == 0 {
			break
		}
	}
	writeCronResponse(writer, http.StatusOK, map[string]any{"ok": true, "processed": total, "batches": batches})
}

func writeCronResponse(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
