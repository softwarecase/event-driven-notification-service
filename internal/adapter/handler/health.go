package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result := map[string]string{
		"status":   "healthy",
		"postgres": "up",
		"redis":    "up",
	}
	statusCode := http.StatusOK

	if err := h.db.Ping(ctx); err != nil {
		result["status"] = "unhealthy"
		result["postgres"] = "down"
		statusCode = http.StatusServiceUnavailable
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		result["status"] = "unhealthy"
		result["redis"] = "down"
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, result)
}
