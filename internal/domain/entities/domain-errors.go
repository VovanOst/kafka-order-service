package entities

import "fmt"

// DomainError представляет базовую ошибку домена
type DomainError struct {
	Type    string
	Message string
}

func (e DomainError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	DomainError
}

// NewValidationError создает новую ошибку валидации
func NewValidationError(format string, args ...interface{}) error {
	return ValidationError{
		DomainError: DomainError{
			Type:    "VALIDATION_ERROR",
			Message: fmt.Sprintf(format, args...),
		},
	}
}

// InvalidStatusTransitionError представляет ошибку перехода статуса
type InvalidStatusTransitionError struct {
	DomainError
	FromStatus OrderStatus
	ToStatus   OrderStatus
}

// NewInvalidStatusTransitionError создает новую ошибку перехода статуса
func NewInvalidStatusTransitionError(from, to OrderStatus) error {
	return InvalidStatusTransitionError{
		DomainError: DomainError{
			Type:    "INVALID_STATUS_TRANSITION",
			Message: fmt.Sprintf("cannot transition from %s to %s", from, to),
		},
		FromStatus: from,
		ToStatus:   to,
	}
}

// OrderNotFoundError представляет ошибку "заказ не найден"
type OrderNotFoundError struct {
	DomainError
	OrderID string
}

// NewOrderNotFoundError создает новую ошибку "заказ не найден"
func NewOrderNotFoundError(orderID string) error {
	return OrderNotFoundError{
		DomainError: DomainError{
			Type:    "ORDER_NOT_FOUND",
			Message: fmt.Sprintf("order with ID %s not found", orderID),
		},
		OrderID: orderID,
	}
}