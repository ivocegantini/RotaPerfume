-- ============================================================
-- REVERT: desfaz sql/34_alter_produtos_nome_descricao.sql (ECOM-11)
-- Data: 2026-11-18 | Agente: DataBrain (🌸)
-- Apaga a descrição completa e devolve `nome` para `descricao`. ATENÇÃO: as
-- descrições se perdem; volte antes o backend para a versão sem `nome`. Idempotente.
-- ============================================================

SET NAMES utf8mb4;

-- 1) Remove a descricao completa (só se `nome` existe, ou seja, a 34 rodou).
SET @tem_nome = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'produtos' AND COLUMN_NAME = 'nome');
SET @tem_descricao = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'produtos' AND COLUMN_NAME = 'descricao');
SET @ddl = IF(@tem_nome = 1 AND @tem_descricao = 1, 'ALTER TABLE `produtos` DROP COLUMN `descricao`', 'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 2) nome -> descricao.
SET @ddl = IF(@tem_nome = 1,
    'ALTER TABLE `produtos` CHANGE `nome` `descricao` VARCHAR(255) NOT NULL COMMENT \'Descrição/nome do produto\'',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
