package handler

import (
	"encoding/json"
	"net/http"

	"hw2/internal/domain"
	"hw2/internal/repository"
	"hw2/pkg/generated"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PromoHandler struct {
	db   *pgxpool.Pool
	repo *repository.PromoRepo
}

func NewPromoHandler(db *pgxpool.Pool, repo *repository.PromoRepo) *PromoHandler {
	return &PromoHandler{db: db, repo: repo}
}

func (h *PromoHandler) CreatePromoCode(w http.ResponseWriter, r *http.Request) {
	_, role := MustGetAuth(r.Context())

	if role == "USER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Users cannot create promo codes")
		return
	}

	var req generated.PromoCodeCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if violations := validatePromoCodeCreate(&req); len(violations) > 0 {
		writeValidationError(w, violations)
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	p := &domain.PromoCode{
		Code:           req.Code,
		DiscountType:   string(req.DiscountType),
		DiscountValue:  req.DiscountValue,
		MinOrderAmount: req.MinOrderAmount,
		MaxUses:        req.MaxUses,
		ValidFrom:      req.ValidFrom,
		ValidUntil:     req.ValidUntil,
		Active:         active,
	}

	if err := h.repo.Create(r.Context(), h.db, p); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, generated.PromoCodeResponse{
		Id:             p.ID,
		Code:           p.Code,
		DiscountType:   generated.DiscountType(p.DiscountType),
		DiscountValue:  p.DiscountValue,
		MinOrderAmount: p.MinOrderAmount,
		MaxUses:        p.MaxUses,
		CurrentUses:    0,
		ValidFrom:      p.ValidFrom,
		ValidUntil:     p.ValidUntil,
		Active:         p.Active,
	})
}
