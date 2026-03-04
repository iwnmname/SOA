package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"hw2/internal/domain"
	"hw2/internal/repository"
)

var (
	ErrOrderNotFound          = errors.New("order not found")
	ErrOrderLimitExceeded     = errors.New("order limit exceeded")
	ErrOrderHasActive         = errors.New("user has active order")
	ErrProductNotFound        = errors.New("product not found")
	ErrProductInactive        = errors.New("product inactive")
	ErrPromoCodeInvalid       = errors.New("promo code invalid")
	ErrPromoCodeMinAmount     = errors.New("promo code min amount")
	ErrOwnershipViolation     = errors.New("order ownership violation")
	ErrInvalidOrderTransition = errors.New("invalid order state transition")
)

type StockDetail struct {
	ProductID uuid.UUID `json:"product_id"`
	Requested int       `json:"requested"`
	Available int       `json:"available"`
}

type InsufficientStockError struct {
	Items []StockDetail
}

func (e *InsufficientStockError) Error() string { return "insufficient stock" }

type OrderService struct {
	db          *pgxpool.Pool
	orderRepo   *repository.OrderRepo
	productRepo *repository.ProductRepo
	promoRepo   *repository.PromoRepo
	userOpRepo  *repository.UserOpRepo
	rateLimit   time.Duration
}

func NewOrderService(
	db *pgxpool.Pool,
	orderRepo *repository.OrderRepo,
	productRepo *repository.ProductRepo,
	promoRepo *repository.PromoRepo,
	userOpRepo *repository.UserOpRepo,
	rateLimitMinutes int,
) *OrderService {
	return &OrderService{
		db:          db,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		promoRepo:   promoRepo,
		userOpRepo:  userOpRepo,
		rateLimit:   time.Duration(rateLimitMinutes) * time.Minute,
	}
}

func (s *OrderService) Create(ctx context.Context, userID uuid.UUID, items []domain.OrderItem, promoCode *string) (*domain.Order, error) {
	if err := s.checkRateLimit(ctx, userID, "CREATE_ORDER"); err != nil {
		return nil, err
	}

	hasActive, err := s.orderRepo.HasActiveOrder(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	if hasActive {
		return nil, ErrOrderHasActive
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	products := make(map[uuid.UUID]*domain.Product, len(items))
	for _, item := range items {
		product, err := s.productRepo.GetByIDForUpdate(ctx, tx, item.ProductID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, fmt.Errorf("%w: %s", ErrProductNotFound, item.ProductID)
			}
			return nil, err
		}
		if product.Status != "ACTIVE" {
			return nil, fmt.Errorf("%w: %s", ErrProductInactive, item.ProductID)
		}
		products[item.ProductID] = product
	}

	var insufficientItems []StockDetail
	for _, item := range items {
		product := products[item.ProductID]
		if product.Stock < item.Quantity {
			insufficientItems = append(insufficientItems, StockDetail{
				ProductID: item.ProductID,
				Requested: item.Quantity,
				Available: product.Stock,
			})
		}
	}
	if len(insufficientItems) > 0 {
		return nil, &InsufficientStockError{Items: insufficientItems}
	}

	for _, item := range items {
		if err := s.productRepo.UpdateStock(ctx, tx, item.ProductID, -item.Quantity); err != nil {
			return nil, err
		}
	}

	for i, item := range items {
		items[i].PriceAtOrder = products[item.ProductID].Price
	}

	var totalAmount float64
	for _, item := range items {
		totalAmount += item.PriceAtOrder * float64(item.Quantity)
	}

	var discountAmount float64
	var promoCodeID *uuid.UUID

	if promoCode != nil && *promoCode != "" {
		promo, err := s.promoRepo.GetByCode(ctx, tx, *promoCode)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrPromoCodeInvalid
			}
			return nil, err
		}
		if err := validatePromo(promo); err != nil {
			return nil, err
		}
		if totalAmount < promo.MinOrderAmount {
			return nil, ErrPromoCodeMinAmount
		}
		discountAmount = calculateDiscount(promo, totalAmount)
		promoCodeID = &promo.ID
		if err := s.promoRepo.IncrementUses(ctx, tx, promo.ID); err != nil {
			return nil, err
		}
	}

	order := &domain.Order{
		UserID:         userID,
		Status:         "CREATED",
		PromoCodeID:    promoCodeID,
		TotalAmount:    totalAmount - discountAmount,
		DiscountAmount: discountAmount,
	}

	if err := s.orderRepo.Create(ctx, tx, order); err != nil {
		return nil, err
	}

	for i := range items {
		items[i].OrderID = order.ID
		if err := s.orderRepo.CreateItem(ctx, tx, &items[i]); err != nil {
			return nil, err
		}
	}
	order.Items = items

	if err := s.userOpRepo.Create(ctx, tx, userID, "CREATE_ORDER"); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	o, err := s.orderRepo.GetByID(ctx, s.db, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrOrderNotFound
	}
	return o, err
}

