-- ============================================================
-- ALTER: usuarios.email_confirmacao_pendente (ECOM-03)
-- Data: 2026-11-16 | Agente: DataBrain (🌸)
-- 1 = conta do autocadastro aguardando o link do e-mail (login bloqueado).
-- Padrão 0: usuários existentes e os criados pelo admin não mudam.
-- Idempotente. Rode junto com o 28 (make db-fix-confirmacao-email).
-- Reversão: sql/29_revert_usuarios_email_confirmacao.sql.
-- ============================================================

SET NAMES utf8mb4;

SET @coluna_existe = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'usuarios' AND COLUMN_NAME = 'email_confirmacao_pendente'
);
SET @ddl = IF(
    @coluna_existe = 0,
    'ALTER TABLE `usuarios` ADD COLUMN `email_confirmacao_pendente` TINYINT(1) NOT NULL DEFAULT 0 COMMENT \'1 = autocadastro aguardando confirmação do e-mail (ECOM-03)\' AFTER `deve_trocar_senha`',
    'SELECT 1'
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
