package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"hw2/internal/middleware"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeErrorWithDetails(w, status, code, message, nil)
}

func writeErrorWithDetails(w http.ResponseWriter, status int, code string, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error_code": code,
		"message":    message,
		"details":    details,
	})
}

func MustGetAuth(ctx context.Context) (uuid.UUID, string) {
	userID, _ := middleware.GetUserID(ctx)
	role, _ := middleware.GetUserRole(ctx)
	return userID, role
}
