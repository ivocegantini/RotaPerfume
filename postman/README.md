# RotaPerfumes API — Postman Collection

> Manual da collection `postman/collection.json` e referência dos endpoints da API Go (`rotaperfumes-api`).
> Fonte da verdade das rotas: `backend/rotaperfumes-api/internal/bootstrap/rotas.go` (77 rotas). Se este manual divergir, vale o código.

## Arquivos

| Arquivo | Descrição |
|---------|-----------|
| `collection.json` | Collection Postman v2.1 ("RotaPerfumes API — Autenticação"), com testes JS por request |
| `README.md` | Este manual |

Não há arquivo de environment no repositório: as variáveis ficam na própria collection.
Para outro ambiente (ex.: QA), crie um environment no Postman que sobrescreva `base_url` e as senhas.

## Como usar

1. No Postman: **Import** → **Upload Files** → `postman/collection.json`.
2. Ajuste `base_url` (padrão `http://localhost:8080`) e preencha `senha_admin` / `senha_vendedor`.
3. Rode **Login** (admin): o teste salva `token`, `admin_token` e `refresh_token`. Para o vendedor, rode **Login — Vendedor**.
4. As requests protegidas usam `Authorization: Bearer {{token}}` (herdado da collection); as de admin usam `{{admin_token}}`.

### Variáveis da collection

| Variável | Padrão | Uso |
|----------|--------|-----|
| `base_url` | `http://localhost:8080` | URL base da API |
| `token` / `admin_token` | vazio | JWT salvo pelo Login (admin) |
| `vendedor_token` | vazio | JWT salvo pelo "Login — Vendedor" |
| `refresh_token` | vazio | Refresh token salvo pelo login/refresh |
| `senha_admin` | vazio | Senha do `admin@rotaperfumes.com.br` no banco **local/dev** (preencher à mão) |
| `senha_vendedor` | vazio | Senha dos usuários de seed (preencher à mão) |
| `imagem_arquivo` | vazio | `<uuid>.<ext>` salvo por "Enviar/Trocar Imagem do Produto" |
| `cookie_acesso` / `cookie_refresh` | `__Host-access_token` / `__Host-refresh_token` | Nomes dos cookies; em dev HTTP são `access_token` / `refresh_token` (SEC-33) |
| `checkout_chave` | vazio | UUID de idempotência do checkout (gerado no pre-request; apague para nova compra) |
| `pedido_loja_id` | vazio | Pedido criado pelo "Finalizar Compra", usado pelo simulador |
| `compra_id` | `1` | Pedido aberto em "Detalhe da Compra" (salvo por "Minhas Compras") |

### Rodar pelo Postman CLI

```bash
postman collection run postman/collection.json \
  --env-var "base_url=http://localhost:8080" \
  --env-var "senha_admin=<senha local>" --env-var "senha_vendedor=<senha do seed>"
```

O login e a troca de senha exigem captcha: em dev use a secret de teste da Cloudflare (`1x…AA`) no `TURNSTILE_SECRET_KEY`.
O `postman` precisa ser o CLI (pacote npm `postman-cli`), não o app desktop (`make setup-check` confere).

## Contas de seed e senhas

Não existe senha fixa. Nunca grave senha de produção na collection nem num environment salvo.

| Conta | Senha | Observação |
|-------|-------|------------|
| `admin@rotaperfumes.com.br` (role `admin`) | `SEED_ADMIN_PASSWORD` do `backend/.env` ou a impressa pelo `make db-seed` (`ADMIN_PASSWORD=...`) | Troca obrigatória no 1º login; `make fix-admin` pede a senha no terminal |
| 42 vendedores `<slug-nome>@rotaperfumes.com.br` (role `normal`) | `SEED_USER_PASSWORD` ou a impressa (`USER_PASSWORD=...`), igual para todos | Nascem com `deve_trocar_senha = 1` |
| `rafael.carvalho@...` (usuário 5, vendedor 4 ativo) | idem | Usado em "Login — Vendedor" |
| `thiago.silva@...` (usuário 4, vendedor 3 desligado) | idem | `ativo = 0` no seed: login dá `401`; para testar o `403` de desligado, reative só no banco de dev |

- **Troca obrigatória (SEC-23):** com `trocar_senha = true`, só `GET /api/auth/me` e `POST /api/auth/reset-password` funcionam; o resto dá `403` `"troca de senha obrigatória"`.
- **Antes de usar a collection com um seed:** login → "Reset Password (própria senha)" (derruba a sessão, SEC-08) → login de novo.
- **Produção:** a senha inicial do admin vem do `deploy/seed-producao.sh` (`docs/deploy-servidor.md` §11); os vendedores nascem inativos.
- **Usuários novos e reset pelo admin:** senha aleatória (`crypto/rand`, 12+ caracteres) enviada por e-mail, nunca na resposta; a resposta traz `email_enviado`.

## Convenções da API

