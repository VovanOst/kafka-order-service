package usecase

import (
	"context"
	"fmt"
	"strings"
	_ "time"

	"kafka-order-service/internal/domain/entities"
	"kafka-order-service/internal/domain/repositories"

	"github.com/google/uuid"
)

// CreateOrderRequest представляет запрос на создание заказа
type CreateOrderRequest struct {
	CustomerID uuid.UUID                `json:"customer_id" validate:"required"`
	Email      string                   `json:"email" validate:"required,email"`
	Items      []CreateOrderItemRequest `json:"items" validate:"required,min=1"`
	Currency   string                   `json:"currency,omitempty"`
	Metadata   map[string]interface{}   `json:"metadata,omitempty"`

	// Адреса (опционально)
	ShippingAddress *CreateAddressRequest `json:"shipping_address,omitempty"`
	BillingAddress  *CreateAddressRequest `json:"billing_address,omitempty"`
}

// CreateOrderItemRequest представляет элемент в запросе создания заказа
type CreateOrderItemRequest struct {
	ProductID uuid.UUID `json:"product_id" validate:"required"`
	Name      string    `json:"name" validate:"required"`
	Price     float64   `json:"price" validate:"required,gt=0"`
	Quantity  int       `json:"quantity" validate:"required,gt=0"`
}

// CreateAddressRequest представляет адрес в запросе
type CreateAddressRequest struct {
	Street  string `json:"street" validate:"required"`
	City    string `json:"city" validate:"required"`
	State   string `json:"state"`
	Country string `json:"country" validate:"required"`
	ZipCode string `json:"zip_code" validate:"required"`
}

// CreateOrderResponse представляет ответ создания заказа
type CreateOrderResponse struct {
	Order   *entities.Order `json:"order"`
	Message string          `json:"message"`
}

// EventPublisher интерфейс для публикации событий в Kafka
type EventPublisher interface {
	PublishOrderEvent(ctx context.Context, event *entities.OrderEvent) error
}

// Logger интерфейс для логирования
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// CreateOrderUseCase представляет use case создания заказа
type CreateOrderUseCase struct {
	orderRepo repositories.OrderRepository
	publisher EventPublisher
	logger    Logger
}

// NewCreateOrderUseCase создает новый use case для создания заказа
func NewCreateOrderUseCase(
	orderRepo repositories.OrderRepository,
	publisher EventPublisher,
	logger Logger,
) *CreateOrderUseCase {
	return &CreateOrderUseCase{
		orderRepo: orderRepo,
		publisher: publisher,
		logger:    logger,
	}
}

// Execute выполняет создание заказа
func (uc *CreateOrderUseCase) Execute(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	// Валидация запроса
	if err := uc.validateRequest(req); err != nil {
		uc.logger.Error("Invalid create order request", "error", err, "customer_id", req.CustomerID)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Создание заказа
	order := entities.NewOrder(req.CustomerID, req.Email)

	// Установка валюты если указана
	if req.Currency != "" {
		order.Currency = req.Currency
	}

	// Добавление метаданных
	if req.Metadata != nil {
		for key, value := range req.Metadata {
			order.Metadata[key] = value
		}
	}

	// Добавление элементов заказа
	for _, item := range req.Items {
		order.AddItem(item.ProductID, item.Name, item.Price, item.Quantity)
	}

	// Добавление адресов
	if req.ShippingAddress != nil {
		address := &entities.Address{
			Street:  req.ShippingAddress.Street,
			City:    req.ShippingAddress.City,
			State:   req.ShippingAddress.State,
			Country: req.ShippingAddress.Country,
			ZipCode: req.ShippingAddress.ZipCode,
		}
		order.SetShippingAddress(address)
	}

	if req.BillingAddress != nil {
		address := &entities.Address{
			Street:  req.BillingAddress.Street,
			City:    req.BillingAddress.City,
			State:   req.BillingAddress.State,
			Country: req.BillingAddress.Country,
			ZipCode: req.BillingAddress.ZipCode,
		}
		order.SetBillingAddress(address)
	}

	// Финальная валидация заказа
	if err := order.Validate(); err != nil {
		uc.logger.Error("Order validation failed", "error", err, "order_id", order.ID)
		return nil, fmt.Errorf("order validation failed: %w", err)
	}

	// Сохранение заказа в базе данных
	if err := uc.orderRepo.Create(ctx, order); err != nil {
		uc.logger.Error("Failed to create order in database", "error", err, "order_id", order.ID)
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	uc.logger.Info("Order created successfully",
		"order_id", order.ID,
		"customer_id", order.CustomerID,
		"total_amount", order.TotalAmount,
		"items_count", len(order.Items))

	// Публикация события в Kafka
	event := order.ToEvent(entities.EventOrderCreated)
	if err := uc.publisher.PublishOrderEvent(ctx, event); err != nil {
		// Событие не критично, логируем ошибку но не возвращаем её
		uc.logger.Error("Failed to publish order created event",
			"error", err,
			"order_id", order.ID,
			"event_id", event.EventID)
	} else {
		uc.logger.Info("Order created event published",
			"order_id", order.ID,
			"event_id", event.EventID)
	}

	return &CreateOrderResponse{
		Order:   order,
		Message: "Order created successfully",
	}, nil
}

// validateRequest валидирует входящий запрос
func (uc *CreateOrderUseCase) validateRequest(req *CreateOrderRequest) error {
	if req == nil {
		return entities.NewValidationError("request cannot be nil")
	}

	if req.CustomerID == uuid.Nil {
		return entities.NewValidationError("customer_id is required")
	}

	if req.Email == "" {
		return entities.NewValidationError("email is required")
	}

	// Простая валидация email
	if !validateEmail(req.Email) {
		return entities.NewValidationError("invalid email format")
	}

	if len(req.Items) == 0 {
		return entities.NewValidationError("at least one item is required")
	}

	// Валидация элементов
	for i, item := range req.Items {
		if item.ProductID == uuid.Nil {
			return entities.NewValidationError("item %d: product_id is required", i)
		}
		if item.Name == "" {
			return entities.NewValidationError("item %d: name is required", i)
		}
		if item.Price <= 0 {
			return entities.NewValidationError("item %d: price must be greater than 0", i)
		}
		if item.Quantity <= 0 {
			return entities.NewValidationError("item %d: quantity must be greater than 0", i)
		}
	}

	return nil
}

// contains проверяет содержит ли строка подстроку
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(substr) == 0 || s[len(s)-len(substr):] == substr ||
			s[:len(substr)] == substr ||
			s != s[:len(substr)]+s[len(substr):])
}

// Упрощенная версия contains для проверки @
func containsAt(email string) bool {
	for _, char := range email {
		if char == '@' {
			return true
		}
	}
	return false
}

// Заменяем contains на containsAt в validateRequest
func validateEmail(email string) bool {
	if len(email) < 5 {
		return false
	}
	if !strings.Contains(email, "@") {
		return false
	}
	parts := strings.Split(email, "@")
	return len(parts) == 2 && len(parts[1]) >= 3 && strings.Contains(parts[1], ".")
}
