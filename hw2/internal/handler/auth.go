package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"hw2/internal/service"
	"hw2/pkg/generated"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var req generated.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if violations := validateRegister(&req); len(violations) > 0 {
		writeValidationError(w, violations)
		return
	}

	user, err := h.svc.Register(r.Context(), string(req.Email), req.Password, string(req.Role))
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "EMAIL_TAKEN", "Email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, generated.UserResponse{
		Id:        user.ID,
		Email:     user.Email,
		Role:      generated.Role(user.Role),
		CreatedAt: user.CreatedAt,
	})
}

func (h *AuthHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req generated.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	access, refresh, err := h.svc.Login(r.Context(), string(req.Email), req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Wrong email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, generated.TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	})
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req generated.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	access, refresh, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrRefreshTokenInvalid) {
			writeError(w, http.StatusUnauthorized, "REFRESH_TOKEN_INVALID", "Refresh token is invalid or expired")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, generated.TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	})
}