| Item | Regra |
|------|-------|
| Envelope | Sucesso `{"success":true,"data":...}`; erro `{"success":false,"error":"..."}` (sem `data`; chaves em ordem alfabética) |
| Políticas | `Public` (sem token) · `Authenticated` (equipe: `admin` e `normal`) · `AdminOnly` (`admin`) · `AnyRole` (inclui o `usuario` da loja) |
| Role `usuario` | Em rota `Authenticated`/`AdminOnly` → `403` `"acesso restrito à equipe interna"` (ECOM-01) |
| Sessão | Bearer ou cookie de acesso; `ativo`, `role` e corte de sessão lidos do banco a cada request, sem cache (SEC-06/SEC-08) |
| Ordem dos erros | `401` (token, inexistente, inativo, sessão encerrada) / `500` → `403` troca de senha → `403` papel |
| Método | Só o método declarado; outro → `405` com `Allow` (texto, sem envelope). `HEAD` casa com `GET`; `OPTIONS` → `204` (CORS) |
| Id de path | Só dígitos e `> 0`; senão `400` `"id inválido"` sem tocar o banco (CHORE-03) |
| Paginação | `page` (máx. 10000, clamp, SEC-16) e `limit` (máx. 100); resposta com `pagination` (`total`, `pages`) |
| Ordenação | `order_by`/`order_dir` (`asc`/`desc`); valor fora da lista cai no padrão, sem `400` |
| Busca `q` | Até 100 caracteres (senão `400` `"o parâmetro q deve ter no máximo 100 caracteres"`); `%` e `_` são literais (SEC-25) |
| Corpo | Até 256 KiB (`413` `"corpo da requisição excede o limite de 256 KB"`); headers acima de 64 KiB → `431` (SEC-14) |
| Datas | `AAAA-MM-DD`, fuso fixo `-03:00`; `PUT`/`PATCH` sem alteração responde `200` |
| Toggle `PATCH .../inativar` | Body vazio/`null`/`{}`/`{"ativo":null}` → inverte (atômico); `{"ativo":bool}` → define; outro body → `400` `"body JSON inválido"` |
| Cookies (SEC-21/SEC-33) | `COOKIE_SECURE=true`: `__Host-access_token` (Strict) e `__Host-refresh_token` (Lax); `false` (dev): nomes sem prefixo |
| IP do cliente (SEC-20) | Item mais à direita do `X-Forwarded-For` (validado); `X-Real-IP` não é lido |

## Endpoints

### Saúde e ambiente (Public)

| Método | Rota | Resposta |
|--------|------|----------|
| GET | `/health` | `{"status":"ok"}` (Caddy e smoke do Jenkins) |
| GET | `/api/ambiente` | `{"ambiente":"dev"\|"qa"\|"producao"}` do `APP_ENV` (ausente = `producao`); selo do rodapé |

### Autenticação (`/api/auth/*`)

| Método | Rota | Política | Body / observação |
|--------|------|----------|-------------------|
| POST | `/api/auth/login` | Public | `email`, `password`, `captchaToken` → `access_token`, `refresh_token`, `token_type`, `expires_in`, `user`, `trocar_senha` |
| POST | `/api/auth/refresh` | Public | `refresh_token` (body ou cookie) → novo par; uso único |
| POST | `/api/auth/logout` | Public | `refresh_token` opcional → `{"mensagem":"logout realizado"}`; limpa os cookies |
| POST | `/api/auth/cadastro` | Public | Autocadastro da loja (ECOM-01/03/05) |
| POST | `/api/auth/google` | Public | `credential` (ID token Google) (ECOM-02) |
| POST | `/api/auth/google/cadastro` | Public | `credential` + CPF e endereço (ECOM-02) |
| POST | `/api/auth/confirmar-email` | Public | `token` do link (`#token=`) (ECOM-03) |
| POST | `/api/auth/reenviar-confirmacao` | Public | `email`, `captchaToken`; sempre `200` neutro |
| POST | `/api/auth/reset-password` | AnyRole | `senha_atual`, `nova_senha`, `captchaToken`; liberada com troca pendente |
| GET | `/api/auth/me` | AnyRole | `id, nome, email, role, ativo, id_vendedor, trocar_senha, created_at, ultimo_login_at, vendedor_desligado` |

**Login**
- Inexistente, inativo e senha errada respondem igual: `401` `"credenciais inválidas"`, mesmo custo de hash (SEC-22, SEC-32).
- Conta da loja com e-mail pendente → `403` `"email não confirmado: abra o link que enviamos para o seu email"`.
- Rate limit por IP: 5 falhas em 1 min bloqueiam 5 min → `429` com `Retry-After`.

**Captcha Turnstile (login, reset-password, cadastro, reenviar-confirmacao)**
- `captchaToken` obrigatório, validado no siteverify (fail-closed, 4 s); falha → `400` `"captcha inválido ou ausente"`, antes do rate limit.
- Confere `hostname` (`TURNSTILE_HOSTNAMES` ou hosts do `CORS_ALLOWED_ORIGINS`) e `action` (`login`, `reset-senha`, `cadastro`, `reenviar-confirmacao`) (SEC-19).
- Com secret de teste da Cloudflare, hostname e action não são conferidos (aviso no startup).

**Refresh**
- Rate limit por IP: 10 falhas em 1 min bloqueiam 5 min (`429`). Erros: `400` `"refresh_token é obrigatório"`, `401`, `429`, `500`.
- Token revogado há até 30 s ou corrida na rotação → `401` `"refresh token revogado"` sem contar no rate limit (SEC-02/SEC-04).
- Falha ao revogar ou gravar o novo token → `500` `"erro interno"`, sem cookie e sem par novo.

