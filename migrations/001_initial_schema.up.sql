-- migrations/001_initial_schema.up.sql

-- Таблица заказов
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    customer_id UUID NOT NULL,
    email VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'confirmed', 'processing', 'shipped', 'delivered', 'cancelled', 'refunded')),
    total_amount DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица элементов заказов
CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    total DECIMAL(10,2) NOT NULL
);

-- Таблица адресов заказов
CREATE TABLE IF NOT EXISTS order_addresses (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('shipping', 'billing')),
    street VARCHAR(255) NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    country VARCHAR(100) NOT NULL,
    zip_code VARCHAR(20) NOT NULL
);

-- Индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_email ON orders(email);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_orders_updated_at ON orders(updated_at);
CREATE INDEX IF NOT EXISTS idx_orders_total_amount ON orders(total_amount);
CREATE INDEX IF NOT EXISTS idx_orders_customer_status ON orders(customer_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_status_created ON orders(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON order_items(product_id);
CREATE INDEX IF NOT EXISTS idx_order_addresses_order_id ON order_addresses(order_id);
CREATE INDEX IF NOT EXISTS idx_order_addresses_type ON order_addresses(order_id, type);

-- Триггер обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER update_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Валидатор email
CREATE OR REPLACE FUNCTION is_valid_email(email TEXT)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$';
END;
$$ LANGUAGE plpgsql;
ALTER TABLE orders
    ADD CONSTRAINT check_valid_email CHECK (is_valid_email(email));

-- Расчёт total_amount и проверки
CREATE OR REPLACE FUNCTION calculate_order_total(order_id_param UUID)
RETURNS DECIMAL(10,2) AS $$
DECLARE
    total_sum DECIMAL(10,2);
BEGIN
    SELECT COALESCE(SUM(total), 0.00) INTO total_sum
    FROM order_items
    WHERE order_id = order_id_param;
    RETURN total_sum;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION check_order_total()
RETURNS TRIGGER AS $$
DECLARE
    calculated_total DECIMAL(10,2);
BEGIN
    calculated_total := calculate_order_total(NEW.id);
    IF ABS(NEW.total_amount - calculated_total) > 0.01 THEN
        RAISE EXCEPTION 'Order total_amount (%) does not match sum (%)', NEW.total_amount, calculated_total;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE CONSTRAINT TRIGGER check_order_total_trigger
    AFTER INSERT OR UPDATE ON orders
                                  DEFERRABLE INITIALLY DEFERRED
                                  FOR EACH ROW EXECUTE FUNCTION check_order_total();

CREATE OR REPLACE FUNCTION update_order_total_on_item_change()
RETURNS TRIGGER AS $$
DECLARE
    affected_order_id UUID;
    new_total DECIMAL(10,2);
BEGIN
    IF TG_OP = 'DELETE' THEN
        affected_order_id := OLD.order_id;
    ELSE
        affected_order_id := NEW.order_id;
    END IF;
    new_total := calculate_order_total(affected_order_id);
    UPDATE orders SET total_amount = new_total, updated_at = NOW() WHERE id = affected_order_id;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER update_order_total_on_item_change_trigger
    AFTER INSERT OR UPDATE OR DELETE ON order_items
    FOR EACH ROW EXECUTE FUNCTION update_order_total_on_item_change();

-- Представление orders_with_stats
CREATE OR REPLACE VIEW orders_with_stats AS
SELECT
    o.id,
    o.customer_id,
    o.email,
    o.status,
    o.total_amount,
    o.currency,
    o.created_at,
    o.updated_at,
    COUNT(oi.id) AS items_count,
    COALESCE(SUM(oi.quantity),0) AS total_quantity,
    (COUNT(oa.id) > 0) AS has_shipping_address
FROM orders o
LEFT JOIN order_items oi ON o.id = oi.order_id
LEFT JOIN order_addresses oa ON o.id = oa.order_id AND oa.type='shipping'
GROUP BY o.id;

-- Статистика по статусам
CREATE OR REPLACE FUNCTION get_order_statistics()
RETURNS TABLE(status VARCHAR, count BIGINT, total_amount DECIMAL(10,2), avg_amount DECIMAL(10,2)) AS $$
BEGIN
    RETURN QUERY
    SELECT status, COUNT(*)::BIGINT, COALESCE(SUM(total_amount),0), COALESCE(AVG(total_amount),0)
    FROM orders GROUP BY status ORDER BY count DESC;
END;
$$ LANGUAGE plpgsql;

-- Функция очистки старых заказов
CREATE OR REPLACE FUNCTION cleanup_old_orders(older_than_days INT DEFAULT 365)
RETURNS INT AS $$
DECLARE deleted_count INT;
BEGIN
    DELETE FROM orders
    WHERE created_at < NOW() - (older_than_days || ' days')::INTERVAL
      AND status IN ('delivered','cancelled','refunded');
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Комментарии
COMMENT ON TABLE orders IS 'Основная таблица заказов';
COMMENT ON TABLE order_items IS 'Элементы заказов';
COMMENT ON TABLE order_addresses IS 'Адреса заказов';
COMMENT ON VIEW orders_with_stats IS 'Заказы с аналитикой';
