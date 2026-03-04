package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"hw2/internal/service"
)

type contextKey string

const (
	userIDKey   contextKey = "user_id"
	userRoleKey contextKey = "user_role"
)

func AuthMiddleware(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/auth/") {
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				writeAuthError(w, http.StatusUnauthorized, "TOKEN_INVALID", "Missing or invalid Authorization header")
				return
			}

			tokenString := strings.TrimPrefix(header, "Bearer ")
			userID, role, err := authSvc.ValidateAccessToken(tokenString)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					writeAuthError(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Access token has expired")
					return
				}
				writeAuthError(w, http.StatusUnauthorized, "TOKEN_INVALID", "Invalid access token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, userRoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func GetUserRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(userRoleKey).(string)
	return role, ok
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error_code": code,
		"message":    message,
		"details":    nil,
	})
}