**Reuso de refresh (SEC-07/SEC-09/SEC-12)**
- Reuso fora dos 30 s de token revogado por `rotacao` → revoga **todas** as sessões (`revogacao_massa`), grava corte e alerta por e-mail (usuário + `SECURITY_ALERT_EMAILS`).
- Novo reuso do mesmo token dentro de `REFRESH_REUSE_SUPPRESS_WINDOW` (30 min) não corta de novo nem reenvia e-mail; a resposta é sempre a mesma `401`.
- Usuário inativo ou apagado com token ativo: o token é revogado (`inativacao`) e a resposta é `401` `"refresh token revogado"` (SEC-22, BUG-23).

**Corte de sessão (SEC-08)**
- Access token com `iat <= usuarios.tokens_validos_desde` → `401` `"sessão encerrada — faça login novamente"`, mesmo dentro do TTL.
- Gravam o corte: inativar usuário, desligar vendedor, troca e reset de senha, reuso de refresh e a CLI `resetpassword`. Reativação e logout não.
- Usuário inativo ou inexistente com token válido → `401` `"usuário inativo"`; erro de banco na checagem → `500` (fail-closed).

**Reset-password (troca da própria senha)**
- Rate limit: 10 falhas em 5 min bloqueiam 10 min. Histórico, senha nova e revogação na mesma transação: falha → `500` sem trocar (BUG-15).
- A troca derruba todas as sessões, inclusive a atual (corte SEC-08); o frontend leva ao login.
- Outra troca concorrente venceu → `409` `"a senha foi alterada por outra operação; tente novamente"` (BUG-24); conta da loja pendente → `409` `"troca de senha bloqueada: email pendente de confirmação"`.

**Loja: cadastro, Google e confirmação (ECOM-01 a 07)**
- Cadastro: `nome`, `email`, `password` (8+ caracteres, 3 de 4 classes), `cpf` (com DV), `cep`, `endereco`, `numero`, `complemento`, `bairro`, `cidade`, `uf`, `captchaToken`; cria role `usuario` + cliente PF (segmento `Loja Online`) numa transação.
- `201` com `email_enviado` e sem sessão (e-mail pendente). E-mail ou CPF repetido → `409` neutro. Domínio fora de `CADASTRO_EMAIL_DOMINIOS` → `400`. Limite: 5 por IP em 10 min.
- Google: valida o ID token (RS256/JWKS, `iss`, `aud = GOOGLE_CLIENT_ID`, `exp`, `email_verified`); sem conta → `200` `{cadastro_necessario, email, nome}`; equipe → `403`; sem `GOOGLE_CLIENT_ID` → `403`.
- `google/cadastro` cria a conta já confirmada e abre a sessão (`201`). Confirmação: link de 24 h; inválido, usado ou expirado → `400` `"link de confirmação inválido ou expirado"` (conta no rate limit: 10 em 10 min).

### Usuários e histórico de senhas (AdminOnly)

| Método | Rota | Body / query | Observação |
|--------|------|--------------|------------|
| GET | `/api/usuarios` | `page, limit, order_by, order_dir` | `order_by`: `id, nome, email, role, ativo, created_at, updated_at, ultimo_login_at` |
| POST | `/api/usuarios` | `nome, email, role` (`admin`\|`normal`) | `201` com `email_enviado`; `409` `"email já cadastrado"`; senha por e-mail |
| PUT | `/api/usuarios/{id}` | `nome, role` | |
| PATCH | `/api/usuarios/{id}/inativar` | `{"ativo":bool}` opcional | Inativar revoga os refresh e grava o corte, na mesma tx (BUG-22) |
| POST | `/api/admin/reset-password` | `usuario_id` | Senha aleatória por e-mail, `deve_trocar_senha = 1`, revoga sessões e grava corte; `404` inexistente |
| GET | `/api/senha-historico` | `page, limit, order_by, order_dir` (padrão `desc`) | `order_by`: `id, usuario_id, tipo_reset, created_at, usuario_nome, resetado_por_nome, ip_origem` |
| GET | `/api/senha-historico/{user_id}` | idem | Id vazio → `400` `"usuario_id é obrigatório na URL"`; `0`, `-1`, `/2/extra` → `400` `"usuario_id inválido"` |

- Histórico: desempate por `id DESC`; NULL no banco sai `""` (IP/UA) ou data zero (BUG-12).
- Sem SMTP (`SMTP_*`), o e-mail vira "noop" (só log, nunca a senha) e `email_enviado` é `false`.

### Vendedores

