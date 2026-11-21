# Invariantes de segurança da API (ARQ-00)

> Origem: 🟣 SecBrain (ARQ-00); referência obrigatória para mudanças na API. Os GAPs da auditoria (SEC-14 a SEC-25) estão resolvidos e incorporados abaixo.
> Raízes: `API` = backend/rotaperfumes-api, `SH` = backend/shared. Onde mora cada ponto na estrutura DDD: `docs/arquitetura-ddd.md`, seção 10.

## 0. Regras globais

- **R1 Paridade de testes:** porte os testes no mesmo lote da mudança; proibido apagar teste ou usar `t.Skip`. Testes `SEC*`, `TST05*`, `Escopo*`, `MAIL01*`, `SEC10_TLS*` passam sem mudar asserções.
- **R2 Matriz de rotas:** cada (método, rota) tem política `Public | Authenticated | AdminOnly`, sem default público; sem token → 401, normal em rota admin → 403, rota pública não consulta o checker.
- **R3 Fail-closed:** erro de infraestrutura → 500 "erro interno", nunca "liberar" ou "sem filtro". **R4:** sem cache de usuário, escopo ou vendedor entre requests (SEC-06/08).

## Fundação (F)

- **F-01** CORS mais externo; OPTIONS → 204 antes da autenticação; respostas 401 já trazem os headers de segurança.
- **F-02** CORS com allowlist exata, `Allow-Credentials: true` e `Vary: Origin`; `*` ou `null` em `CORS_ALLOWED_ORIGINS` impedem a subida (SEC-17); origens de dev fora da produção (SEC-36).
- **F-03** Headers: nosniff, `X-Frame-Options DENY`, Referrer-Policy, CSP `default-src 'none'; frame-ancestors 'none'`, HSTS exceto loopback.
- **F-04** Timeouts HTTP 15s/60s, shutdown 10s, `ReadHeaderTimeout` 5s, `MaxHeaderBytes` 64 KiB; corpo limitado por rota (`MaxCorpoPadrao` 256 KiB; upload de imagem à parte), 413 depois de 401/403; pedido com até 200 itens (SEC-14).
- **F-05** Envelope de erro: erro reconhecido → mensagem fixa; desconhecido → log + 500. Mapper só com sentinelas (`errors.Is`), nunca `err.Error()` de erro embrulhado.
- **F-06** Segredos validados na subida (`SH/config/config.go`): `JWT_SECRET` ≥ 32 bytes e sem placeholder, `PASSWORD_PEPPER` ≥ 32 bytes, `DB_USUARIO`/`DB_SENHA`, `JWT_TTL > 0`; mensagens nunca trazem valores (SEC-18).
- **F-07** DSN com `clientFoundRows=true`; a condição fica no `WHERE`, nunca deduzida por `RowsAffected`.
- **F-08** DSN nunca logado: `maskDSN` segue o parse do driver e, se a senha ainda aparecer, sai só `****` (SEC-15).
- **F-09** vlog desligado por padrão, regras em `docs/logs-verbose.md`; o log de acesso não registra query string.
- **F-10** Paginação: limit padrão 20, máximo 100; `page` entre 1 e `kernel.PaginaMaxima` (10.000) (SEC-16); dashboard `dias` 1..365.

## Catálogo (C)

- **C-01** Produto: leitura comum, escrita admin; estoque só admin.
- **C-02** Ordenação por enum tipado mapeado para coluna por switch; o repositório nunca recebe string livre (cobertura em `sec24_*`, SEC-24).
- **C-03** Busca com `LIKE ? ESCAPE '!'` via `db.TermoContem` e `q` com até 100 caracteres (SEC-25).

## Identidade e acesso (I)

