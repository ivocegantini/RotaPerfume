-- ============================================================
-- ALTER: produtos.descricao vira `nome` e nasce a `descricao` completa (ECOM-11)
-- Data: 2026-11-18 | Agente: DataBrain (🌸)
-- Renomeia a coluna (mesmos dados) e adiciona `descricao` TEXT NULL (texto da
-- página do produto; o `make db-import-produtos` preenche as vazias). Idempotente.
-- Reversão: sql/34_revert_produtos_nome_descricao.sql (perde as descrições).
-- ============================================================

SET NAMES utf8mb4;

-- 1) descricao -> nome (só se `nome` ainda não existe).
SET @tem_nome = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'produtos' AND COLUMN_NAME = 'nome');
SET @ddl = IF(@tem_nome = 0,
    'ALTER TABLE `produtos` CHANGE `descricao` `nome` VARCHAR(255) NOT NULL COMMENT \'Nome do produto (vitrine, pedidos e estoque)\'',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 2) nova descricao (só se ainda não existe).
SET @tem_descricao = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'produtos' AND COLUMN_NAME = 'descricao');
SET @ddl = IF(@tem_descricao = 0,
    'ALTER TABLE `produtos` ADD COLUMN `descricao` TEXT NULL DEFAULT NULL COMMENT \'Descrição completa do produto (página da loja); NULL = sem descrição\' AFTER `nome`',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
