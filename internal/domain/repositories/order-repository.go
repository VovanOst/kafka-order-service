package repositories

import (
	"context"

	"kafka-order-service/internal/domain/entities"

	"github.com/google/uuid"
)

// OrderRepository определяет интерфейс для работы с заказами
type OrderRepository interface {
	// Create создает новый заказ
	Create(ctx context.Context, order *entities.Order) error

	// GetByID получает заказ по ID
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Order, error)

	// Update обновляет заказ
	Update(ctx context.Context, order *entities.Order) error

	// UpdateStatus обновляет только статус заказа
	UpdateStatus(ctx context.Context, id uuid.UUID, status entities.OrderStatus) error

	// Delete удаляет заказ (мягкое удаление)
	Delete(ctx context.Context, id uuid.UUID) error

	// List получает список заказов с пагинацией и фильтрацией
	List(ctx context.Context, filters OrderFilters) ([]*entities.Order, error)

	// GetByCustomerID получает заказы конкретного клиента
	GetByCustomerID(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*entities.Order, error)

	// GetByStatus получает заказы по статусу
	GetByStatus(ctx context.Context, status entities.OrderStatus, limit, offset int) ([]*entities.Order, error)

	// Count возвращает общее количество заказов
	Count(ctx context.Context, filters OrderFilters) (int64, error)

	// Exists проверяет существование заказа
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// OrderFilters представляет фильтры для поиска заказов
type OrderFilters struct {
	CustomerID *uuid.UUID            `json:"customer_id,omitempty"`
	Status     *entities.OrderStatus `json:"status,omitempty"`
	Email      *string               `json:"email,omitempty"`
	MinAmount  *float64              `json:"min_amount,omitempty"`
	MaxAmount  *float64              `json:"max_amount,omitempty"`
	DateFrom   *string               `json:"date_from,omitempty"` // RFC3339 format
	DateTo     *string               `json:"date_to,omitempty"`   // RFC3339 format
	Currency   *string               `json:"currency,omitempty"`

	// Пагинация
	Limit  int `json:"limit" default:"20"`
	Offset int `json:"offset" default:"0"`

	// Сортировка
	SortBy    string `json:"sort_by" default:"created_at"`
	SortOrder string `json:"sort_order" default:"desc"` // "asc" or "desc"
}

// OrderItemRepository определяет интерфейс для работы с элементами заказов
type OrderItemRepository interface {
	// CreateItems создает элементы заказа
	CreateItems(ctx context.Context, items []entities.OrderItem) error

	// GetByOrderID получает элементы заказа по ID заказа
	GetByOrderID(ctx context.Context, orderID uuid.UUID) ([]entities.OrderItem, error)

	// UpdateItem обновляет элемент заказа
	UpdateItem(ctx context.Context, item *entities.OrderItem) error

	// DeleteItem удаляет элемент заказа
	DeleteItem(ctx context.Context, itemID uuid.UUID) error

	// DeleteByOrderID удаляет все элементы заказа
	DeleteByOrderID(ctx context.Context, orderID uuid.UUID) error
}
