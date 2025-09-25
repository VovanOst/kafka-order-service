package entities

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewOrder(t *testing.T) {
	customerID := uuid.New()
	email := "test@example.com"

	order := NewOrder(customerID, email)

	if order.ID == uuid.Nil {
		t.Error("Expected non-nil order ID")
	}
	
	if order.CustomerID != customerID {
		t.Errorf("Expected customer ID %v, got %v", customerID, order.CustomerID)
	}
	
	if order.Email != email {
		t.Errorf("Expected email %s, got %s", email, order.Email)
	}
	
	if order.Status != OrderStatusPending {
		t.Errorf("Expected status %s, got %s", OrderStatusPending, order.Status)
	}
	
	if order.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", order.Currency)
	}
	
	if order.TotalAmount != 0.0 {
		t.Errorf("Expected total amount 0.0, got %f", order.TotalAmount)
	}
	
	if len(order.Items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(order.Items))
	}
}

func TestOrder_AddItem(t *testing.T) {
	order := NewOrder(uuid.New(), "test@example.com")
	productID := uuid.New()
	
	order.AddItem(productID, "Test Product", 10.99, 2)

	if len(order.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(order.Items))
	}
	
	if order.TotalAmount != 21.98 {
		t.Errorf("Expected total amount 21.98, got %f", order.TotalAmount)
	}

	item := order.Items[0]
	if item.ProductID != productID {
		t.Errorf("Expected product ID %v, got %v", productID, item.ProductID)
	}
	
	if item.Name != "Test Product" {
		t.Errorf("Expected name 'Test Product', got %s", item.Name)
	}
	
	if item.Price != 10.99 {
		t.Errorf("Expected price 10.99, got %f", item.Price)
	}
	
	if item.Quantity != 2 {
		t.Errorf("Expected quantity 2, got %d", item.Quantity)
	}
	
	if item.Total != 21.98 {
		t.Errorf("Expected item total 21.98, got %f", item.Total)
	}
}

func TestOrder_RemoveItem(t *testing.T) {
	order := NewOrder(uuid.New(), "test@example.com")
	
	// Добавляем два элемента
	order.AddItem(uuid.New(), "Product 1", 10.0, 1)
	order.AddItem(uuid.New(), "Product 2", 20.0, 1)
	
	if len(order.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(order.Items))
	}
	
	if order.TotalAmount != 30.0 {
		t.Errorf("Expected total amount 30.0, got %f", order.TotalAmount)
	}

	// Удаляем первый элемент
	removed := order.RemoveItem(order.Items[0].ID)
	
	if !removed {
		t.Error("Expected item to be removed")
	}
	
	if len(order.Items) != 1 {
		t.Errorf("Expected 1 item after removal, got %d", len(order.Items))
	}
	
	if order.TotalAmount != 20.0 {
		t.Errorf("Expected total amount 20.0 after removal, got %f", order.TotalAmount)
	}
	
	// Пытаемся удалить несуществующий элемент
	removed = order.RemoveItem(uuid.New())
	if removed {
		t.Error("Expected false when removing non-existent item")
	}
}

func TestOrder_UpdateStatus(t *testing.T) {
	order := NewOrder(uuid.New(), "test@example.com")

	// Валидный переход: pending -> confirmed
	err := order.UpdateStatus(OrderStatusConfirmed)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if order.Status != OrderStatusConfirmed {
		t.Errorf("Expected status confirmed, got %s", order.Status)
	}

	// Валидный переход: confirmed -> processing
	err = order.UpdateStatus(OrderStatusProcessing)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if order.Status != OrderStatusProcessing {
		t.Errorf("Expected status processing, got %s", order.Status)
	}

	// Невалидный переход: processing -> pending
	err = order.UpdateStatus(OrderStatusPending)
	if err == nil {
		t.Error("Expected error for invalid status transition")
	}
	
	if order.Status != OrderStatusProcessing {
		t.Errorf("Expected status to remain processing, got %s", order.Status)
	}
}

func TestOrder_StatusTransitions(t *testing.T) {
	testCases := []struct {
		name          string
		currentStatus OrderStatus
		targetStatus  OrderStatus
		shouldSucceed bool
	}{
		{"pending to confirmed", OrderStatusPending, OrderStatusConfirmed, true},
		{"pending to cancelled", OrderStatusPending, OrderStatusCancelled, true},
		{"pending to processing", OrderStatusPending, OrderStatusProcessing, false},
		{"confirmed to processing", OrderStatusConfirmed, OrderStatusProcessing, true},
		{"confirmed to cancelled", OrderStatusConfirmed, OrderStatusCancelled, true},
		{"processing to shipped", OrderStatusProcessing, OrderStatusShipped, true},
		{"processing to cancelled", OrderStatusProcessing, OrderStatusCancelled, true},
		{"shipped to delivered", OrderStatusShipped, OrderStatusDelivered, true},
		{"shipped to cancelled", OrderStatusShipped, OrderStatusCancelled, false},
		{"delivered to refunded", OrderStatusDelivered, OrderStatusRefunded, true},
		{"cancelled to pending", OrderStatusCancelled, OrderStatusPending, false},
		{"refunded to pending", OrderStatusRefunded, OrderStatusPending, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			order := NewOrder(uuid.New(), "test@example.com")
			order.Status = tc.currentStatus
			
			err := order.UpdateStatus(tc.targetStatus)
			
			if tc.shouldSucceed {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if order.Status != tc.targetStatus {
					t.Errorf("Expected status %s, got %s", tc.targetStatus, order.Status)
				}
			} else {
				if err == nil {
					t.Error("Expected error for invalid transition")
				}
				if order.Status != tc.currentStatus {
					t.Errorf("Expected status to remain %s, got %s", tc.currentStatus, order.Status)
				}
			}
		})
	}
}

