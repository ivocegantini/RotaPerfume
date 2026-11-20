-- ============================================================
-- REVERT: desfaz sql/36 (ECOM-17) | Data: 2026-11-19 | DataBrain (🌸)
-- Remove o CHECK e as colunas entrega_* (perde os endereços de entrega,
-- inclusive os fictícios dos pedidos antigos). Idempotente.
-- ============================================================

SET NAMES utf8mb4;

SET @c = (SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND CONSTRAINT_NAME = 'chk_pedidos_entrega');
SET @ddl = IF(@c > 0, 'ALTER TABLE `pedidos` DROP CONSTRAINT `chk_pedidos_entrega`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_cep');
SET @ddl = IF(@c > 0, 'ALTER TABLE `pedidos` DROP COLUMN `entrega_cep`, DROP COLUMN `entrega_endereco`, DROP COLUMN `entrega_numero`, DROP COLUMN `entrega_complemento`, DROP COLUMN `entrega_bairro`, DROP COLUMN `entrega_cidade`, DROP COLUMN `entrega_uf`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