| Método | Rota | Política | Observação |
|--------|------|----------|------------|
| GET | `/api/vendedores` | Authenticated | Admin: todos. `normal`: só o próprio (`id, nome, regiao, uf, data_desligamento`); sem vínculo `[]`; desligado `403` (SEC-03) |
| GET | `/api/vendedores/{id}/clientes` | Authenticated | Carteira ativa do vendedor (dropdown Vendedor → Cliente); `404` vendedor inexistente |
| POST | `/api/vendedores` | AdminOnly | Cria vendedor |
| GET | `/api/vendedores/{id}` | AdminOnly | Detalhe com `meta_mensal` (a listagem não traz meta) |
| PUT | `/api/vendedores/{id}` | AdminOnly | |
| DELETE | `/api/vendedores/{id}` | AdminOnly | Desliga (`data_desligamento`), revoga sessões dos usuários ligados e grava corte; já desligado → `200` |
| POST | `/api/vendedores/{id}/reativar` | AdminOnly | |
| POST | `/api/vendedores/{id}/clientes` | AdminOnly | Vincula/transfere cliente (tx com lock); mesmo par na mesma data → `409` `"vínculo entre cliente e vendedor já cadastrado nesta data"` |
| DELETE | `/api/vendedores/{id}/clientes/{clienteId}` | AdminOnly | Encerra vínculo; `{clienteId}` inválido → `400` `"cliente id inválido"` |

### Escopo por vendedor (carteiras)

Vale para Clientes, Pedidos, Pagamentos, Oportunidades, Visitas, `GET /api/vendedores*` e Dashboard. O admin não tem restrição.

| Situação do `normal` | Leitura | Escrita |
|----------------------|---------|---------|
| Com vendedor ativo | Só os registros do próprio vendedor / carteira ativa (`carteiras.data_fim IS NULL`) | `vendedor_id` do body é ignorado e forçado; `cliente_id` fora da carteira → `400` `"cliente não pertence à carteira deste vendedor"` |
| Sem vendedor | Lista vazia; detalhe `404` | `POST` → `403` `"usuário sem vendedor vinculado"`; `PUT`/`DELETE` → `404` |
| Vendedor desligado | `403` `"acesso bloqueado: vendedor desligado"` (Dashboard: `200` zerado) | `403` idem |

- Registro de outro vendedor → `404` com a mesma mensagem de inexistente (sem enumeração). Falha ao resolver o escopo → `500`.
- O pedido pertence a quem o registrou (`pedidos.vendedor_id`): após transferir o cliente, o novo vendedor não vê os pedidos antigos e o original não consegue editá-los (`400`).

### Clientes (Authenticated, escopo pela carteira)

| Método | Rota | Body / query |
|--------|------|--------------|
| GET | `/api/clientes` | `page, limit, uf, segmento, ativo, q` (razão social ou CNPJ), `order_by` (`id, razao_social, cnpj, segmento, cidade, uf, data_cadastro, ativo, created_at, updated_at`) |
| POST | `/api/clientes` | `cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro` (opcional, padrão hoje) |
| GET | `/api/clientes/{id}` | Id = `cliente_id_origem` (não há campo `id`) |
| PUT | `/api/clientes/{id}` | Mesmo body do POST (`ativo` e id não são editáveis) |
| PATCH | `/api/clientes/{id}/inativar` | `{"ativo":bool}` opcional; o body só é lido depois do escopo |

- `POST` do `normal` vincula o cliente à carteira do vendedor na mesma tx; o admin cria sem vínculo. O `201` traz os timestamps reais.
- **CNPJ (NEG-01/02/04):** numérico ou alfanumérico (12 posições `[0-9A-Z]` + 2 DVs numéricos, módulo 11 com ASCII − 48); aceita máscara e minúsculas; grava 14 caracteres em maiúsculas.
- DV exigido em toda gravação, inclusive no `PUT` com o mesmo CNPJ → `400` `"cnpj inválido"`; repetido → `409` `"cnpj já cadastrado"` (índice `uq_clientes_cnpj`). Exemplos: `11222333000181`, `12ABC34501DE35`.
- Busca `q` com máscara é normalizada para o CNPJ; a razão social usa o `q` como veio.

### Produtos e imagens

| Método | Rota | Política | Observação |
|--------|------|----------|------------|
| GET | `/api/produtos` | Authenticated | `page, limit, categoria, marca, ativo, q` (`nome` ou `sku`), `order_by` (`id, sku, nome, categoria, marca, preco_tabela, custo_unitario, data_lancamento, ativo, created_at, updated_at`) |
| GET | `/api/produtos/{id}` | Authenticated | Detalhe |
| POST | `/api/produtos` | AdminOnly | `sku, nome, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento`; SKU repetido → `409` `"sku já cadastrado"` |
| PUT | `/api/produtos/{id}` | AdminOnly | Mesmo body sem `sku` (`sku` e `ativo` não editáveis) |
| PATCH | `/api/produtos/{id}/inativar` | AdminOnly | Exclusão lógica (não há `DELETE`); toggle atômico |
| POST | `/api/produtos/{id}/imagem` | AdminOnly | Upload multipart (abaixo) |
| DELETE | `/api/produtos/{id}/imagem` | AdminOnly | `200` com `imagem_url: null`; idempotente; `404` produto inexistente |
| GET | `/api/public/produtos/imagens/{arquivo}` | Public | Serve a imagem (abaixo) |

- **Campos (ECOM-11):** `nome` é o nome curto; `descricao` é o texto da página do produto (opcional, até 5000 caracteres). Estoque e itens de pedido devolvem `produto_nome`.
- `imagem_url`: `"/api/public/produtos/imagens/<uuid-v4>.<jpg|png|webp>"` ou `null`; o diretório do servidor nunca é exposto.

