-- migrations/001_initial_schema.down.sql

-- Drop triggers
DROP TRIGGER IF EXISTS update_order_total_on_item_change_trigger ON order_items;
DROP TRIGGER IF EXISTS check_order_total_trigger ON orders;
DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;

-- Drop functions
DROP FUNCTION IF EXISTS update_order_total_on_item_change() CASCADE;
DROP FUNCTION IF EXISTS check_order_total() CASCADE;
DROP FUNCTION IF EXISTS calculate_order_total(UUID) CASCADE;
DROP FUNCTION IF EXISTS is_valid_email(TEXT) CASCADE;
DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;
DROP FUNCTION IF EXISTS get_order_statistics() CASCADE;
DROP FUNCTION IF EXISTS cleanup_old_orders(INT) CASCADE;

-- Drop view
DROP VIEW IF EXISTS orders_with_stats;

-- Drop tables
DROP TABLE IF EXISTS order_addresses;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
