-- Leva um banco que já existe à versão de sql/schema.sql (make db-migrar; Jenkins, estágio Migracoes).
-- Idempotente: rodar de novo não muda nada. Não há revert: para voltar, restaure o backup (deploy/backup-banco.sh).
-- Base exigida: banco de 2026-10-31 em diante (imagem do produto); um mais antigo é recusado sem alterar nada.
-- senha_historico com created_at NULL também é recusado (o MODIFY preencheria com a hora da migração). Remédio
-- (decisão do responsável pelo banco; a data real não é recuperável), na mesma sessão:
--   SET time_zone = '-03:00';
--   UPDATE senha_historico SET created_at = '1970-01-01 00:00:01' WHERE created_at IS NULL;

SET NAMES utf8mb4;

-- 0. Guarda da base (banco anterior à imagem do produto; as migrações antigas estão no git, até o a179cf6)
--    e senha_historico.created_at NOT NULL.
DELIMITER $$
BEGIN NOT ATOMIC
    DECLARE v_faltas INT DEFAULT 0;
    SELECT (SELECT COUNT(*) = 0 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'usuarios' AND COLUMN_NAME = 'tokens_validos_desde')
         + (SELECT COUNT(*) = 0 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND COLUMN_NAME = 'reuso_detectado_em')
         + (SELECT COUNT(*) = 0 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'produtos' AND COLUMN_NAME = 'imagem')
         + (SELECT COUNT(*) = 0 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'clientes' AND INDEX_NAME = 'uq_clientes_cnpj')
         + (SELECT COUNT(*) = 0 FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'pagamentos' AND CONSTRAINT_NAME = 'chk_pagamentos_taxa_pct')
         + (SELECT COUNT(*) > 0 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'estoque' AND COLUMN_NAME = 'origem')
      INTO v_faltas;
    IF v_faltas > 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migracao.sql: banco anterior a 2026-10-31 (falta a base da imagem do produto). Nada foi alterado; restaure um backup recente ou recrie com sql/schema.sql.';
    END IF;
    IF EXISTS (SELECT 1 FROM `senha_historico` WHERE `created_at` IS NULL) THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migracao.sql: senha_historico com created_at NULL. Nada foi alterado; veja o remedio no cabecalho do sql/migracao.sql e rode de novo.';
    END IF;
    -- senha_historico.created_at obrigatório: dentro do bloco, depois do SIGNAL (nem com --force roda sobre NULL).
    ALTER TABLE `senha_historico` MODIFY COLUMN created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
END$$
DELIMITER ;

-- 1. usuarios: role 'usuario' (cliente da loja) e confirmação de e-mail do autocadastro.
ALTER TABLE `usuarios`
    MODIFY `role` ENUM('admin','normal','usuario') NOT NULL DEFAULT 'normal' COMMENT 'Papel: admin, normal (equipe interna) ou usuario (cliente da loja, ECOM-01)',
    ADD COLUMN IF NOT EXISTS `email_confirmacao_pendente` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 = autocadastro aguardando confirmação do e-mail (ECOM-03)' AFTER `deve_trocar_senha`;

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

-- 2. clientes: pessoa física da loja (cpf, endereço, cep, login dono) e número/complemento.
ALTER TABLE `clientes`
    MODIFY `cnpj` CHAR(14) NULL DEFAULT NULL COMMENT 'CNPJ normalizado (pessoa jurídica): 14 caracteres, sem máscara, maiúsculas (NEG-02); NULL = pessoa física (cpf)',
    ADD COLUMN IF NOT EXISTS `cpf` CHAR(11) NULL DEFAULT NULL COMMENT 'CPF (pessoa física, só dígitos); NULL = pessoa jurídica (ECOM-05)' AFTER `cnpj`,
    ADD COLUMN IF NOT EXISTS `endereco` VARCHAR(255) NULL DEFAULT NULL COMMENT 'Logradouro (rua, avenida...; ECOM-05/09)' AFTER `bairro`,
    ADD COLUMN IF NOT EXISTS `numero` VARCHAR(20) NULL DEFAULT NULL COMMENT 'Número do endereço ("s/n" aceito, ECOM-09)' AFTER `endereco`,
    ADD COLUMN IF NOT EXISTS `complemento` VARCHAR(120) NULL DEFAULT NULL COMMENT 'Complemento do endereço (opcional, ECOM-09)' AFTER `numero`,
    ADD COLUMN IF NOT EXISTS `cep` CHAR(8) NULL DEFAULT NULL COMMENT 'CEP (só dígitos, ECOM-05)' AFTER `complemento`,
    ADD COLUMN IF NOT EXISTS `usuario_id` BIGINT NULL DEFAULT NULL COMMENT 'Login da loja dono deste cliente (ECOM-05); NULL = cliente do CRM' AFTER `ativo`;
