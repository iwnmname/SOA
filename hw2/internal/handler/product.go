package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"hw2/internal/domain"
	"hw2/internal/repository"
	"hw2/internal/service"
	"hw2/pkg/generated"
)

type ProductHandler struct {
	svc *service.ProductService
}

func NewProductHandler(svc *service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	userID, role := MustGetAuth(r.Context())

	if role == "USER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Users cannot create products")
		return
	}

	var req generated.ProductCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if violations := validateProductCreate(&req); len(violations) > 0 {
		writeValidationError(w, violations)
		return
	}

	p := &domain.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    req.Category,
	}

	if role == "SELLER" {
		p.SellerID = &userID
	}

	if err := h.svc.Create(r.Context(), p); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toProductResponse(p))
}

func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	p, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProductResponse(p))
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	userID, role := MustGetAuth(r.Context())

	if role == "USER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Users cannot update products")
		return
	}

	if role == "SELLER" {
		if denied := h.checkOwnership(w, r.Context(), id, userID); denied {
			return
		}
	}

	var req generated.ProductUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if violations := validateProductUpdate(&req); len(violations) > 0 {
		writeValidationError(w, violations)
		return
	}

	p := &domain.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    req.Category,
		Status:      string(req.Status),
	}

	updated, err := h.svc.Update(r.Context(), id, p)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}
		if errors.Is(err, service.ErrInvalidStateTransition) {
			writeError(w, http.StatusConflict, "INVALID_STATE_TRANSITION", "Cannot update archived product")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProductResponse(updated))
}

func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	userID, role := MustGetAuth(r.Context())

	if role == "USER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Users cannot delete products")
		return
	}

	if role == "SELLER" {
		if denied := h.checkOwnership(w, r.Context(), id, userID); denied {
			return
		}
	}

	p, err := h.svc.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}
		if errors.Is(err, service.ErrInvalidStateTransition) {
			writeError(w, http.StatusConflict, "INVALID_STATE_TRANSITION", "Product is already archived")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProductResponse(p))
}

func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request, params generated.ListProductsParams) {
	page := 0
	size := 20
	if params.Page != nil {
		page = *params.Page
	}
	if params.Size != nil {
		size = *params.Size
	}
	filter := repository.ProductFilter{Page: page, Size: size}
	if params.Status != nil {
		s := string(*params.Status)
		filter.Status = &s
	}
	if params.Category != nil {
		filter.Category = params.Category
	}
	products, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	items := make([]generated.ProductResponse, 0, len(products))
	for _, p := range products {
		items = append(items, toProductResponse(p))
	}
	writeJSON(w, http.StatusOK, generated.ProductListResponse{
		Items:         items,
		TotalElements: total,
		Page:          page,
		Size:          size,
	})
}

func (h *ProductHandler) checkOwnership(w http.ResponseWriter, ctx context.Context, productID, userID uuid.UUID) bool {
	product, err := h.svc.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return true
	}
	if product.SellerID == nil || *product.SellerID != userID {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "You can only manage your own products")
		return true
	}
	return false
}

func toProductResponse(p *domain.Product) generated.ProductResponse {
	resp := generated.ProductResponse{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		Category:    p.Category,
		Status:      generated.ProductStatus(p.Status),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
	if p.SellerID != nil {
		resp.SellerId = p.SellerID
	}
	return resp
}
