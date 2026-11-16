-- ============================================================
-- REVERT: desfaz sql/30_alter_clientes_loja.sql (ECOM-05)
-- Data: 2026-11-16 | Agente: DataBrain (🌸)
-- Aborta (SIGNAL) se houver cliente sem cnpj (pessoa física da loja): apague
-- ou converta esses clientes antes; o revert nunca apaga dados sozinho.
-- ============================================================

SET NAMES utf8mb4;

SET @pf = (SELECT COUNT(*) FROM `clientes` WHERE `cnpj` IS NULL);
SET @ddl = IF(@pf = 0, 'SELECT 1', 'SIGNAL SQLSTATE \'45000\' SET MESSAGE_TEXT = \'ha clientes pessoa fisica (cnpj NULL); revert abortado\'');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND CONSTRAINT_NAME = 'chk_clientes_documento');
SET @ddl = IF(@c = 1, 'ALTER TABLE `clientes` DROP CONSTRAINT `chk_clientes_documento`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_clientes_usuario');
SET @ddl = IF(@c = 1, 'ALTER TABLE `clientes` DROP FOREIGN KEY `fk_clientes_usuario`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'uq_clientes_usuario');
SET @ddl = IF(@c > 0, 'ALTER TABLE `clientes` DROP INDEX `uq_clientes_usuario`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'uq_clientes_cpf');
SET @ddl = IF(@c > 0, 'ALTER TABLE `clientes` DROP INDEX `uq_clientes_cpf`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND COLUMN_NAME IN ('usuario_id','cep','endereco','cpf'));
SET @ddl = IF(@c > 0, 'ALTER TABLE `clientes` DROP COLUMN IF EXISTS `usuario_id`, DROP COLUMN IF EXISTS `cep`, DROP COLUMN IF EXISTS `endereco`, DROP COLUMN IF EXISTS `cpf`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

ALTER TABLE `clientes` MODIFY `cnpj` CHAR(14) NOT NULL
    COMMENT 'CNPJ normalizado: 14 caracteres, sem máscara, em maiúsculas; 12 primeiras posições em [0-9A-Z] e 2 DVs numéricos (NEG-02)';