**Upload (`POST /api/produtos/{id}/imagem`, IMG-01/SEC-34)**
- Header `X-Requested-With: XMLHttpRequest` obrigatório (senão `403` `"requisição de upload recusada"`); `Origin`, se houver, precisa ser permitida.
- Multipart com **uma** parte `imagem` (JPEG, PNG ou WebP, até 1 MB). Tipo detectado pelo conteúdo; 1 a 4096 px por lado, até 16 Mpx.
- JPEG/PNG são reencodados; WebP vira PNG (com transparência) ou JPEG. Saída até 4 MiB. Nome `<uuid-v4>.<ext>`, escrita atômica em `PRODUTO_IMAGENS_DIR`.
- Ordem: policy → id → anti-CSRF → corpo (`413` `"imagem excede 1 MB"` / `400` `"arquivo de imagem inválido (use JPEG, PNG ou WebP)"`) → produto (`404`).

**Leitura pública**
- `arquivo` = `<uuid-v4>.jpg|.png|.webp` em minúsculas; fora do padrão ou ausente → `404` `"imagem não encontrada"`. Não lê token nem envia `Set-Cookie`.
- `Content-Type` fixo por extensão, `Cache-Control: public, max-age=31536000, immutable`, `Cross-Origin-Resource-Policy: same-site`, `nosniff`. Aceita `HEAD` e `Range` (`206`).
- Em dev, use o mesmo host (`localhost`) no frontend e na API, por causa do `same-site`. Cache Redis na área `imagens` (CACHE-01).

### Pedidos (Authenticated, escopo por `pedidos.vendedor_id`)

| Método | Rota | Body / query |
|--------|------|--------------|
| GET | `/api/pedidos` | `page, limit, status, canal, cliente_id, vendedor_id, data_inicio, data_fim, q` (razão social), `order_by` (`id, data_pedido, canal, status, valor_total, created_at, updated_at, cliente_nome, vendedor_nome`; padrão `desc`) |
| POST | `/api/pedidos` | `cliente_id, vendedor_id, data_pedido, canal, status, itens: [{produto_id, quantidade, preco_praticado, desconto_pct}]` |
| GET | `/api/pedidos/{id}` | Detalhe com `itens` (`produto_sku`, `produto_nome`) |
| PUT | `/api/pedidos/{id}` | Mesmo body; substitui os itens |
| DELETE | `/api/pedidos/{id}` | Hard delete → `204` |

- Status: `Aguardando pagamento`, `Em separação`, `Faturado`, `Entregue`, `Cancelado`. Canal: `App`, `Telefone`, `Visita`, `WhatsApp`, `Loja`.
- `valor_bruto` e `valor_total` são calculados no backend. Mínimo 1 e máximo 200 itens (`400` `"o pedido deve ter no máximo 200 itens"`).
- Criado como `Faturado` baixa o estoque na mesma tx; criado como `Entregue` → `400` (BUG-20).

**Máquina de status no PUT (BUG-16, checada sob lock antes dos itens)**

| Status gravado | Destinos aceitos | Recusa (`400`) |
|----------------|------------------|----------------|
| Em separação / Cancelado | Em separação, Cancelado, Faturado (baixa o estoque) | Entregue → `"pedido só pode ser marcado como Entregue depois de Faturado"` |
| Faturado | Faturado (idempotente), Entregue | outro → `"pedido faturado só pode ter o status alterado para Entregue"` |
| Entregue (terminal) | Entregue | outro → `"pedido entregue não pode ter o status alterado"` |

- Faturado ou Entregue com itens diferentes → `409` `"pedido faturado ou entregue: não é possível alterar os itens"`. A baixa acontece uma vez só.
- **DELETE:** `409` `"pedido possui pagamentos vinculados"` (checado primeiro), `409` `"pedido faturado não pode ser excluído; só pode ter o status alterado para Entregue"`, `409` `"pedido entregue não pode ser excluído"`.
- Erro de banco ao ler a posse ou no lock → `500` (SEC-38).

### Pagamentos (Authenticated, escopo herdado do pedido)

| Método | Rota | Body / query |
|--------|------|--------------|
| GET | `/api/pagamentos` | `page, limit, status_pagamento, forma_pagamento, pedido_id, vencimento_de, vencimento_ate`, `order_by` (`pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento, created_at, updated_at`) |
| POST | `/api/pagamentos` | `pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento` (opcional), `status_pagamento` |
| GET | `/api/pagamentos/{id}` | Id = `pagamento_id` |
| PUT | `/api/pagamentos/{id}` | Mesmo body sem `pedido_id` (vínculo definitivo) |
| DELETE | `/api/pagamentos/{id}` | Hard delete → `204` |

- Formas: `Boleto 14 dias`, `Boleto 28 dias`, `Cartão de crédito`, `Cartão de débito`, `Cheque a prazo`, `Dinheiro`, `PIX`. Status: `Em aberto`, `Inadimplente`, `Pago`, `Pago com atraso`, `Recusado`.
- `taxa_pct` de 0 a 30, inclusive → senão `400` `"taxa_pct deve estar entre 0 e 30"` (SEC-27; CHECK `chk_pagamentos_taxa_pct`). `valor_liquido` é sempre enviado, não calculado.
- `pedido_id` ausente/`<= 0` → `400` `"pedido_id é obrigatório"`; inexistente ou de outro vendedor → `404` `"pedido não encontrado"` (inclusive apagado durante a criação, SEC-35).
- Quitado (`Pago`/`Pago com atraso`): `PUT` do `normal` → `409` `"pagamento já quitado não pode ser editado"` (admin edita); `DELETE` → `409` `"pagamento já quitado não pode ser excluído"`.

