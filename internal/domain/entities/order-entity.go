package entities

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus представляет статус заказа
type OrderStatus string

// Возможные статусы заказа
const (
	OrderStatusPending    OrderStatus = "pending"     // Ожидает обработки
	OrderStatusConfirmed  OrderStatus = "confirmed"   // Подтвержден
	OrderStatusProcessing OrderStatus = "processing"  // В обработке
	OrderStatusShipped    OrderStatus = "shipped"     // Отправлен
	OrderStatusDelivered  OrderStatus = "delivered"   // Доставлен
	OrderStatusCancelled  OrderStatus = "cancelled"   // Отменен
	OrderStatusRefunded   OrderStatus = "refunded"    // Возврат
)

// OrderItem представляет элемент заказа
type OrderItem struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrderID   uuid.UUID `json:"order_id" db:"order_id"`
	ProductID uuid.UUID `json:"product_id" db:"product_id"`
	Name      string    `json:"name" db:"name"`
	Price     float64   `json:"price" db:"price"`
	Quantity  int       `json:"quantity" db:"quantity"`
	Total     float64   `json:"total" db:"total"`
}

// Order представляет заказ в системе
type Order struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	CustomerID  uuid.UUID     `json:"customer_id" db:"customer_id"`
	Email       string        `json:"email" db:"email"`
	Status      OrderStatus   `json:"status" db:"status"`
	TotalAmount float64       `json:"total_amount" db:"total_amount"`
	Currency    string        `json:"currency" db:"currency"`
	Items       []OrderItem   `json:"items" db:"-"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
	
	// Дополнительные поля для доставки
	ShippingAddress *Address `json:"shipping_address,omitempty" db:"-"`
	BillingAddress  *Address `json:"billing_address,omitempty" db:"-"`
	
	// Метаданные
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"-"`
}

// Address представляет адрес доставки/выставления счета
type Address struct {
	ID       uuid.UUID `json:"id" db:"id"`
	OrderID  uuid.UUID `json:"order_id" db:"order_id"`
	Type     string    `json:"type" db:"type"` // "shipping" или "billing"
	Street   string    `json:"street" db:"street"`
	City     string    `json:"city" db:"city"`
	State    string    `json:"state" db:"state"`
	Country  string    `json:"country" db:"country"`
	ZipCode  string    `json:"zip_code" db:"zip_code"`
}

