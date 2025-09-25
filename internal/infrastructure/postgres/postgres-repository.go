package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"kafka-order-service/internal/domain/entities"
	"kafka-order-service/internal/domain/repositories"
)

// OrderRepository реализация репозитория заказов для PostgreSQL
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository создает новый репозиторий заказов
func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{
		db: db,
	}
}

// Create создает новый заказ
func (r *OrderRepository) Create(ctx context.Context, order *entities.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Вставка основной информации о заказе
	query := `
		INSERT INTO orders (
			id, customer_id, email, status, total_amount, currency, 
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.ExecContext(ctx, query,
		order.ID, order.CustomerID, order.Email, order.Status,
		order.TotalAmount, order.Currency, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	// Вставка элементов заказа
	if len(order.Items) > 0 {
		if err := r.insertOrderItems(ctx, tx, order.Items); err != nil {
			return fmt.Errorf("failed to insert order items: %w", err)
		}
	}

	// Вставка адресов
	if order.ShippingAddress != nil {
		if err := r.insertAddress(ctx, tx, order.ShippingAddress); err != nil {
			return fmt.Errorf("failed to insert shipping address: %w", err)
		}
	}

	if order.BillingAddress != nil {
		if err := r.insertAddress(ctx, tx, order.BillingAddress); err != nil {
			return fmt.Errorf("failed to insert billing address: %w", err)
		}
	}

	// Фиксация транзакции
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID получает заказ по ID
func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.Order, error) {
	// Получение основной информации о заказе
	query := `
		SELECT id, customer_id, email, status, total_amount, currency, 
			   created_at, updated_at
		FROM orders 
		WHERE id = $1`

	var order entities.Order
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&order.ID, &order.CustomerID, &order.Email, &order.Status,
		&order.TotalAmount, &order.Currency, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, entities.NewOrderNotFoundError(id.String())
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Получение элементов заказа
	items, err := r.getOrderItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	order.Items = items

	// Получение адресов
	addresses, err := r.getOrderAddresses(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order addresses: %w", err)
	}

	for _, addr := range addresses {
		if addr.Type == "shipping" {
			order.ShippingAddress = addr
		} else if addr.Type == "billing" {
			order.BillingAddress = addr
		}
	}

	// Инициализация метаданных
	order.Metadata = make(map[string]interface{})

	return &order, nil
}

// Update обновляет заказ
func (r *OrderRepository) Update(ctx context.Context, order *entities.Order) error {
	query := `
		UPDATE orders 
		SET customer_id = $2, email = $3, status = $4, total_amount = $5, 
			currency = $6, updated_at = $7
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		order.ID, order.CustomerID, order.Email, order.Status,
		order.TotalAmount, order.Currency, order.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return entities.NewOrderNotFoundError(order.ID.String())
	}

	return nil
}

// UpdateStatus обновляет только статус заказа
func (r *OrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entities.OrderStatus) error {
	query := `
		UPDATE orders 
		SET status = $2, updated_at = $3
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, status, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return entities.NewOrderNotFoundError(id.String())
	}

	return nil
}

// Delete удаляет заказ (мягкое удаление)
func (r *OrderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE orders 
		SET status = 'cancelled', updated_at = $2
		WHERE id = $1 AND status NOT IN ('delivered', 'refunded', 'cancelled')`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return entities.NewOrderNotFoundError(id.String())
	}

	return nil
}

// List получает список заказов с пагинацией и фильтрацией
func (r *OrderRepository) List(ctx context.Context, filters repositories.OrderFilters) ([]*entities.Order, error) {
	query, args := r.buildListQuery(filters)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute list query: %w", err)
	}
	defer rows.Close()

	var orders []*entities.Order
	for rows.Next() {
		var order entities.Order
		err := rows.Scan(
			&order.ID, &order.CustomerID, &order.Email, &order.Status,
			&order.TotalAmount, &order.Currency, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}

		// Инициализация метаданных и элементов
		order.Metadata = make(map[string]interface{})
		order.Items = make([]entities.OrderItem, 0)

		orders = append(orders, &order)
	}

	return orders, nil
}

// GetByCustomerID получает заказы конкретного клиента
func (r *OrderRepository) GetByCustomerID(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*entities.Order, error) {
	filters := repositories.OrderFilters{
		CustomerID: &customerID,
		Limit:      limit,
		Offset:     offset,
		SortBy:     "created_at",
		SortOrder:  "desc",
	}

	return r.List(ctx, filters)
}

// GetByStatus получает заказы по статусу
func (r *OrderRepository) GetByStatus(ctx context.Context, status entities.OrderStatus, limit, offset int) ([]*entities.Order, error) {
	filters := repositories.OrderFilters{
		Status:    &status,
		Limit:     limit,
		Offset:    offset,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	return r.List(ctx, filters)
}

// Count возвращает общее количество заказов
func (r *OrderRepository) Count(ctx context.Context, filters repositories.OrderFilters) (int64, error) {
	query, args := r.buildCountQuery(filters)

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders: %w", err)
	}

	return count, nil
}

