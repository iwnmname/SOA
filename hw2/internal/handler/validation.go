package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"unicode/utf8"

	"hw2/pkg/generated"
)

type Violation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

var promoCodeRegex = regexp.MustCompile(`^[A-Z0-9_]{4,20}$`)

func validateProductCreate(req *generated.ProductCreate) []Violation {
	var v []Violation

	nameLen := utf8.RuneCountInString(req.Name)
	if nameLen < 1 || nameLen > 255 {
		v = append(v, Violation{Field: "name", Message: "must have length between 1 and 255"})
	}

	if req.Description != nil {
		descLen := utf8.RuneCountInString(*req.Description)
		if descLen > 4000 {
			v = append(v, Violation{Field: "description", Message: "must have length at most 4000"})
		}
	}

	if req.Price < 0.01 {
		v = append(v, Violation{Field: "price", Message: "must be at least 0.01"})
	}

	if req.Stock < 0 {
		v = append(v, Violation{Field: "stock", Message: "must be at least 0"})
	}

	catLen := utf8.RuneCountInString(req.Category)
	if catLen < 1 || catLen > 100 {
		v = append(v, Violation{Field: "category", Message: "must have length between 1 and 100"})
	}

	return v
}

func validateProductUpdate(req *generated.ProductUpdate) []Violation {
	var v []Violation

	nameLen := utf8.RuneCountInString(req.Name)
	if nameLen < 1 || nameLen > 255 {
		v = append(v, Violation{Field: "name", Message: "must have length between 1 and 255"})
	}

	if req.Description != nil {
		descLen := utf8.RuneCountInString(*req.Description)
		if descLen > 4000 {
			v = append(v, Violation{Field: "description", Message: "must have length at most 4000"})
		}
	}

	if req.Price < 0.01 {
		v = append(v, Violation{Field: "price", Message: "must be at least 0.01"})
	}

	if req.Stock < 0 {
		v = append(v, Violation{Field: "stock", Message: "must be at least 0"})
	}

	catLen := utf8.RuneCountInString(req.Category)
	if catLen < 1 || catLen > 100 {
		v = append(v, Violation{Field: "category", Message: "must have length between 1 and 100"})
	}

	validStatuses := map[string]bool{"ACTIVE": true, "INACTIVE": true, "ARCHIVED": true}
	if !validStatuses[string(req.Status)] {
		v = append(v, Violation{Field: "status", Message: "must be one of: ACTIVE, INACTIVE, ARCHIVED"})
	}

	return v
}

func validateOrderCreate(req *generated.OrderCreate) []Violation {
	var v []Violation

	if len(req.Items) < 1 {
		v = append(v, Violation{Field: "items", Message: "must have at least 1 item"})
	}
	if len(req.Items) > 50 {
		v = append(v, Violation{Field: "items", Message: "must have at most 50 items"})
	}

	for i, item := range req.Items {
		if item.Quantity < 1 || item.Quantity > 999 {
			v = append(v, Violation{
				Field:   fmt.Sprintf("items[%d].quantity", i),
				Message: "must be between 1 and 999",
			})
		}
	}

	if req.PromoCode != nil && *req.PromoCode != "" {
		if !promoCodeRegex.MatchString(*req.PromoCode) {
			v = append(v, Violation{Field: "promo_code", Message: "must match pattern ^[A-Z0-9_]{4,20}$"})
		}
	}

	return v
}

func validateOrderUpdate(req *generated.OrderUpdate) []Violation {
	var v []Violation

	if len(req.Items) < 1 {
		v = append(v, Violation{Field: "items", Message: "must have at least 1 item"})
	}
	if len(req.Items) > 50 {
		v = append(v, Violation{Field: "items", Message: "must have at most 50 items"})
	}

	for i, item := range req.Items {
		if item.Quantity < 1 || item.Quantity > 999 {
			v = append(v, Violation{
				Field:   fmt.Sprintf("items[%d].quantity", i),
				Message: "must be between 1 and 999",
			})
		}
	}

	return v
}

func validateRegister(req *generated.RegisterRequest) []Violation {
	var v []Violation

	if string(req.Email) == "" {
		v = append(v, Violation{Field: "email", Message: "must not be empty"})
	}

	passLen := utf8.RuneCountInString(req.Password)
	if passLen < 6 || passLen > 100 {
		v = append(v, Violation{Field: "password", Message: "must have length between 6 and 100"})
	}

	validRoles := map[string]bool{"USER": true, "SELLER": true, "ADMIN": true}
	if !validRoles[string(req.Role)] {
		v = append(v, Violation{Field: "role", Message: "must be one of: USER, SELLER, ADMIN"})
	}

	return v
}

func validatePromoCodeCreate(req *generated.PromoCodeCreate) []Violation {
	var v []Violation

	if !promoCodeRegex.MatchString(req.Code) {
		v = append(v, Violation{Field: "code", Message: "must match pattern ^[A-Z0-9_]{4,20}$"})
	}

	validTypes := map[string]bool{"PERCENTAGE": true, "FIXED_AMOUNT": true}
	if !validTypes[string(req.DiscountType)] {
		v = append(v, Violation{Field: "discount_type", Message: "must be PERCENTAGE or FIXED_AMOUNT"})
	}

	if req.DiscountValue < 0.01 {
		v = append(v, Violation{Field: "discount_value", Message: "must be at least 0.01"})
	}

	if req.MinOrderAmount < 0 {
		v = append(v, Violation{Field: "min_order_amount", Message: "must be at least 0"})
	}

	if req.MaxUses < 1 {
		v = append(v, Violation{Field: "max_uses", Message: "must be at least 1"})
	}

	return v
}

func writeValidationError(w http.ResponseWriter, violations []Violation) {
	writeErrorWithDetails(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", map[string]interface{}{
		"violations": violations,
	})
}