-- Banco que já tinha as colunas: só acerta os comentários.
ALTER TABLE `clientes`
    MODIFY `cpf` CHAR(11) NULL DEFAULT NULL COMMENT 'CPF (pessoa física, só dígitos); NULL = pessoa jurídica (ECOM-05)',
    MODIFY `endereco` VARCHAR(255) NULL DEFAULT NULL COMMENT 'Logradouro (rua, avenida...; ECOM-05/09)',
    MODIFY `cep` CHAR(8) NULL DEFAULT NULL COMMENT 'CEP (só dígitos, ECOM-05)';
ALTER TABLE `clientes`
    ADD UNIQUE KEY IF NOT EXISTS `uq_clientes_cpf` (`cpf`),
    ADD UNIQUE KEY IF NOT EXISTS `uq_clientes_usuario` (`usuario_id`);
SET @ddl = IF((SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_clientes_usuario') = 0,
    'ALTER TABLE `clientes` ADD CONSTRAINT `fk_clientes_usuario` FOREIGN KEY (`usuario_id`) REFERENCES `usuarios`(`id`) ON DELETE SET NULL ON UPDATE CASCADE',
    'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
ALTER TABLE `clientes`
    ADD CONSTRAINT IF NOT EXISTS `chk_clientes_documento` CHECK (`cnpj` IS NOT NULL OR `cpf` IS NOT NULL);

-- 3. produtos: a antiga descricao vira nome; descricao nova (TEXT) para a página da loja.
SET @ddl = IF((SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'produtos' AND COLUMN_NAME = 'nome') = 0,
    'ALTER TABLE `produtos` CHANGE `descricao` `nome` VARCHAR(255) NOT NULL COMMENT ''Nome do produto (vitrine, pedidos e estoque)''',
    'DO 0');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
ALTER TABLE `produtos`
    ADD COLUMN IF NOT EXISTS `descricao` TEXT NULL DEFAULT NULL COMMENT 'Descrição completa do produto (página da loja); NULL = sem descrição' AFTER `nome`;

-- 4. pedidos: canal Loja (+ vendedor "Loja Online"), status Aguardando pagamento e chave do checkout.
ALTER TABLE `pedidos`
    MODIFY `canal` ENUM('App','Telefone','Visita','WhatsApp','Loja') NOT NULL COMMENT 'Canal de venda do pedido (Loja = checkout da loja online, ECOM-06)',
    MODIFY `status` ENUM('Cancelado','Em separação','Entregue','Faturado','Aguardando pagamento') NOT NULL COMMENT 'Status do pedido (Aguardando pagamento = loja antes da confirmação, ECOM-18)',
    ADD COLUMN IF NOT EXISTS `chave_checkout` CHAR(36) NULL DEFAULT NULL COMMENT 'Chave de idempotência do checkout da loja (ECOM-14); NULL = pedido do CRM/ERP' AFTER `valor_total`;
ALTER TABLE `pedidos` ADD UNIQUE KEY IF NOT EXISTS `uq_pedidos_chave_checkout` (`chave_checkout`);
INSERT INTO `vendedores` (`nome`, `regiao`, `uf`, `data_admissao`, `data_desligamento`, `meta_mensal`)
SELECT 'Loja Online', 'Online', 'SP', CURDATE(), NULL, 0.00
FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM `vendedores` WHERE `nome` = 'Loja Online');

-- 5. pedidos: endereço de entrega obrigatório; pedido antigo recebe um endereço fictício (10 capitais).
ALTER TABLE `pedidos`
    ADD COLUMN IF NOT EXISTS `entrega_cep` CHAR(8) NULL AFTER `chave_checkout`,
    ADD COLUMN IF NOT EXISTS `entrega_endereco` VARCHAR(255) NULL AFTER `entrega_cep`,
    ADD COLUMN IF NOT EXISTS `entrega_numero` VARCHAR(20) NULL AFTER `entrega_endereco`,
    ADD COLUMN IF NOT EXISTS `entrega_complemento` VARCHAR(120) NULL AFTER `entrega_numero`,
    ADD COLUMN IF NOT EXISTS `entrega_bairro` VARCHAR(120) NULL AFTER `entrega_complemento`,
    ADD COLUMN IF NOT EXISTS `entrega_cidade` VARCHAR(120) NULL AFTER `entrega_bairro`,
    ADD COLUMN IF NOT EXISTS `entrega_uf` CHAR(2) NULL AFTER `entrega_cidade`;
UPDATE `pedidos` p
JOIN (
    SELECT 0 AS k, '01310100' AS cep, 'Avenida Paulista' AS rua, 'Bela Vista' AS bairro, 'São Paulo' AS cidade, 'SP' AS uf
    UNION ALL SELECT 1, '22070011', 'Avenida Atlântica', 'Copacabana', 'Rio de Janeiro', 'RJ'
    UNION ALL SELECT 2, '30130010', 'Avenida Afonso Pena', 'Centro', 'Belo Horizonte', 'MG'
    UNION ALL SELECT 3, '80020310', 'Rua XV de Novembro', 'Centro', 'Curitiba', 'PR'
    UNION ALL SELECT 4, '90020004', 'Rua dos Andradas', 'Centro Histórico', 'Porto Alegre', 'RS'
    UNION ALL SELECT 5, '40060001', 'Avenida Sete de Setembro', 'Vitória', 'Salvador', 'BA'
    UNION ALL SELECT 6, '50050000', 'Rua da Aurora', 'Boa Vista', 'Recife', 'PE'
    UNION ALL SELECT 7, '60025100', 'Rua Major Facundo', 'Centro', 'Fortaleza', 'CE'
    UNION ALL SELECT 8, '70070120', 'SBS Quadra 2', 'Asa Sul', 'Brasília', 'DF'
    UNION ALL SELECT 9, '74015010', 'Avenida Goiás', 'Setor Central', 'Goiânia', 'GO'
) e ON e.k = MOD(p.`pedido_id_origem` * 31, 10)
SET p.`entrega_cep` = e.cep,
    p.`entrega_endereco` = e.rua,
    p.`entrega_numero` = CAST(1 + MOD(p.`pedido_id_origem` * 7919, 1999) AS CHAR),
    p.`entrega_complemento` = COALESCE(p.`entrega_complemento`, ''),
    p.`entrega_bairro` = e.bairro,
    p.`entrega_cidade` = e.cidade,
    p.`entrega_uf` = e.uf
WHERE p.`entrega_cep` IS NULL OR p.`entrega_endereco` IS NULL OR p.`entrega_numero` IS NULL
   OR p.`entrega_bairro` IS NULL OR p.`entrega_cidade` IS NULL OR p.`entrega_uf` IS NULL;
UPDATE `pedidos` SET `entrega_complemento` = '' WHERE `entrega_complemento` IS NULL;
ALTER TABLE `pedidos`
    MODIFY `entrega_cep` CHAR(8) NOT NULL COMMENT 'CEP de entrega, só dígitos (ECOM-17)',
    MODIFY `entrega_endereco` VARCHAR(255) NOT NULL COMMENT 'Rua de entrega (ECOM-17)',
    MODIFY `entrega_numero` VARCHAR(20) NOT NULL COMMENT 'Número de entrega ("s/n" aceito, ECOM-17)',
    MODIFY `entrega_complemento` VARCHAR(120) NOT NULL DEFAULT '' COMMENT 'Complemento de entrega ('''' = sem, ECOM-17)',
    MODIFY `entrega_bairro` VARCHAR(120) NOT NULL COMMENT 'Bairro de entrega (ECOM-17)',
    MODIFY `entrega_cidade` VARCHAR(120) NOT NULL COMMENT 'Cidade de entrega (ECOM-17)',
    MODIFY `entrega_uf` CHAR(2) NOT NULL COMMENT 'UF de entrega (ECOM-17)';
ALTER TABLE `pedidos`
    ADD CONSTRAINT IF NOT EXISTS `chk_pedidos_entrega` CHECK (`entrega_cep` REGEXP '^[0-9]{8}$' AND TRIM(`entrega_endereco`) <> ''
        AND TRIM(`entrega_numero`) <> '' AND TRIM(`entrega_bairro`) <> '' AND TRIM(`entrega_cidade`) <> ''
        AND CHAR_LENGTH(`entrega_uf`) = 2);

-- 6. pagamentos: status Recusado (gateway da loja recusou).
ALTER TABLE `pagamentos`
    MODIFY `status_pagamento` ENUM('Em aberto','Inadimplente','Pago','Pago com atraso','Recusado') NOT NULL COMMENT 'Status atual do pagamento (Recusado = gateway da loja recusou, ECOM-18)';