### Oportunidades e visitas (Authenticated, escopo por `vendedor_id`)

| Método | Rota | Body / query |
|--------|------|--------------|
| GET | `/api/oportunidades` | `page, limit, cliente_id, vendedor_id, etapa, origem, data_abertura_de, data_abertura_ate, q` (origem ou etapa), `order_by` (`id, data_abertura, valor_estimado, probabilidade_pct, etapa, origem, created_at, updated_at`) |
| POST / PUT | `/api/oportunidades`, `/api/oportunidades/{id}` | `cliente_id, vendedor_id, origem, data_abertura` (POST: padrão hoje; PUT: obrigatória), `etapa, probabilidade_pct` (0–100), `valor_estimado` (≥ 0), `data_fechamento, ciclo_dias, motivo_perda` (obrigatório em "Fechado perdido") |
| GET / DELETE | `/api/oportunidades/{id}` | Detalhe; exclusão definitiva → `204` |
| GET | `/api/visitas` | `page, limit, cliente_id, vendedor_id, resultado, data_visita_de, data_visita_ate, q` (resultado), `order_by` (`id, visita_id, data_visita, duracao_min, resultado, created_at, updated_at`) |
| POST / PUT | `/api/visitas`, `/api/visitas/{id}` | `cliente_id, vendedor_id, data_visita` (obrigatória, sem padrão), `resultado` (não vazio), `duracao_min` (≥ 0) |
| GET / DELETE | `/api/visitas/{id}` | Detalhe; exclusão definitiva → `204` |

- `data_visita` vazia → `400` `"data_visita inválida (use o formato AAAA-MM-DD)"`. Exclusões sem regra extra de negócio.
- Para o admin, `vendedor_id` da oportunidade não depende do vínculo de carteira; para o `normal` vale a regra de escopo acima.

### Estoque (AdminOnly)

| Método | Rota | Body / query |
|--------|------|--------------|
| GET | `/api/estoque` | `page, limit, sku, ruptura, data_de, data_ate, historico`, `order_by` (`id, sku, data_snapshot, saldo, ruptura, created_at, updated_at`; padrão `data_snapshot desc`) |
| GET | `/api/estoque/{id}` | Detalhe com `produto_nome` |
| POST | `/api/estoque` | `sku, data_snapshot, saldo`; par repetido → `409` `"registro de estoque já cadastrado para este sku nesta data"` |
| PUT | `/api/estoque/{id}` | `saldo` (sobrescrita absoluta; `sku` e data imutáveis) |

- Sem `historico`: última posição de cada SKU (até `data_ate`, se informada). `historico=true`: série completa. Não há `DELETE`.
- `ruptura = saldo <= 0`, derivada. `UNIQUE (data_snapshot, sku)`: o faturamento faz upsert no snapshot do dia (`time.Now()`), sob `FOR UPDATE`.

### Dashboard (Authenticated, escopo por vendedor)

| Método | Rota | Query | Resposta |
|--------|------|-------|----------|
| GET | `/api/dashboard/metrics` | `periodo=today\|week\|month`, `ano`, `mes` | `total_vendas, total_vendas_qtd, total_pedidos, ticket_medio, top_vendedores, metas_vendedores, meta_mes, vendedor_desligado` |
| GET | `/api/dashboard/vendas` | `dias` (padrão 30, máx. 365) | `{dias, pontos: [{dia, total_vendas, total_pedidos}]}`, um ponto por dia (zerado se sem venda) |
| GET | `/api/dashboard/vendedores` | `page, limit` | Ranking por `meta_mensal DESC`, desempate por atingimento |
| GET | `/api/dashboard/clientes` | `periodo` (padrão `month`) | `total_clientes, total_ativos, total_inativos, novos_no_periodo, por_segmento, por_uf` (carteira ativa) |
| GET | `/api/dashboard/funil` | `janela=30d\|90d\|365d\|all` (padrão `all`) | `janela, total_oportunidades, etapas[5], perdidas, taxa_ganho_pct, vendedor_desligado` (DASH-01) |

- Admin vê tudo; `normal` vê só o próprio vendedor; sem vendedor, desligado ou inexistente → `200` zerado (`vendedor_desligado: true` só no desligado).
- `week` = últimos 7 dias corridos. Listas vazias vêm `[]`, nunca `null`; valores em `float64` com centavos.
- Funil cumulativo Prospecção → Qualificação → Proposta enviada → Negociação → Fechado ganho; janela fora do enum (inclusive `30D`) → `400` `"janela deve ser '30d', '90d', '365d' ou 'all'"`.

### Loja online (vitrine Public; compras AnyRole)

