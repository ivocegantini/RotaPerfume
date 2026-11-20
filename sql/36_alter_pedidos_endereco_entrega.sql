-- ============================================================
-- ALTER: pedidos.entrega_* (ECOM-17) | Data: 2026-11-19 | DataBrain (🌸)
-- Endereço de entrega obrigatório: CEP, rua, número, complemento (opcional,
-- ''), bairro, cidade e UF. Pedidos antigos recebem um endereço fictício
-- (10 capitais, escolhidas pelo id) antes do NOT NULL + CHECK. Idempotente.
-- Reversão: sql/36_revert_pedidos_endereco_entrega.sql.
-- ============================================================

SET NAMES utf8mb4;

-- 1) Colunas novas, ainda NULL (cada uma só se não existir).
SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_cep');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD COLUMN `entrega_cep` CHAR(8) NULL AFTER `chave_checkout`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_endereco');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD COLUMN `entrega_endereco` VARCHAR(255) NULL AFTER `entrega_cep`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_numero');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD COLUMN `entrega_numero` VARCHAR(20) NULL AFTER `entrega_endereco`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_complemento');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD COLUMN `entrega_complemento` VARCHAR(120) NULL AFTER `entrega_numero`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_bairro');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD COLUMN `entrega_bairro` VARCHAR(120) NULL AFTER `entrega_complemento`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_cidade');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD COLUMN `entrega_cidade` VARCHAR(120) NULL AFTER `entrega_bairro`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'entrega_uf');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD COLUMN `entrega_uf` CHAR(2) NULL AFTER `entrega_cidade`', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2) Endereço fictício nos pedidos sem endereço (capital = MOD(id*31, 10);
--    número 1..1999 derivado do id). Só toca linhas com algum campo vazio.
UPDATE `pedidos` p
JOIN (
    SELECT 0 AS k, '01310100' AS cep, 'Avenida Paulista' AS rua, 'Bela Vista' AS bairro, 'São Paulo' AS cidade, 'SP' AS uf
    UNION ALL SELECT 1, '22070011', 'Avenida Atlântica', 'Copacabana', 'Rio de Janeiro', 'RJ'
    UNION ALL SELECT 2, '30130010', 'Avenida Afonso Pena', 'Centro', 'Belo Horizonte', 'MG'
    UNION ALL SELECT 3, '80020310', 'Rua XV de Novembro', 'Centro', 'Curitiba', 'PR'
    UNION ALL SELECT 4, '90020004', 'Rua dos Andradas', 'Centro Histórico', 'Porto Alegre', 'RS'
    UNION ALL SELECT 5, '40060001', 'Avenida Sete de Setembro', 'Vitória', 'Salvador', 'BA'
    UNION ALL SELECT 6, '50050000', 'Rua da Aurora', 'Boa Vista', 'Recife', 'PE'
    UNION ALL SELECT 7, '60025100', 'Rua Major Facundo', 'Centro', 'Fortaleza', 'CE'
    UNION ALL SELECT 8, '70070120', 'SBS Quadra 2', 'Asa Sul', 'Brasília', 'DF'
    UNION ALL SELECT 9, '74015010', 'Avenida Goiás', 'Setor Central', 'Goiânia', 'GO'
) e ON e.k = MOD(p.`pedido_id_origem` * 31, 10)
SET p.`entrega_cep` = e.cep,
    p.`entrega_endereco` = e.rua,
    p.`entrega_numero` = CAST(1 + MOD(p.`pedido_id_origem` * 7919, 1999) AS CHAR),
    p.`entrega_complemento` = COALESCE(p.`entrega_complemento`, ''),
    p.`entrega_bairro` = e.bairro,
    p.`entrega_cidade` = e.cidade,
    p.`entrega_uf` = e.uf
WHERE p.`entrega_cep` IS NULL OR p.`entrega_endereco` IS NULL OR p.`entrega_numero` IS NULL
   OR p.`entrega_bairro` IS NULL OR p.`entrega_cidade` IS NULL OR p.`entrega_uf` IS NULL;

UPDATE `pedidos` SET `entrega_complemento` = '' WHERE `entrega_complemento` IS NULL;

-- 3) Obrigatório: NOT NULL (complemento vazio = sem complemento) + CHECK.
ALTER TABLE `pedidos`
    MODIFY `entrega_cep` CHAR(8) NOT NULL COMMENT 'CEP de entrega, só dígitos (ECOM-17)',
    MODIFY `entrega_endereco` VARCHAR(255) NOT NULL COMMENT 'Rua de entrega (ECOM-17)',
    MODIFY `entrega_numero` VARCHAR(20) NOT NULL COMMENT 'Número de entrega ("s/n" aceito, ECOM-17)',
    MODIFY `entrega_complemento` VARCHAR(120) NOT NULL DEFAULT '' COMMENT 'Complemento de entrega ('''' = sem, ECOM-17)',
    MODIFY `entrega_bairro` VARCHAR(120) NOT NULL COMMENT 'Bairro de entrega (ECOM-17)',
    MODIFY `entrega_cidade` VARCHAR(120) NOT NULL COMMENT 'Cidade de entrega (ECOM-17)',
    MODIFY `entrega_uf` CHAR(2) NOT NULL COMMENT 'UF de entrega (ECOM-17)';

SET @c = (SELECT COUNT(*) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND CONSTRAINT_NAME = 'chk_pedidos_entrega');
SET @ddl = IF(@c = 0, 'ALTER TABLE `pedidos` ADD CONSTRAINT `chk_pedidos_entrega` CHECK (`entrega_cep` REGEXP ''^[0-9]{8}$'' AND TRIM(`entrega_endereco`) <> '''' AND TRIM(`entrega_numero`) <> '''' AND TRIM(`entrega_bairro`) <> '''' AND TRIM(`entrega_cidade`) <> '''' AND CHAR_LENGTH(`entrega_uf`) = 2)', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
