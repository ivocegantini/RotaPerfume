-- ============================================================
-- REVERT: desfaz sql/27_alter_usuarios_role_usuario.sql (ECOM-01)
-- Data: 2026-11-16 | Agente: DataBrain (🌸)
-- Aborta (SIGNAL) se ainda houver usuário com role 'usuario': inative ou
-- apague essas contas antes; o revert nunca converte papel sozinho.
-- ============================================================

SET NAMES utf8mb4;

SET @qtd = (SELECT COUNT(*) FROM `usuarios` WHERE `role` = 'usuario');
SET @ddl = IF(
    @qtd = 0,
    'ALTER TABLE `usuarios` MODIFY `role` ENUM(\'admin\',\'normal\') NOT NULL DEFAULT \'normal\' COMMENT \'Papel do usuário no sistema\'',
    'SIGNAL SQLSTATE \'45000\' SET MESSAGE_TEXT = \'ha usuarios com role usuario; revert abortado\''
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
