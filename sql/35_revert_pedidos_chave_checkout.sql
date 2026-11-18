-- ============================================================
-- REVERT: desfaz sql/35 (ECOM-14) | Data: 2026-11-18 | DataBrain (🌸)
-- Remove o índice e a coluna chave_checkout (os pedidos da loja continuam;
-- só se perde a proteção contra compra repetida). Idempotente.
-- ============================================================

SET NAMES utf8mb4;

SET @c = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND INDEX_NAME = 'uq_pedidos_chave_checkout');
SET @ddl = IF(@c > 0, 'ALTER TABLE `pedidos` DROP INDEX `uq_pedidos_chave_checkout`', 'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'chave_checkout');
SET @ddl = IF(@c > 0, 'ALTER TABLE `pedidos` DROP COLUMN `chave_checkout`', 'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
