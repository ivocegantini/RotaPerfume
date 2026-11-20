-- ============================================================
-- ALTER: confirmação do pagamento da loja (ECOM-18) | Data: 2026-11-20 | DataBrain (🌸)
-- pedidos.status ganha 'Aguardando pagamento' (pedido da loja antes da
-- confirmação) e pagamentos.status_pagamento ganha 'Recusado'. Só acrescenta
-- valores ao ENUM (nenhuma linha muda). Idempotente.
-- Reversão: sql/37_revert_pagamento_loja.sql.
-- ============================================================

SET NAMES utf8mb4;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pedidos' AND COLUMN_NAME = 'status'
      AND COLUMN_TYPE LIKE '%Aguardando pagamento%');
SET @ddl = IF(@c = 0,
    'ALTER TABLE `pedidos` MODIFY COLUMN `status` ENUM(\'Cancelado\',\'Em separação\',\'Entregue\',\'Faturado\',\'Aguardando pagamento\') NOT NULL COMMENT \'Status do pedido (Aguardando pagamento = loja antes da confirmação, ECOM-18)\'',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @c = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pagamentos' AND COLUMN_NAME = 'status_pagamento'
      AND COLUMN_TYPE LIKE '%Recusado%');
SET @ddl = IF(@c = 0,
    'ALTER TABLE `pagamentos` MODIFY COLUMN `status_pagamento` ENUM(\'Em aberto\',\'Inadimplente\',\'Pago\',\'Pago com atraso\',\'Recusado\') NOT NULL COMMENT \'Status atual do pagamento (Recusado = gateway da loja recusou, ECOM-18)\'',
    'SELECT 1');
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