func (s *OrderService) Update(ctx context.Context, userID uuid.UUID, orderID uuid.UUID, newItems []domain.OrderItem, bypassOwnership bool) (*domain.Order, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order, err := s.orderRepo.GetByID(ctx, tx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if !bypassOwnership && order.UserID != userID {
		return nil, ErrOwnershipViolation
	}
	if order.Status != "CREATED" {
		return nil, ErrInvalidOrderTransition
	}

	if err := s.checkRateLimit(ctx, userID, "UPDATE_ORDER"); err != nil {
		return nil, err
	}

	for _, item := range order.Items {
		if err := s.productRepo.UpdateStock(ctx, tx, item.ProductID, item.Quantity); err != nil {
			return nil, err
		}
	}

	products := make(map[uuid.UUID]*domain.Product, len(newItems))
	for _, item := range newItems {
		product, err := s.productRepo.GetByIDForUpdate(ctx, tx, item.ProductID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, fmt.Errorf("%w: %s", ErrProductNotFound, item.ProductID)
			}
			return nil, err
		}
		if product.Status != "ACTIVE" {
			return nil, fmt.Errorf("%w: %s", ErrProductInactive, item.ProductID)
		}
		products[item.ProductID] = product
	}

	var insufficientItems []StockDetail
	for _, item := range newItems {
		product := products[item.ProductID]
		if product.Stock < item.Quantity {
			insufficientItems = append(insufficientItems, StockDetail{
				ProductID: item.ProductID,
				Requested: item.Quantity,
				Available: product.Stock,
			})
		}
	}
	if len(insufficientItems) > 0 {
		return nil, &InsufficientStockError{Items: insufficientItems}
	}

	for _, item := range newItems {
		if err := s.productRepo.UpdateStock(ctx, tx, item.ProductID, -item.Quantity); err != nil {
			return nil, err
		}
	}

	if err := s.orderRepo.DeleteItems(ctx, tx, orderID); err != nil {
		return nil, err
	}

	for i, item := range newItems {
		newItems[i].PriceAtOrder = products[item.ProductID].Price
	}

	var totalAmount float64
	for _, item := range newItems {
		totalAmount += item.PriceAtOrder * float64(item.Quantity)
	}

	var discountAmount float64
	var promoCodeID *uuid.UUID

	if order.PromoCodeID != nil {
		promo, err := s.promoRepo.GetByID(ctx, tx, *order.PromoCodeID)
		if err == nil {
			if validatePromo(promo) == nil && totalAmount >= promo.MinOrderAmount {
				discountAmount = calculateDiscount(promo, totalAmount)
				promoCodeID = &promo.ID
			} else {
				if err := s.promoRepo.DecrementUses(ctx, tx, promo.ID); err != nil {
					return nil, err
				}
			}
		}
	}

	order.TotalAmount = totalAmount - discountAmount
	order.DiscountAmount = discountAmount
	order.PromoCodeID = promoCodeID

	if err := s.orderRepo.Update(ctx, tx, order); err != nil {
		return nil, err
	}

	for i := range newItems {
		newItems[i].OrderID = orderID
		if err := s.orderRepo.CreateItem(ctx, tx, &newItems[i]); err != nil {
			return nil, err
		}
	}
	order.Items = newItems

	if err := s.userOpRepo.Create(ctx, tx, userID, "UPDATE_ORDER"); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) Cancel(ctx context.Context, userID uuid.UUID, orderID uuid.UUID, bypassOwnership bool) (*domain.Order, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order, err := s.orderRepo.GetByID(ctx, tx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if !bypassOwnership && order.UserID != userID {
		return nil, ErrOwnershipViolation
	}
	if order.Status != "CREATED" && order.Status != "PAYMENT_PENDING" {
		return nil, ErrInvalidOrderTransition
	}

	for _, item := range order.Items {
		if err := s.productRepo.UpdateStock(ctx, tx, item.ProductID, item.Quantity); err != nil {
			return nil, err
		}
	}

	if order.PromoCodeID != nil {
		if err := s.promoRepo.DecrementUses(ctx, tx, *order.PromoCodeID); err != nil {
			return nil, err
		}
	}

	order.Status = "CANCELED"
	if err := s.orderRepo.Update(ctx, tx, order); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) checkRateLimit(ctx context.Context, userID uuid.UUID, opType string) error {
	lastOp, err := s.userOpRepo.GetLastOperation(ctx, s.db, userID, opType)
	if err != nil {
		return err
	}
	if lastOp != nil && time.Since(*lastOp) < s.rateLimit {
		return ErrOrderLimitExceeded
	}
	return nil
}

func validatePromo(p *domain.PromoCode) error {
	now := time.Now()
	if !p.Active || p.CurrentUses >= p.MaxUses || now.Before(p.ValidFrom) || now.After(p.ValidUntil) {
		return ErrPromoCodeInvalid
	}
	return nil
}

func calculateDiscount(p *domain.PromoCode, totalAmount float64) float64 {
	switch p.DiscountType {
	case "PERCENTAGE":
		discount := totalAmount * p.DiscountValue / 100
		maxDiscount := totalAmount * 0.7
		if discount > maxDiscount {
			discount = maxDiscount
		}
		return discount
	case "FIXED_AMOUNT":
		if p.DiscountValue > totalAmount {
			return totalAmount
		}
		return p.DiscountValue
	}
	return 0
}