| Método | Rota | Política | Observação |
|--------|------|----------|------------|
| GET | `/api/loja/produtos` | Public | Vitrine: `page, limit, q, categoria, marca, nota_olfativa, order_by` (`nome`, `categoria`, `marca`, `preco_tabela`); só ativos, sem custo/ativo/datas; preço em `preco` |
| GET | `/api/loja/produtos/{id}` | Public | Item da vitrine + `descricao`; inexistente **ou inativo** → `404` `"produto não encontrado"` |
| GET | `/api/loja/filtros` | Public | `categorias`, `marcas`, `notas_olfativas` dos ativos, em ordem alfabética |
| POST | `/api/loja/checkout` | AnyRole | Fecha o carrinho do cliente da loja do token (ECOM-14/17/18) |
| GET | `/api/loja/minhas-compras` | AnyRole | Pedidos do usuário do token: `pedido_id, data, status, valor_total, produtos`; `order_by` `data` (padrão), `pedido_id`, `valor_total` |
| GET | `/api/loja/minhas-compras/{id}` | AnyRole | Detalhe com `itens` e `pagamentos` (sem taxa nem líquido); de outra pessoa → `404` |
| POST | `/api/loja/pagamentos/{id}/simular` | AnyRole | Simulador do gateway, só `APP_ENV=dev\|qa` (ECOM-18) |

- Vitrine e página do produto: 300 req/min por IP (`429`); cache Redis fail-open nas áreas `vitrine`, `filtros` e `imagens` (CACHE-01, `docs/cache-redis.md`).
- Minhas compras: o usuário vem só do token (id na query é ignorado); login sem cliente da loja → `data: []`.

**Checkout (`POST /api/loja/checkout`)**
- Body: `chave` (UUID de idempotência), `forma_pagamento` (`pix`, `cartao`, `boleto`), `itens: [{produto_id, quantidade}]` (1–99 cada, até 50 itens, sem repetir) e `entrega`.
- `entrega` obrigatória: `cep` (8 dígitos), `endereco`, `numero` (`s/n` aceito), `bairro`, `cidade`, `uf`; `complemento` opcional. Faltando → `400` sem gravar (ECOM-17).
- Preço do banco; cria pedido canal `Loja`, vendedor "Loja Online", `Aguardando pagamento` + pagamento `Em aberto` (PIX/cartão vencem hoje; boleto em 14 dias) numa tx; mesma chave → `200` `repetida: true`.
- Estoque disponível = último snapshot − unidades em pedidos `Loja` aguardando ou em separação, com o produto travado na tx (SEC-79).
- Limite: 10 por usuário e 30 por IP em 10 min → `429` por 15 min. Erros: `400`, `401`, `403` (login sem cliente da loja), `409` (inativo/inexistente, sem estoque, chave de outro cliente).

**Pagamento da loja (ECOM-18/19)**
- Simular: body `{"resultado":"aprovado"|"recusado"}`. Aprovado → pedido `Em separação` + pagamento `Pago` (hoje); recusado → `Cancelado` + `Recusado` (libera a reserva).
- Só admin ou dono do pedido. Erros: `400` resultado inválido; `404` em produção, inexistente ou alheio; `409` pedido que não está aguardando pagamento.
- Job interno (`LOJA_EXPIRACAO_INTERVAL`, padrão 1h, `0` desliga) cancela os aguardando com vencimento antes de hoje (`Cancelado` + `Recusado`).

## Testes da collection

Cada request tem testes JS (`pm.test`) de status e contrato. Os principais:

| Pasta / request | Confere | Salva |
|-----------------|---------|-------|
| Login (admin) | `200`, `user.role`, `access_token`, `refresh_token` | `token`, `admin_token`, `refresh_token` |
| Login — Vendedor | `200`, role `normal`, `trocar_senha` booleano, tokens | `vendedor_token`, `refresh_token` |
| Refresh Token | `200`, novo par, `expires_in` | `token`, `refresh_token` |
| Me | `200`, dados, `vendedor_desligado` booleano | — |
| Ambiente da API | `ambiente` em `dev`/`qa`/`producao` | — |
| Cadastro / Confirmar / Reenviar | `201`\|`409` sem sessão; `200`\|`400`; `200` neutro | — |
| Usuários, Histórico de Senhas | `200`/`201`, paginação, `email_enviado` booleano | — |
| Dashboard (5 rotas, funil com 30d/90d/365d e janela inválida) | estrutura, listas como array, `400` da janela | — |
| Clientes, Produtos, Pedidos, Pagamentos, Oportunidades, Visitas, Estoque | CRUD (`200`/`201`/`204`), paginação, ids (`cliente_id_origem`, `pedido_id_origem`, `pagamento_id`) | — |
| Pagamentos e "Listar Clientes do Vendedor" | o `normal` não recebe `403` (acesso comum) | — |
| Imagem do produto (enviar → ler → remover) | `imagem_url` UUID, headers de cache/segurança, sem `Set-Cookie`, `null` após remover | `imagem_arquivo` |
| Loja: filtros, vitrine | listas; sem `custo_unitario` | — |
| Minhas Compras / Detalhe | um pedido por linha; sem `taxa_pct`/`valor_liquido` | `compra_id` |
| Finalizar Compra | `201`\|`200`, `Aguardando pagamento` + `Em aberto`, sem `vendedor_id` | `checkout_chave`, `pedido_loja_id` |
| Simular Pagamento (aprovado/recusado) | `200`\|`409`; status do pedido e do pagamento | — |