// Exists проверяет существование заказа
func (r *OrderRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM orders WHERE id = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check order existence: %w", err)
	}

	return exists, nil
}

// Helper methods

// insertOrderItems вставляет элементы заказа
func (r *OrderRepository) insertOrderItems(ctx context.Context, tx *sql.Tx, items []entities.OrderItem) error {
	query := `
		INSERT INTO order_items (id, order_id, product_id, name, price, quantity, total)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	for _, item := range items {
		_, err := tx.ExecContext(ctx, query,
			item.ID, item.OrderID, item.ProductID, item.Name,
			item.Price, item.Quantity, item.Total)
		if err != nil {
			return err
		}
	}

	return nil
}

// insertAddress вставляет адрес
func (r *OrderRepository) insertAddress(ctx context.Context, tx *sql.Tx, address *entities.Address) error {
	query := `
		INSERT INTO order_addresses (id, order_id, type, street, city, state, country, zip_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := tx.ExecContext(ctx, query,
		address.ID, address.OrderID, address.Type, address.Street,
		address.City, address.State, address.Country, address.ZipCode)

	return err
}

// getOrderItems получает элементы заказа
func (r *OrderRepository) getOrderItems(ctx context.Context, orderID uuid.UUID) ([]entities.OrderItem, error) {
	query := `
		SELECT id, order_id, product_id, name, price, quantity, total
		FROM order_items 
		WHERE order_id = $1
		ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []entities.OrderItem
	for rows.Next() {
		var item entities.OrderItem
		err := rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID, &item.Name,
			&item.Price, &item.Quantity, &item.Total)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// getOrderAddresses получает адреса заказа
func (r *OrderRepository) getOrderAddresses(ctx context.Context, orderID uuid.UUID) ([]*entities.Address, error) {
	query := `
		SELECT id, order_id, type, street, city, state, country, zip_code
		FROM order_addresses 
		WHERE order_id = $1`

	rows, err := r.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []*entities.Address
	for rows.Next() {
		var addr entities.Address
		err := rows.Scan(
			&addr.ID, &addr.OrderID, &addr.Type, &addr.Street,
			&addr.City, &addr.State, &addr.Country, &addr.ZipCode)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, &addr)
	}

	return addresses, nil
}

// buildListQuery строит запрос для получения списка заказов
func (r *OrderRepository) buildListQuery(filters repositories.OrderFilters) (string, []interface{}) {
	query := `
		SELECT id, customer_id, email, status, total_amount, currency, created_at, updated_at
		FROM orders`

	var conditions []string
	var args []interface{}
	argIndex := 1

	// Фильтры
	if filters.CustomerID != nil {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argIndex))
		args = append(args, *filters.CustomerID)
		argIndex++
	}

	if filters.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filters.Status)
		argIndex++
	}

	if filters.Email != nil {
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.Email+"%")
		argIndex++
	}

	if filters.MinAmount != nil {
		conditions = append(conditions, fmt.Sprintf("total_amount >= $%d", argIndex))
		args = append(args, *filters.MinAmount)
		argIndex++
	}

	if filters.MaxAmount != nil {
		conditions = append(conditions, fmt.Sprintf("total_amount <= $%d", argIndex))
		args = append(args, *filters.MaxAmount)
		argIndex++
	}

	if filters.Currency != nil {
		conditions = append(conditions, fmt.Sprintf("currency = $%d", argIndex))
		args = append(args, *filters.Currency)
		argIndex++
	}

	if filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filters.DateFrom)
		argIndex++
	}

	if filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filters.DateTo)
		argIndex++
	}

	// WHERE clause
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// ORDER BY
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}

	sortOrder := filters.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// LIMIT and OFFSET
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filters.Limit)
		argIndex++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filters.Offset)
	}

	return query, args
}

// buildCountQuery строит запрос для подсчета заказов
func (r *OrderRepository) buildCountQuery(filters repositories.OrderFilters) (string, []interface{}) {
	query := "SELECT COUNT(*) FROM orders"

	var conditions []string
	var args []interface{}
	argIndex := 1

	// Те же фильтры что и в buildListQuery, но без LIMIT/OFFSET/ORDER BY
	if filters.CustomerID != nil {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argIndex))
		args = append(args, *filters.CustomerID)
		argIndex++
	}

	if filters.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filters.Status)
		argIndex++
	}

	if filters.Email != nil {
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", argIndex))
		args = append(args, "%"+*filters.Email+"%")
		argIndex++
	}

	if filters.MinAmount != nil {
		conditions = append(conditions, fmt.Sprintf("total_amount >= $%d", argIndex))
		args = append(args, *filters.MinAmount)
		argIndex++
	}

	if filters.MaxAmount != nil {
		conditions = append(conditions, fmt.Sprintf("total_amount <= $%d", argIndex))
		args = append(args, *filters.MaxAmount)
		argIndex++
	}

	if filters.Currency != nil {
		conditions = append(conditions, fmt.Sprintf("currency = $%d", argIndex))
		args = append(args, *filters.Currency)
		argIndex++
	}

	if filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filters.DateFrom)
		argIndex++
	}

	if filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filters.DateTo)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	return query, args
}
