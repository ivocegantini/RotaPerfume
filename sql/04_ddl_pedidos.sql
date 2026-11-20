-- ============================================================
-- DDL: Pedidos (ERP)
-- Projeto: SistemaCompleto - rotaperfumes-api
-- Data: 2026-09-13
-- Agente: DataBrain (🌸)
--
-- Origem dos dados: dados/erp/pedidos.csv (28.729 registros, com header).
-- Colunas de origem: pedido_id,cliente_id,vendedor_id,data_pedido,canal,status,valor_total
--
-- Decisões de schema:
--   - ATUALIZAÇÃO (2026-09-15): `pedido_id_origem` deixou de ser uma coluna
--     auxiliar desacoplada e passou a ser a própria PK BIGINT AUTO_INCREMENT
--     da tabela (mesmo padrão adotado em `pagamentos.pagamento_id`), pois o
--     `pedido_id` do CSV é sequencial e único. A antiga PK interna `id` foi
--     removida — não há mais UNIQUE separada (a PK já garante unicidade).
--   - `cliente_id` do CSV NÃO pode ser usado direto como FK: a tabela
--     `clientes` agora tem PK própria `cliente_id_origem`. O importador
--     resolve via lookup `SELECT cliente_id_origem FROM clientes WHERE
--     cliente_id_origem = ?` (lookup direto, já que a coluna é a mesma do
--     CSV). Aqui a coluna `cliente_id` já armazena o `cliente_id_origem`
--     de `clientes`.
--   - `vendedor_id` do CSV, ao contrário, JÁ corresponde 1:1 ao `id` de
--     `vendedores` (seed com IDs explícitos 1..42) — usado direto como FK,
--     sem necessidade de lookup.
--   - `data_pedido` vira DATE (o CSV traz apenas data, sem horário).
--   - `canal` e `status` viram ENUM com os valores reais observados no CSV
--     (análise prévia dos 28.729 registros), evitando strings livres:
--       canal:  App, Telefone, Visita, WhatsApp
--       status: Cancelado, Em separação, Entregue, Faturado
--   - `valor_total` vira DECIMAL(15,2) para evitar erros de arredondamento
--     de ponto flutuante em valores monetários.
--   - ATUALIZAÇÃO (2026-11-18, ECOM-14): `chave_checkout` CHAR(36) NULL
--     UNIQUE, a idempotência do checkout da loja (migração sql/35,
--     make db-fix-pedidos-chave-checkout, para bancos já criados).
--   - ATUALIZAÇÃO (2026-11-20, ECOM-18): status `Aguardando pagamento` (pedido
--     da loja antes da confirmação; migração sql/37, make db-fix-pagamento-loja).
--   - ATUALIZAÇÃO (2026-11-19, ECOM-17): endereço de entrega obrigatório
--     (entrega_cep/endereco/numero/complemento/bairro/cidade/uf, NOT NULL +
--     chk_pedidos_entrega). Bancos já criados: sql/36 (make db-fix-pedidos-entrega).
--   - Índices para os filtros esperados pelo Backend/Frontend (tela de
--     Pedidos master-detail): `status`, `canal`, `data_pedido`, `cliente_id`,
--     `vendedor_id`.
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- Tabela: pedidos
-- Pedidos do ERP (importados de dados/erp/pedidos.csv)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `pedidos` (
    `pedido_id_origem` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao pedido_id do CSV de origem (dados/erp/pedidos.csv), sem desacoplamento',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.cliente_id_origem',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (vendedor_id do CSV já corresponde 1:1)',
    `data_pedido` DATE NOT NULL COMMENT 'Data do pedido',
    `canal` ENUM('App','Telefone','Visita','WhatsApp','Loja') NOT NULL COMMENT 'Canal de venda do pedido (Loja = checkout da loja online, ECOM-06)',
    `status` ENUM('Cancelado','Em separação','Entregue','Faturado','Aguardando pagamento') NOT NULL COMMENT 'Status do pedido (Aguardando pagamento = loja antes da confirmação, ECOM-18)',
    `valor_total` DECIMAL(15,2) NOT NULL DEFAULT 0.00 COMMENT 'Valor total do pedido em R$',
    `chave_checkout` CHAR(36) NULL DEFAULT NULL COMMENT 'Chave de idempotência do checkout da loja (ECOM-14); NULL = pedido do CRM/ERP',
    `entrega_cep` CHAR(8) NOT NULL COMMENT 'CEP de entrega, só dígitos (ECOM-17)',
    `entrega_endereco` VARCHAR(255) NOT NULL COMMENT 'Rua de entrega (ECOM-17)',
    `entrega_numero` VARCHAR(20) NOT NULL COMMENT 'Número de entrega ("s/n" aceito, ECOM-17)',
    `entrega_complemento` VARCHAR(120) NOT NULL DEFAULT '' COMMENT 'Complemento de entrega ('''' = sem, ECOM-17)',
    `entrega_bairro` VARCHAR(120) NOT NULL COMMENT 'Bairro de entrega (ECOM-17)',
    `entrega_cidade` VARCHAR(120) NOT NULL COMMENT 'Cidade de entrega (ECOM-17)',
    `entrega_uf` CHAR(2) NOT NULL COMMENT 'UF de entrega (ECOM-17)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`pedido_id_origem`),
    UNIQUE KEY `uq_pedidos_chave_checkout` (`chave_checkout`),
    KEY `idx_pedidos_status` (`status`),
    KEY `idx_pedidos_canal` (`canal`),
    KEY `idx_pedidos_data_pedido` (`data_pedido`),
    KEY `idx_pedidos_cliente_id` (`cliente_id`),
    KEY `idx_pedidos_vendedor_id` (`vendedor_id`),
    CONSTRAINT `chk_pedidos_entrega` CHECK (`entrega_cep` REGEXP '^[0-9]{8}$' AND TRIM(`entrega_endereco`) <> ''
        AND TRIM(`entrega_numero`) <> '' AND TRIM(`entrega_bairro`) <> '' AND TRIM(`entrega_cidade`) <> ''
        AND CHAR_LENGTH(`entrega_uf`) = 2),
    CONSTRAINT `fk_pedidos_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`cliente_id_origem`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    CONSTRAINT `fk_pedidos_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de pedidos do ERP';

SET FOREIGN_KEY_CHECKS = 1;
