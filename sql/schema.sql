-- Schema completo do RotaPerfumes (MariaDB 11.4+), na versão atual.
-- Banco vazio: make db-up (dev) ou deploy/seed-producao.sh (produção).
-- Banco que já existe: sql/migracao.sql (make db-migrar), idempotente.

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- vendedores: CRM; FK de usuarios
CREATE TABLE IF NOT EXISTS `vendedores` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do vendedor',
    `nome` VARCHAR(120) NOT NULL COMMENT 'Nome completo do vendedor',
    `regiao` VARCHAR(80) NOT NULL COMMENT 'Região de atuação',
    `uf` CHAR(2) NOT NULL COMMENT 'UF do estado',
    `data_admissao` DATE NOT NULL COMMENT 'Data de admissão',
    `data_desligamento` DATE NULL COMMENT 'Data de desligamento (NULL = ativo)',
    `meta_mensal` DECIMAL(15,2) NOT NULL DEFAULT 0.00 COMMENT 'Meta mensal em R$',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de vendedores (CRM)';

-- usuarios: autenticação, roles e vínculo opcional a vendedor
CREATE TABLE IF NOT EXISTS `usuarios` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do usuário',
    `nome` VARCHAR(120) NOT NULL COMMENT 'Nome completo do usuário',
    `email` VARCHAR(120) NOT NULL COMMENT 'E-mail de login (único)',
    `password_hash` VARCHAR(255) NOT NULL COMMENT 'Hash Argon2id (PHC, com pepper) da senha; bcrypt legado migra no login',
    `role` ENUM('admin','normal','usuario') NOT NULL DEFAULT 'normal' COMMENT 'Papel: admin, normal (equipe interna) ou usuario (cliente da loja, ECOM-01)',
    `id_vendedor` BIGINT NULL COMMENT 'FK opcional para vendedor vinculado',
    `ativo` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '0 = inativo, 1 = ativo',
    `deve_trocar_senha` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = usuário deve trocar a senha no próximo login (senha gerada pelo sistema)',
    `email_confirmacao_pendente` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = autocadastro aguardando confirmação do e-mail (ECOM-03)',
    `tokens_validos_desde` DATETIME NULL DEFAULT NULL COMMENT 'Access tokens com iat <= este instante são rejeitados (SEC-08); NULL = sem corte',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    `ultimo_login_at` TIMESTAMP NULL COMMENT 'Data do último login (nullable)',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_usuarios_email` (`email`),
    KEY `idx_usuarios_role` (`role`),
    KEY `idx_usuarios_ativo` (`ativo`),
    KEY `idx_usuarios_id_vendedor` (`id_vendedor`),
    CONSTRAINT `fk_usuarios_vendedor` FOREIGN KEY (`id_vendedor`)
        REFERENCES `vendedores`(`id`)
        ON DELETE SET NULL
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de usuários para autenticação do sistema';

-- refresh_tokens: renovação do JWT; motivo da revogação e marca de reuso
CREATE TABLE IF NOT EXISTS `refresh_tokens` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do refresh token',
    `usuario_id` BIGINT NOT NULL COMMENT 'FK para usuario dono do token',
    `token_hash` VARCHAR(255) NOT NULL COMMENT 'SHA-256 do refresh token (único)',
    `expires_at` DATETIME NOT NULL COMMENT 'Data/hora de expiração do token',
    `revoked_at` DATETIME NULL COMMENT 'Data/hora de revogação (NULL = ativo)',
    `revoked_reason` ENUM('rotacao','logout','revogacao_massa','senha','inativacao') NULL DEFAULT NULL COMMENT 'Motivo da revogação (NULL = ativo ou revogado antes do SEC-07/legado)',
    `reuso_detectado_em` DATETIME NULL DEFAULT NULL COMMENT 'Último reuso detectado deste token já rotacionado (SEC-12); NULL = nunca',
    `ip_origem` VARCHAR(45) NULL COMMENT 'IP de origem que solicitou o token (IPv4/IPv6)',
    `user_agent` TEXT NULL COMMENT 'User-Agent do navegador/cliente no momento da criação',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_refresh_token_hash` (`token_hash`),
    KEY `idx_refresh_usuario_id` (`usuario_id`),
    KEY `idx_refresh_expires_at` (`expires_at`),
    KEY `idx_refresh_revoked_at` (`revoked_at`),
    CONSTRAINT `fk_refresh_token_usuario`
        FOREIGN KEY (`usuario_id`)
        REFERENCES `usuarios`(`id`)
        ON DELETE CASCADE
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
COMMENT='Tabela de refresh tokens para renovação automática de JWT';

