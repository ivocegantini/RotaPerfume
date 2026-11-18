-- ============================================================
-- ALTER: pedidos.chave_checkout (ECOM-14) | Data: 2026-11-18 | DataBrain (🌸)
-- Chave de idempotência do checkout da loja (UUID gerado no navegador):
-- reenviar a mesma compra devolve o pedido já criado. NULL nos pedidos do
-- CRM/ERP (o UNIQUE aceita vários NULL). Idempotente.
-- Reversão: sql/35_revert_pedidos_chave_checkout.sql.
-- ============================================================

SET NAMES utf8mb4;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'chave_checkout');
SET @ddl = IF(@c = 0,
    'ALTER TABLE `pedidos` ADD COLUMN `chave_checkout` CHAR(36) NULL DEFAULT NULL COMMENT \'Chave de idempotência do checkout da loja (ECOM-14); NULL = pedido do CRM/ERP\' AFTER `valor_total`',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND INDEX_NAME = 'uq_pedidos_chave_checkout');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD UNIQUE KEY `uq_pedidos_chave_checkout` (`chave_checkout`)', 'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