func TestOrder_IsActive(t *testing.T) {
	order := NewOrder(uuid.New(), "test@example.com")

	// Активные статусы
	activeStatuses := []OrderStatus{
		OrderStatusPending,
		OrderStatusConfirmed,
		OrderStatusProcessing,
		OrderStatusShipped,
	}

	for _, status := range activeStatuses {
		order.Status = status
		if !order.IsActive() {
			t.Errorf("Order should be active with status: %s", status)
		}
	}

	// Неактивные статусы
	inactiveStatuses := []OrderStatus{
		OrderStatusDelivered,
		OrderStatusCancelled,
		OrderStatusRefunded,
	}

	for _, status := range inactiveStatuses {
		order.Status = status
		if order.IsActive() {
			t.Errorf("Order should not be active with status: %s", status)
		}
	}
}

func TestOrder_IsFinal(t *testing.T) {
	order := NewOrder(uuid.New(), "test@example.com")

	finalStatuses := []OrderStatus{
		OrderStatusDelivered,
		OrderStatusCancelled,
		OrderStatusRefunded,
	}

	for _, status := range finalStatuses {
		order.Status = status
		if !order.IsFinal() {
			t.Errorf("Order should be final with status: %s", status)
		}
	}

	nonFinalStatuses := []OrderStatus{
		OrderStatusPending,
		OrderStatusConfirmed,
		OrderStatusProcessing,
		OrderStatusShipped,
	}

	for _, status := range nonFinalStatuses {
		order.Status = status
		if order.IsFinal() {
			t.Errorf("Order should not be final with status: %s", status)
		}
	}
}

func TestOrder_GetItemCount(t *testing.T) {
	order := NewOrder(uuid.New(), "test@example.com")

	if order.GetItemCount() != 0 {
		t.Errorf("Expected item count 0, got %d", order.GetItemCount())
	}

	order.AddItem(uuid.New(), "Product 1", 10.0, 2)
	if order.GetItemCount() != 2 {
		t.Errorf("Expected item count 2, got %d", order.GetItemCount())
	}

	order.AddItem(uuid.New(), "Product 2", 20.0, 3)
	if order.GetItemCount() != 5 {
		t.Errorf("Expected item count 5, got %d", order.GetItemCount())
	}
}

func TestOrder_Validate(t *testing.T) {
	t.Run("valid order", func(t *testing.T) {
		order := NewOrder(uuid.New(), "test@example.com")
		order.AddItem(uuid.New(), "Test Product", 10.0, 1)

		err := order.Validate()
		if err != nil {
			t.Errorf("Expected no error for valid order, got %v", err)
		}
	})

	t.Run("empty order ID", func(t *testing.T) {
		order := NewOrder(uuid.New(), "test@example.com")
		order.ID = uuid.Nil

		err := order.Validate()
		if err == nil {
			t.Error("Expected error for empty order ID")
		}
	})

	t.Run("empty customer ID", func(t *testing.T) {
		order := NewOrder(uuid.Nil, "test@example.com")

		err := order.Validate()
		if err == nil {
			t.Error("Expected error for empty customer ID")
		}
	})

	t.Run("empty email", func(t *testing.T) {
		order := NewOrder(uuid.New(), "")

		err := order.Validate()
		if err == nil {
			t.Error("Expected error for empty email")
		}
	})

	t.Run("no items", func(t *testing.T) {
		order := NewOrder(uuid.New(), "test@example.com")

		err := order.Validate()
		if err == nil {
			t.Error("Expected error for order with no items")
		}
	})
}

func TestOrder_ToEvent(t *testing.T) {
	order := NewOrder(uuid.New(), "test@example.com")
	order.AddItem(uuid.New(), "Test Product", 10.0, 2)

	event := order.ToEvent(EventOrderCreated)

	if event.EventType != EventOrderCreated {
		t.Errorf("Expected event type %s, got %s", EventOrderCreated, event.EventType)
	}
	
	if event.OrderID != order.ID {
		t.Errorf("Expected order ID %v, got %v", order.ID, event.OrderID)
	}
	
	if event.CustomerID != order.CustomerID {
		t.Errorf("Expected customer ID %v, got %v", order.CustomerID, event.CustomerID)
	}
	
	if event.Status != order.Status {
		t.Errorf("Expected status %s, got %s", order.Status, event.Status)
	}
	
	if event.TotalAmount != order.TotalAmount {
		t.Errorf("Expected total amount %f, got %f", order.TotalAmount, event.TotalAmount)
	}
	
	if event.Currency != order.Currency {
		t.Errorf("Expected currency %s, got %s", order.Currency, event.Currency)
	}
	
	if event.EventID == uuid.Nil {
		t.Error("Expected non-nil event ID")
	}
	
	// Проверяем что timestamp недавний (в пределах секунды)
	if time.Since(event.Timestamp) > time.Second {
		t.Error("Expected recent timestamp")
	}
	
	if event.Data["email"] != order.Email {
		t.Errorf("Expected email %s in data, got %v", order.Email, event.Data["email"])
	}
	
	if event.Data["item_count"] != 2 {
		t.Errorf("Expected item count 2 in data, got %v", event.Data["item_count"])
	}
}