-- ============================================================
-- ALTER: usuarios.role ganha 'usuario' (cliente do e-commerce)
-- Card: ECOM-01 | Data: 2026-11-16 | Agente: DataBrain (🌸)
-- O 'usuario' se cadastra sozinho (POST /api/auth/cadastro) e só vê a loja.
-- Não altera dados; idempotente (MODIFY com o mesmo ENUM é no-op).
-- Reversão: sql/27_revert_usuarios_role_usuario.sql (make db-revert-role-usuario).
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `usuarios`
    MODIFY `role` ENUM('admin','normal','usuario') NOT NULL DEFAULT 'normal'
    COMMENT 'Papel: admin, normal (equipe interna) ou usuario (cliente da loja)';
