package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"hw2/internal/domain"
	"hw2/internal/service"
	"hw2/pkg/generated"
)

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, role := MustGetAuth(r.Context())

	if role == "SELLER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Sellers cannot create orders")
		return
	}

	var req generated.OrderCreate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if violations := validateOrderCreate(&req); len(violations) > 0 {
		writeValidationError(w, violations)
		return
	}

	items := make([]domain.OrderItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = domain.OrderItem{
			ProductID: it.ProductId,
			Quantity:  it.Quantity,
		}
	}

	order, err := h.svc.Create(r.Context(), userID, items, req.PromoCode)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toOrderResponse(order))
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	userID, role := MustGetAuth(r.Context())

	if role == "SELLER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Sellers cannot access orders")
		return
	}

	order, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			writeError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "Order not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if role == "USER" && order.UserID != userID {
		writeError(w, http.StatusForbidden, "ORDER_OWNERSHIP_VIOLATION", "Order belongs to another user")
		return
	}

	writeJSON(w, http.StatusOK, toOrderResponse(order))
}

func (h *OrderHandler) UpdateOrder(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	userID, role := MustGetAuth(r.Context())

	if role == "SELLER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Sellers cannot update orders")
		return
	}

	var req generated.OrderUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if violations := validateOrderUpdate(&req); len(violations) > 0 {
		writeValidationError(w, violations)
		return
	}

	items := make([]domain.OrderItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = domain.OrderItem{
			ProductID: it.ProductId,
			Quantity:  it.Quantity,
		}
	}

	bypassOwnership := role == "ADMIN"
	order, err := h.svc.Update(r.Context(), userID, id, items, bypassOwnership)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrderResponse(order))
}

func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	userID, role := MustGetAuth(r.Context())

	if role == "SELLER" {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "Sellers cannot cancel orders")
		return
	}

	bypassOwnership := role == "ADMIN"
	order, err := h.svc.Cancel(r.Context(), userID, id, bypassOwnership)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrderResponse(order))
}

func (h *OrderHandler) handleOrderError(w http.ResponseWriter, err error) {
	var stockErr *service.InsufficientStockError
	if errors.As(err, &stockErr) {
		writeErrorWithDetails(w, http.StatusConflict, "INSUFFICIENT_STOCK", "Insufficient stock", map[string]interface{}{
			"items": stockErr.Items,
		})
		return
	}

	switch {
	case errors.Is(err, service.ErrOrderNotFound):
		writeError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "Order not found")
	case errors.Is(err, service.ErrOrderLimitExceeded):
		writeError(w, http.StatusTooManyRequests, "ORDER_LIMIT_EXCEEDED", "Rate limit exceeded")
	case errors.Is(err, service.ErrOrderHasActive):
		writeError(w, http.StatusConflict, "ORDER_HAS_ACTIVE", "User already has an active order")
	case errors.Is(err, service.ErrProductNotFound):
		writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", err.Error())
	case errors.Is(err, service.ErrProductInactive):
		writeError(w, http.StatusConflict, "PRODUCT_INACTIVE", err.Error())
	case errors.Is(err, service.ErrPromoCodeInvalid):
		writeError(w, http.StatusUnprocessableEntity, "PROMO_CODE_INVALID", "Promo code is invalid")
	case errors.Is(err, service.ErrPromoCodeMinAmount):
		writeError(w, http.StatusUnprocessableEntity, "PROMO_CODE_MIN_AMOUNT", "Order amount below minimum")
	case errors.Is(err, service.ErrOwnershipViolation):
		writeError(w, http.StatusForbidden, "ORDER_OWNERSHIP_VIOLATION", "Order belongs to another user")
	case errors.Is(err, service.ErrInvalidOrderTransition):
		writeError(w, http.StatusConflict, "INVALID_STATE_TRANSITION", "Invalid order state transition")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func toOrderResponse(o *domain.Order) generated.OrderResponse {
	items := make([]generated.OrderItemResponse, len(o.Items))
	for i, it := range o.Items {
		items[i] = generated.OrderItemResponse{
			Id:           it.ID,
			ProductId:    it.ProductID,
			Quantity:     it.Quantity,
			PriceAtOrder: it.PriceAtOrder,
		}
	}
	resp := generated.OrderResponse{
		Id:             o.ID,
		UserId:         o.UserID,
		Status:         generated.OrderStatus(o.Status),
		Items:          items,
		TotalAmount:    o.TotalAmount,
		DiscountAmount: o.DiscountAmount,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}
	if o.PromoCodeID != nil {
		resp.PromoCodeId = o.PromoCodeID
	}
	return resp
}
