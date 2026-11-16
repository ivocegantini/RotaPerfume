-- ============================================================
-- ALTER: clientes recebe o cliente da loja (pessoa física) — ECOM-05
-- Data: 2026-11-16 | Agente: DataBrain (🌸)
-- O autocadastro cria um cliente ligado ao login (usuario_id) para as
-- compras entrarem nos pedidos/relatórios. Pessoa física: cnpj NULL e cpf.
-- Mudanças: cnpj vira NULL; novas cpf (UNIQUE), endereco, cep e usuario_id
-- (UNIQUE, FK usuarios ON DELETE SET NULL); CHECK: cnpj ou cpf presente.
-- Não altera linhas (os clientes do CRM seguem com cnpj). Idempotente.
-- Reversão: sql/30_revert_clientes_loja.sql (make db-revert-clientes-loja).
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `clientes` MODIFY `cnpj` CHAR(14) NULL DEFAULT NULL
    COMMENT 'CNPJ normalizado (pessoa jurídica): 14 caracteres, sem máscara, maiúsculas (NEG-02); NULL = pessoa física (cpf)';

-- Colunas novas (cada uma só se ainda não existir).
SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND COLUMN_NAME = 'cpf');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD COLUMN `cpf` CHAR(11) NULL DEFAULT NULL COMMENT \'CPF (pessoa física, só dígitos); NULL = pessoa jurídica\' AFTER `cnpj`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND COLUMN_NAME = 'endereco');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD COLUMN `endereco` VARCHAR(255) NULL DEFAULT NULL COMMENT \'Logradouro, número e complemento\' AFTER `bairro`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND COLUMN_NAME = 'cep');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD COLUMN `cep` CHAR(8) NULL DEFAULT NULL COMMENT \'CEP (só dígitos)\' AFTER `endereco`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND COLUMN_NAME = 'usuario_id');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD COLUMN `usuario_id` BIGINT NULL DEFAULT NULL COMMENT \'Login da loja dono deste cliente (ECOM-05); NULL = cliente do CRM\' AFTER `ativo`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Índices únicos e FK.
SET @c = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'uq_clientes_cpf');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD UNIQUE KEY `uq_clientes_cpf` (`cpf`)', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'uq_clientes_usuario');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD UNIQUE KEY `uq_clientes_usuario` (`usuario_id`)', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_clientes_usuario');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD CONSTRAINT `fk_clientes_usuario` FOREIGN KEY (`usuario_id`) REFERENCES `usuarios`(`id`) ON DELETE SET NULL ON UPDATE CASCADE', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND CONSTRAINT_NAME = 'chk_clientes_documento');
SET @ddl = IF(@c = 0, 'ALTER TABLE `clientes` ADD CONSTRAINT `chk_clientes_documento` CHECK (`cnpj` IS NOT NULL OR `cpf` IS NOT NULL)', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
