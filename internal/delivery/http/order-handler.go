package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"kafka-order-service/internal/domain/entities"
	"kafka-order-service/internal/usecase"
	"kafka-order-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// OrderHandler обрабатывает HTTP запросы для заказов
type OrderHandler struct {
	createOrderUC  *usecase.CreateOrderUseCase
	updateStatusUC *usecase.UpdateOrderStatusUseCase
	getOrderUC     *usecase.GetOrderUseCase
	listOrdersUC   *usecase.ListOrdersUseCase
	logger         *logger.Logger
}

// NewOrderHandler создает новый handler для заказов
func NewOrderHandler(
	createOrderUC *usecase.CreateOrderUseCase,
	updateStatusUC *usecase.UpdateOrderStatusUseCase,
	getOrderUC *usecase.GetOrderUseCase,
	listOrdersUC *usecase.ListOrdersUseCase,
	logger *logger.Logger,
) *OrderHandler {
	return &OrderHandler{
		createOrderUC:  createOrderUC,
		updateStatusUC: updateStatusUC,
		getOrderUC:     getOrderUC,
		listOrdersUC:   listOrdersUC,
		logger:         logger,
	}
}

// CreateOrder создает новый заказ
// POST /api/v1/orders
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Creating order request received")

	var req usecase.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode create order request", "error", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	response, err := h.createOrderUC.Execute(r.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create order", "error", err, "customer_id", req.CustomerID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create order", err)
		return
	}

	h.logger.Info("Order created successfully", "order_id", response.Order.ID)
	h.writeJSONResponse(w, http.StatusCreated, response)
}

// GetOrder получает заказ по ID
// GET /api/v1/orders/{id}
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderIDStr := vars["id"]

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		h.logger.Error("Invalid order ID format", "order_id", orderIDStr, "error", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid order ID format", err)
		return
	}

	h.logger.Info("Getting order", "order_id", orderID)

	req := &usecase.GetOrderRequest{
		OrderID: orderID,
	}

	response, err := h.getOrderUC.Execute(r.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get order", "error", err, "order_id", orderID)
		h.writeErrorResponse(w, http.StatusNotFound, "Order not found", err)
		return
	}

	h.logger.Info("Order retrieved successfully", "order_id", orderID)
	h.writeJSONResponse(w, http.StatusOK, response)
}

// UpdateOrderStatus обновляет статус заказа
// PUT /api/v1/orders/{id}/status
func (h *OrderHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderIDStr := vars["id"]

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		h.logger.Error("Invalid order ID format", "order_id", orderIDStr, "error", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid order ID format", err)
		return
	}

	var requestBody struct {
		NewStatus string `json:"new_status"`
		Reason    string `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		h.logger.Error("Failed to decode update status request", "error", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	h.logger.Info("Updating order status", "order_id", orderID, "new_status", requestBody.NewStatus)

	req := &usecase.UpdateOrderStatusRequest{
		OrderID:   orderID,
		NewStatus: entities.OrderStatus(requestBody.NewStatus),
		Reason:    requestBody.Reason,
	}

	response, err := h.updateStatusUC.Execute(r.Context(), req)
	if err != nil {
		h.logger.Error("Failed to update order status",
			"error", err,
			"order_id", orderID,
			"new_status", requestBody.NewStatus)
		h.writeErrorResponse(w, http.StatusBadRequest, "Failed to update order status", err)
		return
	}

	h.logger.Info("Order status updated successfully",
		"order_id", orderID,
		"old_status", response.OldStatus,
		"new_status", response.NewStatus)
	h.writeJSONResponse(w, http.StatusOK, response)
}

// ListOrders получает список заказов с фильтрацией
// GET /api/v1/orders
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Listing orders request received")

	req := &usecase.ListOrdersRequest{}

	// Парсинг query параметров
	query := r.URL.Query()

	// Customer ID
	if customerIDStr := query.Get("customer_id"); customerIDStr != "" {
		if customerID, err := uuid.Parse(customerIDStr); err == nil {
			req.CustomerID = &customerID
		}
	}

	// Status
	if status := query.Get("status"); status != "" {
		orderStatus := entities.OrderStatus(status)
		req.Status = &orderStatus
	}

	// Email
	if email := query.Get("email"); email != "" {
		req.Email = &email
	}

	// Currency
	if currency := query.Get("currency"); currency != "" {
		req.Currency = &currency
	}

	// Min/Max Amount
	if minAmountStr := query.Get("min_amount"); minAmountStr != "" {
		if minAmount, err := strconv.ParseFloat(minAmountStr, 64); err == nil {
			req.MinAmount = &minAmount
		}
	}

	if maxAmountStr := query.Get("max_amount"); maxAmountStr != "" {
		if maxAmount, err := strconv.ParseFloat(maxAmountStr, 64); err == nil {
			req.MaxAmount = &maxAmount
		}
	}

	// Date range
	if dateFrom := query.Get("date_from"); dateFrom != "" {
		req.DateFrom = &dateFrom
	}

	if dateTo := query.Get("date_to"); dateTo != "" {
		req.DateTo = &dateTo
	}

	// Pagination
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			req.Limit = limit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			req.Offset = offset
		}
	}

	// Sorting
	if sortBy := query.Get("sort_by"); sortBy != "" {
		req.SortBy = sortBy
	}

	if sortOrder := query.Get("sort_order"); sortOrder != "" {
		req.SortOrder = sortOrder
	}

	response, err := h.listOrdersUC.Execute(r.Context(), req)
	if err != nil {
		h.logger.Error("Failed to list orders", "error", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to list orders", err)
		return
	}

	h.logger.Info("Orders listed successfully",
		"count", len(response.Orders),
		"total_count", response.TotalCount)
	h.writeJSONResponse(w, http.StatusOK, response)
}

// HealthCheck проверка здоровья сервиса
// GET /health
func (h *OrderHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "kafka-order-service",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Metrics возвращает метрики сервиса
// GET /metrics
func (h *OrderHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	// В реальном приложении здесь будут метрики из Prometheus/monitoring
	response := map[string]interface{}{
		"service":             "kafka-order-service",
		"uptime_seconds":      "placeholder", // Можно добавить реальный uptime
		"requests_total":      "placeholder", // Счетчик запросов
		"errors_total":        "placeholder", // Счетчик ошибок
		"kafka_messages_sent": "placeholder", // Kafka метрики
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Helper methods

// writeJSONResponse записывает JSON ответ
func (h *OrderHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// writeErrorResponse записывает ошибку в JSON формате
func (h *OrderHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	response := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if err != nil {
		response["details"] = err.Error()
	}

	h.writeJSONResponse(w, statusCode, response)
}

// ErrorResponse структура для ошибок API
type ErrorResponse struct {
	Error     string `json:"error"`
	Details   string `json:"details,omitempty"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
}

// SuccessResponse структура для успешных ответов
type SuccessResponse struct {
	Data      interface{} `json:"data"`
	Message   string      `json:"message,omitempty"`
	Timestamp string      `json:"timestamp"`
}

// wrapSuccessResponse оборачивает данные в стандартный формат ответа
func (h *OrderHandler) wrapSuccessResponse(data interface{}, message string) SuccessResponse {
	return SuccessResponse{
		Data:      data,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