-- senha_historico: auditoria de trocas e resets de senha
CREATE TABLE IF NOT EXISTS senha_historico (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    usuario_id BIGINT NOT NULL,
    resetado_por_id BIGINT NULL,
    senha_hash_anterior VARCHAR(255) NOT NULL,
    ip_origem VARCHAR(45),
    user_agent TEXT,
    tipo_reset ENUM('usuario','admin','primeiro_acesso','esquecimento') NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (usuario_id) REFERENCES usuarios(id) ON DELETE CASCADE,
    FOREIGN KEY (resetado_por_id) REFERENCES usuarios(id) ON DELETE SET NULL,
    INDEX idx_usuario (usuario_id),
    INDEX idx_data (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- confirmacoes_email: confirmação do cadastro da loja
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

-- clientes: CRM; PJ (CNPJ único) ou PF da loja
CREATE TABLE IF NOT EXISTS `clientes` (
    `cliente_id_origem` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao cliente_id do CSV de origem (dados/crm/clientes.csv), sem desacoplamento',
    `cnpj` CHAR(14) NULL DEFAULT NULL COMMENT 'CNPJ normalizado (pessoa jurídica): 14 caracteres, sem máscara, maiúsculas (NEG-02); NULL = pessoa física (cpf)',
    `cpf` CHAR(11) NULL DEFAULT NULL COMMENT 'CPF (pessoa física, só dígitos); NULL = pessoa jurídica (ECOM-05)',
    `razao_social` VARCHAR(255) NOT NULL COMMENT 'Razão social do cliente',
    `segmento` VARCHAR(80) NOT NULL COMMENT 'Segmento de atuação (ex: Perfumaria, E-commerce)',
    `cidade` VARCHAR(120) NOT NULL COMMENT 'Cidade do cliente',
    `uf` CHAR(2) NOT NULL COMMENT 'UF do estado',
    `bairro` VARCHAR(120) NULL COMMENT 'Bairro do cliente',
    `endereco` VARCHAR(255) NULL DEFAULT NULL COMMENT 'Logradouro (rua, avenida...; ECOM-05/09)',
    `numero` VARCHAR(20) NULL DEFAULT NULL COMMENT 'Número do endereço ("s/n" aceito, ECOM-09)',
    `complemento` VARCHAR(120) NULL DEFAULT NULL COMMENT 'Complemento do endereço (opcional, ECOM-09)',
    `cep` CHAR(8) NULL DEFAULT NULL COMMENT 'CEP (só dígitos, ECOM-05)',
    `data_cadastro` DATE NOT NULL COMMENT 'Data de cadastro do cliente no CRM de origem',
    `ativo` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '0 = inativo, 1 = ativo (origem: N/S)',
    `usuario_id` BIGINT NULL DEFAULT NULL COMMENT 'Login da loja dono deste cliente (ECOM-05); NULL = cliente do CRM',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`cliente_id_origem`),
    UNIQUE KEY `uq_clientes_cnpj` (`cnpj`),
    UNIQUE KEY `uq_clientes_cpf` (`cpf`),
    UNIQUE KEY `uq_clientes_usuario` (`usuario_id`),
    KEY `idx_clientes_uf` (`uf`),
    KEY `idx_clientes_segmento` (`segmento`),
    KEY `idx_clientes_ativo` (`ativo`),
    KEY `idx_clientes_razao_social` (`razao_social`),
    CONSTRAINT `fk_clientes_usuario` FOREIGN KEY (`usuario_id`) REFERENCES `usuarios`(`id`) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT `chk_clientes_documento` CHECK (`cnpj` IS NOT NULL OR `cpf` IS NOT NULL)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de clientes do CRM';

-- produtos: catálogo; nome, descrição e imagem
CREATE TABLE IF NOT EXISTS `produtos` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do produto',
    `sku` VARCHAR(40) NOT NULL COMMENT 'SKU original do produto no CSV de origem (dados/erp/produtos.csv)',
    `nome` VARCHAR(255) NOT NULL COMMENT 'Nome do produto (vitrine, pedidos e estoque)',
    `descricao` TEXT NULL DEFAULT NULL COMMENT 'Descrição completa do produto (página da loja); NULL = sem descrição',
    `categoria` VARCHAR(80) NOT NULL COMMENT 'Categoria do produto (ex: Eau de Parfum)',
    `marca` VARCHAR(80) NOT NULL COMMENT 'Marca do produto',
    `nota_olfativa` VARCHAR(80) NULL COMMENT 'Nota olfativa predominante (ex: Cardamomo, Jasmim)',
    `preco_tabela` DECIMAL(10,2) NOT NULL COMMENT 'Preço de tabela (venda) do produto',
    `custo_unitario` DECIMAL(10,2) NOT NULL COMMENT 'Custo unitário do produto',
    `unidade` VARCHAR(40) NOT NULL COMMENT 'Unidade/embalagem de venda (ex: UN, KIT 5, DISPLAY 24)',
    `data_lancamento` DATE NULL COMMENT 'Data de lançamento do produto (pode ser nula na origem)',
    `ativo` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '0 = inativo, 1 = ativo (origem: N/S; padrão de importação: ativo)',
    `imagem` VARCHAR(41) NULL DEFAULT NULL COMMENT 'Nome do arquivo (UUIDv4 + .jpg/.png/.webp); NULL = sem imagem',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_produtos_sku` (`sku`),
    UNIQUE KEY `uk_produtos_imagem` (`imagem`),
    KEY `idx_produtos_categoria` (`categoria`),
    KEY `idx_produtos_marca` (`marca`),
    KEY `idx_produtos_ativo` (`ativo`),
    CONSTRAINT `chk_produtos_imagem` CHECK (`imagem` IS NULL OR `imagem` REGEXP '(?-i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\\.(jpg|png|webp)$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de produtos do catálogo/ERP';

-- pedidos: ERP; canal Loja, chave do checkout e endereço de entrega
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

-- itens_pedido: ERP; itens de cada pedido
CREATE TABLE IF NOT EXISTS `itens_pedido` (
    `item_id_origem` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao item_id do CSV de origem (dados/erp/itens_pedido.csv), sem desacoplamento',
    `pedido_id` BIGINT NOT NULL COMMENT 'FK para pedidos.pedido_id_origem',
    `produto_id` BIGINT NOT NULL COMMENT 'FK para produtos.id (resolvido via sku na importação)',
    `quantidade` INT NOT NULL COMMENT 'Quantidade de unidades do item',
    `preco_praticado` DECIMAL(10,2) NOT NULL COMMENT 'Preço unitário praticado no item',
    `desconto_pct` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT 'Percentual de desconto aplicado ao item (ex: 5.00 = 5%)',
    `valor_bruto` DECIMAL(15,2) NOT NULL COMMENT 'Valor bruto do item (quantidade x preco_praticado, já com desconto conforme origem)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`item_id_origem`),
    KEY `idx_itens_pedido_pedido_id` (`pedido_id`),
    KEY `idx_itens_pedido_produto_id` (`produto_id`),
    CONSTRAINT `fk_itens_pedido_pedido` FOREIGN KEY (`pedido_id`)
        REFERENCES `pedidos`(`pedido_id_origem`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_itens_pedido_produto` FOREIGN KEY (`produto_id`)
        REFERENCES `produtos`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de itens de pedido do ERP';

-- pagamentos: ERP; taxa 0 a 30%, status Recusado
CREATE TABLE IF NOT EXISTS `pagamentos` (
    `pagamento_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao pagamento_id do CSV de origem (dados/erp/pagamentos.csv), sem desacoplamento',
    `pedido_id` BIGINT NOT NULL COMMENT 'FK para pedidos.pedido_id_origem',
    `forma_pagamento` ENUM('Boleto 14 dias','Boleto 28 dias','Cartão de crédito','Cartão de débito','Cheque a prazo','Dinheiro','PIX') NOT NULL COMMENT 'Forma de pagamento utilizada',
    `parcelas` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT 'Número de parcelas',
    `valor` DECIMAL(15,2) NOT NULL COMMENT 'Valor bruto do pagamento em R$',
    `taxa_pct` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT 'Percentual de taxa aplicado (ex: 3.20 = 3,20%)',
    `valor_liquido` DECIMAL(15,2) NOT NULL COMMENT 'Valor líquido do pagamento em R$ (valor descontada a taxa)',
    `data_vencimento` DATE NOT NULL COMMENT 'Data de vencimento do pagamento',
    `data_pagamento` DATE NULL COMMENT 'Data em que o pagamento efetivamente ocorreu (NULL quando ainda não pago — status Em aberto/Inadimplente)',
    `status_pagamento` ENUM('Em aberto','Inadimplente','Pago','Pago com atraso','Recusado') NOT NULL COMMENT 'Status atual do pagamento (Recusado = gateway da loja recusou, ECOM-18)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`pagamento_id`),
    KEY `idx_pagamentos_pedido_id` (`pedido_id`),
    KEY `idx_pagamentos_status_pagamento` (`status_pagamento`),
    KEY `idx_pagamentos_data_vencimento` (`data_vencimento`),
    CONSTRAINT `fk_pagamentos_pedido` FOREIGN KEY (`pedido_id`)
        REFERENCES `pedidos`(`pedido_id_origem`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    CONSTRAINT `chk_pagamentos_taxa_pct` CHECK (`taxa_pct` >= 0 AND `taxa_pct` <= 30)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tabela de pagamentos do ERP';

-- carteiras: CRM; vínculo cliente ↔ vendedor
CREATE TABLE IF NOT EXISTS `carteiras` (
    `carteira_id_origem` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao carteira_id do CSV de origem (dados/crm/carteira.csv), sem desacoplamento',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.cliente_id_origem',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (vendedor_id do CSV já corresponde 1:1)',
    `data_inicio` DATE NOT NULL COMMENT 'Data de início do vínculo cliente-vendedor',
    `data_fim` DATE NULL COMMENT 'Data de fim do vínculo (NULL = vínculo ativo)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`carteira_id_origem`),
    UNIQUE KEY `uk_carteiras_cliente_vendedor_inicio` (`cliente_id`, `vendedor_id`, `data_inicio`),
    KEY `idx_carteiras_cliente_id` (`cliente_id`),
    KEY `idx_carteiras_vendedor_id` (`vendedor_id`),
    KEY `idx_carteiras_data_fim` (`data_fim`),
    CONSTRAINT `fk_carteiras_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`cliente_id_origem`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_carteiras_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Vínculo histórico Cliente ↔ Vendedor (carteira de clientes)';

-- oportunidades: CRM; funil de vendas
CREATE TABLE IF NOT EXISTS `oportunidades` (
    `oportunidade_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao oportunidade_id do CSV de origem (dados/crm/oportunidades.csv)',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.cliente_id_origem',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (dono direto da oportunidade)',
    `origem` VARCHAR(80) NOT NULL COMMENT 'Canal de origem da oportunidade (ex.: WhatsApp, Indicação, Inbound site, Instagram)',
    `data_abertura` DATE NOT NULL COMMENT 'Data de abertura da oportunidade',
    `etapa` VARCHAR(40) NOT NULL COMMENT 'Etapa atual do funil (ex.: Prospecção, Qualificação, Proposta enviada, Negociação, Fechado ganho, Fechado perdido)',
    `probabilidade_pct` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT 'Probabilidade de fechamento (0-100)',
    `valor_estimado` DECIMAL(15,2) NOT NULL DEFAULT 0.00 COMMENT 'Valor estimado da oportunidade',
    `data_fechamento` DATE NULL COMMENT 'Data de fechamento (NULL enquanto a oportunidade está aberta)',
    `ciclo_dias` INT NULL COMMENT 'Dias entre abertura e fechamento (NULL enquanto aberta)',
    `motivo_perda` VARCHAR(255) NULL COMMENT 'Motivo da perda, quando etapa = Fechado perdido',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`oportunidade_id`),
    KEY `idx_oportunidades_cliente_id` (`cliente_id`),
    KEY `idx_oportunidades_vendedor_id` (`vendedor_id`),
    KEY `idx_oportunidades_etapa` (`etapa`),
    KEY `idx_oportunidades_origem` (`origem`),
    KEY `idx_oportunidades_data_abertura` (`data_abertura`),
    CONSTRAINT `fk_oportunidades_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`cliente_id_origem`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_oportunidades_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Oportunidades de venda (CRM)';

-- visitas: CRM; visitas de vendedores
CREATE TABLE IF NOT EXISTS `visitas` (
    `visita_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'PK — corresponde 1:1 ao visita_id do CSV de origem (dados/crm/visitas.csv)',
    `cliente_id` BIGINT NOT NULL COMMENT 'FK para clientes.cliente_id_origem',
    `vendedor_id` BIGINT NOT NULL COMMENT 'FK para vendedores.id (responsável pela visita)',
    `data_visita` DATE NOT NULL COMMENT 'Data em que a visita ocorreu',
    `resultado` VARCHAR(40) NOT NULL COMMENT 'Resultado da visita (ex.: Sem pedido, Pedido realizado, Reagendada, Cliente ausente, Apenas relacionamento)',
    `duracao_min` INT NOT NULL DEFAULT 0 COMMENT 'Duração da visita em minutos',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`visita_id`),
    KEY `idx_visitas_cliente_id` (`cliente_id`),
    KEY `idx_visitas_vendedor_id` (`vendedor_id`),
    KEY `idx_visitas_data_visita` (`data_visita`),
    KEY `idx_visitas_resultado` (`resultado`),
    CONSTRAINT `fk_visitas_cliente` FOREIGN KEY (`cliente_id`)
        REFERENCES `clientes`(`cliente_id_origem`)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT `fk_visitas_vendedor` FOREIGN KEY (`vendedor_id`)
        REFERENCES `vendedores`(`id`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Visitas de vendedores a clientes (CRM)';

-- estoque: ERP; snapshot diário de saldo
CREATE TABLE IF NOT EXISTS `estoque` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT 'ID interno do registro de estoque',
    `data_snapshot` DATE NOT NULL COMMENT 'Data do snapshot de estoque',
    `sku` VARCHAR(40) NOT NULL COMMENT 'SKU do produto (FK para produtos.sku)',
    `saldo` INT NOT NULL COMMENT 'Saldo em estoque na data do snapshot',
    `ruptura` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '0 = sem ruptura, 1 = em ruptura (origem: N/S)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Data de criação do registro',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Data de última atualização',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_estoque_data_sku` (`data_snapshot`, `sku`),
    KEY `idx_estoque_sku` (`sku`),
    KEY `idx_estoque_data_snapshot` (`data_snapshot`),
    KEY `idx_estoque_ruptura` (`ruptura`),
    CONSTRAINT `fk_estoque_sku` FOREIGN KEY (`sku`)
        REFERENCES `produtos`(`sku`)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Snapshots diários de saldo/ruptura de estoque por SKU';

SET FOREIGN_KEY_CHECKS = 1;
