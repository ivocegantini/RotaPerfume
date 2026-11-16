-- ============================================================
-- DDL: confirmacoes_email (ECOM-03)
-- Data: 2026-11-16 | Agente: DataBrain (🌸)
-- Token de confirmação do e-mail do autocadastro da loja. Guarda SÓ o
-- SHA-256 do token (o token vai no link do e-mail); uso único, 24h.
-- ============================================================

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `confirmacoes_email` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno',
    `usuario_id` BIGINT NOT NULL COMMENT 'Usuário que confirma o e-mail',
    `token_hash` CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'SHA-256 hex do token do link',
    `expira_em` DATETIME NOT NULL COMMENT 'Fim da validade do link',
    `usado_em` DATETIME NULL DEFAULT NULL COMMENT 'Uso (ou invalidação por reenvio); NULL = ainda válido',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Criação',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_confirmacoes_email_token` (`token_hash`),
    KEY `idx_confirmacoes_email_usuario` (`usuario_id`),
    CONSTRAINT `fk_confirmacoes_email_usuario` FOREIGN KEY (`usuario_id`)
        REFERENCES `usuarios`(`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Links de confirmação de e-mail (ECOM-03)';
