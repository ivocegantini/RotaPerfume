-- ============================================================
-- REVERT: desfaz sql/33 (ECOM-06) | Data: 2026-11-17 | DataBrain (🌸)
-- Aborta (erro 1265/1366 do ENUM) se já houver pedido com canal 'Loja'.
-- Remove o vendedor "Loja Online" só se ele não tiver pedidos nem carteiras.
-- ============================================================

SET NAMES utf8mb4;
SET SESSION sql_mode = 'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION'; -- só nesta sessão: o MODIFY falha em vez de truncar

ALTER TABLE `pedidos` MODIFY `canal` ENUM('App','Telefone','Visita','WhatsApp') NOT NULL COMMENT 'Canal de venda do pedido';

DELETE v FROM `vendedores` v
WHERE v.`nome` = 'Loja Online'
  AND NOT EXISTS (SELECT 1 FROM `pedidos` p WHERE p.`vendedor_id` = v.`id`)
  AND NOT EXISTS (SELECT 1 FROM `carteiras` c WHERE c.`vendedor_id` = v.`id`)
  AND NOT EXISTS (SELECT 1 FROM `usuarios` u WHERE u.`id_vendedor` = v.`id`);
