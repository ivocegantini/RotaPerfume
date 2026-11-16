-- ============================================================
-- REVERT: desfaz 28 + 29 (ECOM-03) | Data: 2026-11-16 | DataBrain (🌸)
-- Apaga a tabela confirmacoes_email e a coluna usuarios.email_confirmacao_pendente.
-- ATENÇÃO: contas pendentes passam a entrar sem confirmar. Volte antes o
-- backend para uma versão sem o ECOM-03.
-- ============================================================

SET NAMES utf8mb4;

DROP TABLE IF EXISTS `confirmacoes_email`;

SET @coluna_existe = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'usuarios' AND COLUMN_NAME = 'email_confirmacao_pendente'
);
SET @ddl = IF(@coluna_existe = 1, 'ALTER TABLE `usuarios` DROP COLUMN `email_confirmacao_pendente`', 'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