- **I-01** JWT só HS256, com `iss`, `exp` e `iat` obrigatórios; `iat` no futuro recusado (SEC-18).
- **I-02** Bearer tem precedência; header malformado em rota protegida → 401 sem cair para o cookie `access_token`.
- **I-03** SEC-06: usuário consultado no banco a cada request protegido; inexistente/inativo → 401; erro → 500; vale a role do banco; JWT inválido → 401 sem consultar o banco.
- **I-04** SEC-08: `iat <= tokens_validos_desde` → 401; corte gravado em troca/reset de senha, inativação, desligamento de vendedor e reuso; o rehash não mexe no corte.
- **I-05** RBAC pela role do banco (`admin`, `normal`, `usuario`); fora do admin, escopo restrito. `deve_trocar_senha` imposto: 403 "troca de senha obrigatória", exceto `POST /api/auth/reset-password` e `GET /api/auth/me` (SEC-23).
- **I-06** Refresh: 32 bytes de `crypto/rand`, banco guarda SHA-256, TTL 7d; rotação atômica na mesma transação; expiração checada antes da revogação; janela de graça 30 s.
- **I-07** Reuso (SEC-07/12): `rotacao` fora da janela → marca atômica, revogação em massa, corte e alerta; erro ao marcar corta (fail-closed); corte incompleto desfaz a marca; resposta HTTP idêntica.
- **I-08** Alerta SEC-09 assíncrono: dedup 30 min por usuário, teto 20/h, 4 concorrentes, timeout 30 s; lista de admins sem CR/LF.
- **I-09** Revogações: logout idempotente limpa cookies; troca de senha e reset pelo admin: corte + revogação total; desligamento de vendedor numa transação única.
- **I-10** Limpeza de refresh: retenção mínima 24h, lotes de 1000, cancelável, parada antes de fechar o pool.
- **I-11** Hash SEC-13: Argon2id sobre HMAC-SHA256(pepper, senha), salt 16 B, comparação em tempo constante; bcrypt legado só verifica e marca rehash; hash descartável para e-mail inexistente ou hash malformado (SEC-32).
- **I-12** Senha: 8 runes a 72 bytes, 3 de 4 classes. Reset: captcha → rate limit → senha atual → nova ≠ atual → força → histórico (atual + 2 últimas, sem short-circuit) → grava → revoga.
- **I-13** Turnstile em login e reset, antes do rate limit, fail-closed (400); confere `hostname` (`TURNSTILE_HOSTNAMES` ou hosts do CORS) e `action` (`login`, `reset-senha`) (SEC-19); recusa logada com o motivo (LOG-03); secret de teste não sobe com `COOKIE_SECURE=true`, salvo `TURNSTILE_PERMITIR_SECRET_TESTE=true` (SEC-37).
- **I-14** Rate limit em memória: login `ip:` e `acct:` 5/min (bloqueio 5 min), refresh 10/min, reset 10/5 min (bloqueio 10 min); 429 + `Retry-After`. IP: item mais à direita do XFF só com `TRUST_PROXY_HEADERS=true`; `X-Real-IP` ignorado (SEC-20).
- **I-15** Cookies HttpOnly (`access_token` SameSite=Strict, `refresh_token` Lax 7d), `Secure` por `COOKIE_SECURE` (padrão true), limpeza com os mesmos atributos (SEC-21); tokens nunca no JSON.
- **I-16** Anti-enumeração: inexistente, senha errada e inativo → mesmo 401 "credenciais inválidas" com o mesmo custo de hash (SEC-22).
- Domínio: VO de senha/token com `String()` = `***` e sem JSON; rate limiters e notifier como singletons na composition root.

## Comercial (E)

- **E-01** Escopo resolvido a cada request: admin irrestrito; outros restritos; sem vínculo → sem acesso; vendedor desligado → 403 "acesso bloqueado: vendedor desligado"; erro → 500.
- **E-02** Escopo é tipo soma `Irrestrito | Vendedor(id>0) | SemAcesso`, sem zero value útil; restrito com id ≤ 0 é erro. O repositório recebe `Escopo`, nunca int64.
- **E-03** Listagem restrita ignora o vendedor da query; sem acesso → 200 com lista vazia, sem tocar o banco.
- **E-04** Fora da carteira → 404 com a mesma mensagem de inexistente (nunca 403).
- **E-05** Escrita: create força o vendedor do escopo e exige cliente na carteira ativa; update/delete checam a posse atual; cliente autoriza antes de decodificar o corpo.
- **E-06** Vendedor normal vê só o próprio. **E-07** CNPJ duplicado → 409 genérico, sem expor id ou vendedor.

## Vendas (V)

- **V-01** Pedido segue E-03..E-05; exclusão → 409 com pagamentos ou status Faturado; PUT/DELETE sob lock (SEC-26).
- **V-02** Pagamento herda o escopo do pedido; `pedido_id` não editável; alheio ou inexistente → 404, infraestrutura → 500 (SEC-38). Criação e exclusão em transação com lock pedido → pagamento (SEC-35); FK 1452 → 404.
- **V-03** `valor_bruto` e `valor_total` calculados no backend.

## Dashboard (D)

- **D-01** Escopo próprio: desligado, inexistente ou sem vendedor → 200 zerado com `vendedor_desligado`; nunca zerar o vendedor e seguir.
- **D-02** `fmt.Sprintf` no SQL só com constantes; período por enum validado; valores por placeholder. **D-03** `dias` 1..365.

## SMTP, importadores e CLIs (M)

- **M-01** SMTP: TLS implícito ou STARTTLS obrigatório, AUTH só sobre TLS, verificação de certificado, TLS ≥ 1.2; destinatário sem CR/LF; config inválida derruba a subida.
- **M-02** Senha gerada nunca em resposta ou log; o noop loga e-mail mascarado.
- **M-03** `LoadSemCredenciaisDB` só em dry-run; `resetsenhas` simula por padrão (`-executar`), grava histórico e revoga tokens. **M-04** Importadores com placeholders; CNPJ nunca logado.
