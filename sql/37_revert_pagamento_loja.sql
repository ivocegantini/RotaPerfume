-- ============================================================
-- REVERT: desfaz sql/37 (ECOM-18) | Data: 2026-11-20 | DataBrain (🌸)
-- Antes de tirar os valores do ENUM: 'Aguardando pagamento' volta a
-- 'Em separação' (comportamento do ECOM-14) e 'Recusado' vira 'Em aberto'
-- (o pedido recusado já está Cancelado). Idempotente.
-- ============================================================

SET NAMES utf8mb4;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'status'
      AND COLUMN_TYPE LIKE '%Aguardando pagamento%');
SET @ddl = IF(@c > 0, 'UPDATE `pedidos` SET `status` = \'Em separação\' WHERE `status` = \'Aguardando pagamento\'', 'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
SET @ddl = IF(@c > 0,
    'ALTER TABLE `pedidos` MODIFY COLUMN `status` ENUM(\'Cancelado\',\'Em separação\',\'Entregue\',\'Faturado\') NOT NULL COMMENT \'Status do pedido\'',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pagamentos' AND COLUMN_NAME = 'status_pagamento'
      AND COLUMN_TYPE LIKE '%Recusado%');
SET @ddl = IF(@c > 0, 'UPDATE `pagamentos` SET `status_pagamento` = \'Em aberto\' WHERE `status_pagamento` = \'Recusado\'', 'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
SET @ddl = IF(@c > 0,
    'ALTER TABLE `pagamentos` MODIFY COLUMN `status_pagamento` ENUM(\'Em aberto\',\'Inadimplente\',\'Pago\',\'Pago com atraso\') NOT NULL COMMENT \'Status atual do pagamento\'',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
