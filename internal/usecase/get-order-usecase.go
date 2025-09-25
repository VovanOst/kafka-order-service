package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"kafka-order-service/internal/domain/entities"
	"kafka-order-service/internal/domain/repositories"
)

// GetOrderRequest представляет запрос на получение заказа
type GetOrderRequest struct {
	OrderID uuid.UUID `json:"order_id" validate:"required"`
}

// GetOrderResponse представляет ответ получения заказа
type GetOrderResponse struct {
	Order *entities.Order `json:"order"`
}

// GetOrderUseCase представляет use case получения заказа
type GetOrderUseCase struct {
	orderRepo repositories.OrderRepository
	logger    Logger
}

// NewGetOrderUseCase создает новый use case для получения заказа
func NewGetOrderUseCase(
	orderRepo repositories.OrderRepository,
	logger Logger,
) *GetOrderUseCase {
	return &GetOrderUseCase{
		orderRepo: orderRepo,
		logger:    logger,
	}
}

// Execute выполняет получение заказа
func (uc *GetOrderUseCase) Execute(ctx context.Context, req *GetOrderRequest) (*GetOrderResponse, error) {
	// Валидация запроса
	if err := uc.validateRequest(req); err != nil {
		uc.logger.Error("Invalid get order request", "error", err, "order_id", req.OrderID)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Получение заказа
	order, err := uc.orderRepo.GetByID(ctx, req.OrderID)
	if err != nil {
		uc.logger.Error("Failed to get order", "error", err, "order_id", req.OrderID)
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	uc.logger.Info("Order retrieved successfully", 
		"order_id", order.ID,
		"customer_id", order.CustomerID,
		"status", order.Status)

	return &GetOrderResponse{
		Order: order,
	}, nil
}

// validateRequest валидирует входящий запрос
func (uc *GetOrderUseCase) validateRequest(req *GetOrderRequest) error {
	if req == nil {
		return entities.NewValidationError("request cannot be nil")
	}

	if req.OrderID == uuid.Nil {
		return entities.NewValidationError("order_id is required")
	}

	return nil
}

// ListOrdersRequest представляет запрос на список заказов
type ListOrdersRequest struct {
	CustomerID *uuid.UUID                `json:"customer_id,omitempty"`
	Status     *entities.OrderStatus     `json:"status,omitempty"`
	Email      *string                   `json:"email,omitempty"`
	MinAmount  *float64                  `json:"min_amount,omitempty"`
	MaxAmount  *float64                  `json:"max_amount,omitempty"`
	DateFrom   *string                   `json:"date_from,omitempty"`
	DateTo     *string                   `json:"date_to,omitempty"`
	Currency   *string                   `json:"currency,omitempty"`
	Limit      int                       `json:"limit"`
	Offset     int                       `json:"offset"`
	SortBy     string                    `json:"sort_by"`
	SortOrder  string                    `json:"sort_order"`
}

// ListOrdersResponse представляет ответ списка заказов
type ListOrdersResponse struct {
	Orders     []*entities.Order `json:"orders"`
	TotalCount int64            `json:"total_count"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
}

// ListOrdersUseCase представляет use case получения списка заказов
type ListOrdersUseCase struct {
	orderRepo repositories.OrderRepository
	logger    Logger
}

// NewListOrdersUseCase создает новый use case для получения списка заказов
func NewListOrdersUseCase(
	orderRepo repositories.OrderRepository,
	logger Logger,
) *ListOrdersUseCase {
	return &ListOrdersUseCase{
		orderRepo: orderRepo,
		logger:    logger,
	}
}

// Execute выполняет получение списка заказов
func (uc *ListOrdersUseCase) Execute(ctx context.Context, req *ListOrdersRequest) (*ListOrdersResponse, error) {
	// Валидация и установка значений по умолчанию
	if err := uc.validateAndSetDefaults(req); err != nil {
		uc.logger.Error("Invalid list orders request", "error", err)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Преобразование в фильтры репозитория
	filters := repositories.OrderFilters{
		CustomerID: req.CustomerID,
		Status:     req.Status,
		Email:      req.Email,
		MinAmount:  req.MinAmount,
		MaxAmount:  req.MaxAmount,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		Currency:   req.Currency,
		Limit:      req.Limit,
		Offset:     req.Offset,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
	}

	// Получение заказов
	orders, err := uc.orderRepo.List(ctx, filters)
	if err != nil {
		uc.logger.Error("Failed to list orders", "error", err, "filters", filters)
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}

	// Получение общего количества
	totalCount, err := uc.orderRepo.Count(ctx, filters)
	if err != nil {
		uc.logger.Error("Failed to count orders", "error", err, "filters", filters)
		return nil, fmt.Errorf("failed to count orders: %w", err)
	}

	uc.logger.Info("Orders listed successfully",
		"count", len(orders),
		"total_count", totalCount,
		"limit", req.Limit,
		"offset", req.Offset)

	return &ListOrdersResponse{
		Orders:     orders,
		TotalCount: totalCount,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}, nil
}

// validateAndSetDefaults валидирует запрос и устанавливает значения по умолчанию
func (uc *ListOrdersUseCase) validateAndSetDefaults(req *ListOrdersRequest) error {
	if req == nil {
		return entities.NewValidationError("request cannot be nil")
	}

	// Установка значений по умолчанию
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100 // Максимальный лимит
	}

	if req.Offset < 0 {
		req.Offset = 0
	}

	if req.SortBy == "" {
		req.SortBy = "created_at"
	}

	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	// Валидация сортировки
	validSortFields := []string{"created_at", "updated_at", "total_amount", "status"}
	isValidSortBy := false
	for _, field := range validSortFields {
		if req.SortBy == field {
			isValidSortBy = true
			break
		}
	}
	if !isValidSortBy {
		return entities.NewValidationError("invalid sort_by field: %s", req.SortBy)
	}

	if req.SortOrder != "asc" && req.SortOrder != "desc" {
		return entities.NewValidationError("sort_order must be 'asc' or 'desc'")
	}

	// Валидация сумм
	if req.MinAmount != nil && *req.MinAmount < 0 {
		return entities.NewValidationError("min_amount cannot be negative")
	}

	if req.MaxAmount != nil && *req.MaxAmount < 0 {
		return entities.NewValidationError("max_amount cannot be negative")
	}

	if req.MinAmount != nil && req.MaxAmount != nil && *req.MinAmount > *req.MaxAmount {
		return entities.NewValidationError("min_amount cannot be greater than max_amount")
	}

	return nil
}