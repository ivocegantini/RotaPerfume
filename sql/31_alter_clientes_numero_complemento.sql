-- ============================================================
-- ALTER: clientes.numero e clientes.complemento (ECOM-09)
-- Data: 2026-11-16 | Agente: DataBrain (🌸)
-- O cadastro da loja passa a separar o endereço: endereco = logradouro,
-- numero ("s/n" aceito), complemento (opcional); bairro já existia. O CEP
-- fica opcional (endereços sem CEP). Não altera linhas. Idempotente.
-- Reversão: sql/31_revert_clientes_numero_complemento.sql.
-- ============================================================

SET NAMES utf8mb4;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND COLUMN_NAME = 'numero');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD COLUMN `numero` VARCHAR(20) NULL DEFAULT NULL COMMENT \'Número do endereço ("s/n" aceito, ECOM-09)\' AFTER `endereco`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND COLUMN_NAME = 'complemento');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD COLUMN `complemento` VARCHAR(120) NULL DEFAULT NULL COMMENT \'Complemento do endereço (opcional, ECOM-09)\' AFTER `numero`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

ALTER TABLE `clientes` MODIFY `endereco` VARCHAR(255) NULL DEFAULT NULL COMMENT 'Logradouro (rua, avenida...; ECOM-09)';