// OrderEvent представляет событие заказа для Kafka
type OrderEvent struct {
	EventType   string                 `json:"event_type"`
	EventID     uuid.UUID              `json:"event_id"`
	OrderID     uuid.UUID              `json:"order_id"`
	CustomerID  uuid.UUID              `json:"customer_id"`
	Status      OrderStatus            `json:"status"`
	TotalAmount float64                `json:"total_amount"`
	Currency    string                 `json:"currency"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// Константы для типов событий
const (
	EventOrderCreated   = "order.created"
	EventOrderConfirmed = "order.confirmed"
	EventOrderCancelled = "order.cancelled"
	EventOrderShipped   = "order.shipped"
	EventOrderDelivered = "order.delivered"
	EventOrderRefunded  = "order.refunded"
)

// NewOrder создает новый заказ
func NewOrder(customerID uuid.UUID, email string) *Order {
	now := time.Now()
	return &Order{
		ID:          uuid.New(),
		CustomerID:  customerID,
		Email:       email,
		Status:      OrderStatusPending,
		Currency:    "USD",
		Items:       make([]OrderItem, 0),
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    make(map[string]interface{}),
	}
}

// AddItem добавляет элемент к заказу
func (o *Order) AddItem(productID uuid.UUID, name string, price float64, quantity int) {
	item := OrderItem{
		ID:        uuid.New(),
		OrderID:   o.ID,
		ProductID: productID,
		Name:      name,
		Price:     price,
		Quantity:  quantity,
		Total:     price * float64(quantity),
	}
	
	o.Items = append(o.Items, item)
	o.calculateTotal()
	o.UpdatedAt = time.Now()
}

// RemoveItem удаляет элемент из заказа
func (o *Order) RemoveItem(itemID uuid.UUID) bool {
	for i, item := range o.Items {
		if item.ID == itemID {
			o.Items = append(o.Items[:i], o.Items[i+1:]...)
			o.calculateTotal()
			o.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// UpdateStatus обновляет статус заказа
func (o *Order) UpdateStatus(newStatus OrderStatus) error {
	if !o.canTransitionTo(newStatus) {
		return NewInvalidStatusTransitionError(o.Status, newStatus)
	}
	
	o.Status = newStatus
	o.UpdatedAt = time.Now()
	return nil
}

// canTransitionTo проверяет возможность перехода к новому статусу
func (o *Order) canTransitionTo(newStatus OrderStatus) bool {
	validTransitions := map[OrderStatus][]OrderStatus{
		OrderStatusPending: {
			OrderStatusConfirmed,
			OrderStatusCancelled,
		},
		OrderStatusConfirmed: {
			OrderStatusProcessing,
			OrderStatusCancelled,
		},
		OrderStatusProcessing: {
			OrderStatusShipped,
			OrderStatusCancelled,
		},
		OrderStatusShipped: {
			OrderStatusDelivered,
		},
		OrderStatusDelivered: {
			OrderStatusRefunded,
		},
		OrderStatusCancelled: {}, // Финальный статус
		OrderStatusRefunded:  {}, // Финальный статус
	}

	allowedStatuses, exists := validTransitions[o.Status]
	if !exists {
		return false
	}

	for _, status := range allowedStatuses {
		if status == newStatus {
			return true
		}
	}
	return false
}

// calculateTotal пересчитывает общую сумму заказа
func (o *Order) calculateTotal() {
	total := 0.0
	for _, item := range o.Items {
		total += item.Total
	}
	o.TotalAmount = total
}

// IsActive проверяет, активен ли заказ (не отменен и не завершен)
func (o *Order) IsActive() bool {
	return o.Status != OrderStatusCancelled && 
		   o.Status != OrderStatusRefunded &&
		   o.Status != OrderStatusDelivered
}

// IsFinal проверяет, находится ли заказ в финальном состоянии
func (o *Order) IsFinal() bool {
	return o.Status == OrderStatusCancelled ||
		   o.Status == OrderStatusRefunded ||
		   o.Status == OrderStatusDelivered
}

// GetItemCount возвращает общее количество товаров в заказе
func (o *Order) GetItemCount() int {
	count := 0
	for _, item := range o.Items {
		count += item.Quantity
	}
	return count
}

// SetShippingAddress устанавливает адрес доставки
func (o *Order) SetShippingAddress(address *Address) {
	address.ID = uuid.New()
	address.OrderID = o.ID
	address.Type = "shipping"
	o.ShippingAddress = address
	o.UpdatedAt = time.Now()
}

// SetBillingAddress устанавливает адрес выставления счета
func (o *Order) SetBillingAddress(address *Address) {
	address.ID = uuid.New()
	address.OrderID = o.ID
	address.Type = "billing"
	o.BillingAddress = address
	o.UpdatedAt = time.Now()
}

// ToEvent создает событие заказа для отправки в Kafka
func (o *Order) ToEvent(eventType string) *OrderEvent {
	return &OrderEvent{
		EventType:   eventType,
		EventID:     uuid.New(),
		OrderID:     o.ID,
		CustomerID:  o.CustomerID,
		Status:      o.Status,
		TotalAmount: o.TotalAmount,
		Currency:    o.Currency,
		Timestamp:   time.Now(),
		Data: map[string]interface{}{
			"email":      o.Email,
			"item_count": o.GetItemCount(),
		},
	}
}

// Validate выполняет валидацию заказа
func (o *Order) Validate() error {
	if o.ID == uuid.Nil {
		return NewValidationError("order ID cannot be empty")
	}
	
	if o.CustomerID == uuid.Nil {
		return NewValidationError("customer ID cannot be empty")
	}
	
	if o.Email == "" {
		return NewValidationError("email cannot be empty")
	}
	
	if len(o.Items) == 0 {
		return NewValidationError("order must have at least one item")
	}
	
	if o.TotalAmount <= 0 {
		return NewValidationError("total amount must be greater than zero")
	}
	
	// Валидация элементов заказа
	for i, item := range o.Items {
		if err := item.Validate(); err != nil {
			return NewValidationError("item %d: %s", i, err.Error())
		}
	}
	
	return nil
}

// Validate выполняет валидацию элемента заказа
func (item *OrderItem) Validate() error {
	if item.ID == uuid.Nil {
		return NewValidationError("item ID cannot be empty")
	}
	
	if item.ProductID == uuid.Nil {
		return NewValidationError("product ID cannot be empty")
	}
	
	if item.Name == "" {
		return NewValidationError("item name cannot be empty")
	}
	
	if item.Price <= 0 {
		return NewValidationError("item price must be greater than zero")
	}
	
	if item.Quantity <= 0 {
		return NewValidationError("item quantity must be greater than zero")
	}
	
	expectedTotal := item.Price * float64(item.Quantity)
	if item.Total != expectedTotal {
		return NewValidationError("item total (%f) doesn't match price * quantity (%f)", 
			item.Total, expectedTotal)
	}
	
	return nil
}