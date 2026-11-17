-- ============================================================
-- ALTER: pedidos.canal ganha 'Loja' + vendedor "Loja Online" (ECOM-06)
-- Data: 2026-11-17 | Agente: DataBrain (🌸)
-- O checkout da loja grava pedidos com canal 'Loja' e vendedor "Loja Online"
-- (pedidos exigem os dois). Não altera linhas existentes. Idempotente.
-- Reversão: sql/33_revert_pedidos_canal_loja.sql.
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `pedidos` MODIFY `canal` ENUM('App','Telefone','Visita','WhatsApp','Loja') NOT NULL COMMENT 'Canal de venda do pedido (Loja = checkout da loja online, ECOM-06)';

-- Sem id explícito (o AUTO_INCREMENT não colide com os vendedores do seed);
-- sem usuário de login. Também está no fim do sql/03_seed_vendedores.sql.
INSERT INTO `vendedores` (`nome`, `regiao`, `uf`, `data_admissao`, `data_desligamento`, `meta_mensal`)
SELECT 'Loja Online', 'Online', 'SP', CURDATE(), NULL, 0.00
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `vendedores` WHERE `nome` = 'Loja Online');