- Rode a imagem na ordem enviar → leitura pública → remover (remover antes dá `404`). Escolha o arquivo na parte `imagem` do Body.
- Os testes de Dashboard com `{{vendedor_token}}` também passam: os números vêm restritos ou zerados, mas a estrutura é a mesma.
- O simulador só passa com a API em `APP_ENV=dev` ou `qa`; na produção a rota responde `404`.

## Códigos de erro comuns

| Código | Quando |
|--------|--------|
| 400 | Body/campos inválidos, `"id inválido"`, `q` longo, mais de 200 itens, transição de status proibida, `taxa_pct` fora de 0–30, CNPJ inválido, captcha |
| 401 | Sem token, `"credenciais inválidas"`, `"usuário inativo"`, `"sessão encerrada — faça login novamente"`, `"refresh token revogado"` |
| 403 | `"acesso restrito a administradores"`, `"acesso restrito à equipe interna"`, `"troca de senha obrigatória"`, `"usuário sem vendedor vinculado"`, `"acesso bloqueado: vendedor desligado"`, e-mail não confirmado, upload recusado |
| 404 | Inexistente ou fora do escopo (mesma mensagem); simulador fora de dev/qa |
| 405 | Método diferente do declarado (header `Allow`) |
| 409 | Duplicidade (e-mail, CNPJ, CPF, SKU, estoque do dia, vínculo do dia), pedido Faturado/Entregue, pagamento quitado, estoque insuficiente no checkout, senha alterada por outra operação |
| 413 | Corpo acima de 256 KiB; `"imagem excede 1 MB"` no upload |
| 429 | Rate limit (login, refresh, reset, cadastro, confirmação, vitrine, checkout), com `Retry-After` |
| 431 | Headers acima de 64 KiB |
| 500 | `"erro interno"` (mensagem genérica; checagens de sessão e escopo são fail-closed) |

## Subindo o ambiente

- Ferramentas: `make setup-check` (confere) e `make setup` (instala Go, Node, govulncheck, cliente `mariadb` ≥ 11.4 e Postman CLI). Detalhes no `CLAUDE.md`.
- Copie `backend/.env.example` para `backend/.env` e preencha `DB_USUARIO`/`DB_SENHA` (obrigatórios, SEC-11), `JWT_SECRET` (32+ bytes), `TURNSTILE_SECRET_KEY`, `SMTP_*` e `SEED_*_PASSWORD`.
- Banco de dev: MariaDB do CasaOS `192.168.168.106:3307` com `DB_TLS_CA=deploy/ca-cert.pem`. Host fora do loopback sem CA derruba a API (SEC-42).

```bash
make db-reset        # recria o banco: sql/schema.sql + seeds sql/seed_*.sql (só dev/descartável)
make db-migrar       # leva um banco existente à versão atual (sql/migracao.sql, idempotente)
make dev-api         # API em http://localhost:8080 (COOKIE_SECURE=false)
make dev-frontend    # frontend em http://localhost:3000
```

| Comando | Carrega |
|---------|---------|
| `make db-import-clientes` | `dados/crm/clientes.csv` |
| `make db-import-produtos` | `dados/erp/produtos.csv` |
| `make db-import-pedidos` | `dados/erp/pedidos.csv` e `itens_pedido.csv` |
| `make db-import-pagamentos` | `dados/erp/pagamentos.csv` (depois dos pedidos) |
| `make db-import-carteiras` | `dados/crm/carteira.csv` |
| `make db-import-oportunidades` / `db-import-visitas` | `dados/crm/oportunidades.csv` / `visitas.csv` |
| `make db-import-estoque` | `dados/erp/estoque.csv` (depois dos produtos) |
| `make db-gerar-imagens-produtos` | PNG 600x600 com o nome para produtos sem imagem (`GERAR_IMAGENS_FLAGS=-sobrescrever`) |

- Importadores são upsert idempotentes; sem eles as rotas funcionam com base vazia. `make db-rebuild` recria e importa tudo.
- `make db-reset`/`db-seed`/`db-down`/`db-rebuild` são recusados sempre na produção; os demais exigem `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` (DEPLOY-07, SEC-46).
- Imagens em `PRODUTO_IMAGENS_DIR` (padrão `./data/produtos-imagens`); vazio, `/` ou dentro de `frontend/public` impede a subida. `CORS_ALLOWED_ORIGINS` não aceita `*` nem `null` (SEC-17).

## Roteiros de teste manual

| Roteiro | Cobre |
|---------|-------|
| `docs/roteiro-teste-sessao-autenticacao.md` | Login, refresh, reuso, corte de sessão, troca obrigatória, captcha |
| `docs/roteiro-teste-clientes.md` | Escopo da carteira e CNPJ |
| `docs/roteiro-teste-pedidos-pagamentos.md` | Máquina de status, baixa de estoque, pagamentos quitados |
| `docs/roteiro-teste-dashboard.md` | Métricas, ranking, clientes e funil |
| `docs/roteiro-teste-produtos-imagens.md` | Upload, leitura pública e remoção da imagem |
| `docs/roteiro-teste-smoke.md` | Smoke pós-deploy |
