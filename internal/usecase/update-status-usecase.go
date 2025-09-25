package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"kafka-order-service/internal/domain/entities"
	"kafka-order-service/internal/domain/repositories"
)

// UpdateOrderStatusRequest представляет запрос на обновление статуса заказа
type UpdateOrderStatusRequest struct {
	OrderID   uuid.UUID             `json:"order_id" validate:"required"`
	NewStatus entities.OrderStatus  `json:"new_status" validate:"required"`
	Reason    string               `json:"reason,omitempty"`
}

// UpdateOrderStatusResponse представляет ответ обновления статуса
type UpdateOrderStatusResponse struct {
	Order     *entities.Order `json:"order"`
	Message   string         `json:"message"`
	OldStatus entities.OrderStatus `json:"old_status"`
	NewStatus entities.OrderStatus `json:"new_status"`
}

// UpdateOrderStatusUseCase представляет use case обновления статуса заказа
type UpdateOrderStatusUseCase struct {
	orderRepo repositories.OrderRepository
	publisher EventPublisher
	logger    Logger
}

// NewUpdateOrderStatusUseCase создает новый use case для обновления статуса
func NewUpdateOrderStatusUseCase(
	orderRepo repositories.OrderRepository,
	publisher EventPublisher,
	logger Logger,
) *UpdateOrderStatusUseCase {
	return &UpdateOrderStatusUseCase{
		orderRepo: orderRepo,
		publisher: publisher,
		logger:    logger,
	}
}

// Execute выполняет обновление статуса заказа
func (uc *UpdateOrderStatusUseCase) Execute(ctx context.Context, req *UpdateOrderStatusRequest) (*UpdateOrderStatusResponse, error) {
	// Валидация запроса
	if err := uc.validateRequest(req); err != nil {
		uc.logger.Error("Invalid update status request", "error", err, "order_id", req.OrderID)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Получение заказа
	order, err := uc.orderRepo.GetByID(ctx, req.OrderID)
	if err != nil {
		uc.logger.Error("Failed to get order", "error", err, "order_id", req.OrderID)
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Сохраняем старый статус для ответа
	oldStatus := order.Status

	// Обновляем статус
	if err := order.UpdateStatus(req.NewStatus); err != nil {
		uc.logger.Error("Failed to update order status", 
			"error", err, 
			"order_id", req.OrderID,
			"old_status", oldStatus,
			"new_status", req.NewStatus)
		return nil, fmt.Errorf("status update failed: %w", err)
	}

	// Добавляем причину в метаданные если указана
	if req.Reason != "" {
		if order.Metadata == nil {
			order.Metadata = make(map[string]interface{})
		}
		order.Metadata["status_change_reason"] = req.Reason
	}

	// Сохранение обновленного заказа
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		uc.logger.Error("Failed to update order in database", "error", err, "order_id", req.OrderID)
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	uc.logger.Info("Order status updated successfully",
		"order_id", order.ID,
		"old_status", oldStatus,
		"new_status", order.Status,
		"reason", req.Reason)

	// Публикуем событие в зависимости от нового статуса
	var eventType string
	switch req.NewStatus {
	case entities.OrderStatusConfirmed:
		eventType = entities.EventOrderConfirmed
	case entities.OrderStatusCancelled:
		eventType = entities.EventOrderCancelled
	case entities.OrderStatusShipped:
		eventType = entities.EventOrderShipped
	case entities.OrderStatusDelivered:
		eventType = entities.EventOrderDelivered
	case entities.OrderStatusRefunded:
		eventType = entities.EventOrderRefunded
	default:
		eventType = "order.status_changed"
	}

	// Создаем и публикуем событие
	event := order.ToEvent(eventType)
	event.Data["old_status"] = string(oldStatus)
	event.Data["change_reason"] = req.Reason

	if err := uc.publisher.PublishOrderEvent(ctx, event); err != nil {
		uc.logger.Error("Failed to publish order status event",
			"error", err,
			"order_id", order.ID,
			"event_type", eventType,
			"event_id", event.EventID)
	} else {
		uc.logger.Info("Order status event published",
			"order_id", order.ID,
			"event_type", eventType,
			"event_id", event.EventID)
	}

	return &UpdateOrderStatusResponse{
		Order:     order,
		Message:   fmt.Sprintf("Order status updated from %s to %s", oldStatus, req.NewStatus),
		OldStatus: oldStatus,
		NewStatus: req.NewStatus,
	}, nil
}

// validateRequest валидирует входящий запрос
func (uc *UpdateOrderStatusUseCase) validateRequest(req *UpdateOrderStatusRequest) error {
	if req == nil {
		return entities.NewValidationError("request cannot be nil")
	}

	if req.OrderID == uuid.Nil {
		return entities.NewValidationError("order_id is required")
	}

	if req.NewStatus == "" {
		return entities.NewValidationError("new_status is required")
	}

	// Валидация что статус является валидным
	validStatuses := []entities.OrderStatus{
		entities.OrderStatusPending,
		entities.OrderStatusConfirmed,
		entities.OrderStatusProcessing,
		entities.OrderStatusShipped,
		entities.OrderStatusDelivered,
		entities.OrderStatusCancelled,
		entities.OrderStatusRefunded,
	}

	isValid := false
	for _, validStatus := range validStatuses {
		if req.NewStatus == validStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		return entities.NewValidationError("invalid status: %s", req.NewStatus)
	}

	return nil
}