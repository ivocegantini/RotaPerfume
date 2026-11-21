# Roteiro de teste: sessão e autenticação

**Escopo:** login, refresh, revogação, corte de sessão, troca/reset de senha, reuso de token, alertas, vendedor desligado e falhas de rede.
**Pré-requisitos:** banco de dev (`make db-reset` recria; `make db-migrar` atualiza), `make dev-api` (cookies `access_token`/`refresh_token`, sem `__Host-`) e `make dev-frontend`. Contas na seção 0.
**Tempo estimado:** 60 a 80 minutos (seções independentes).

---

## 0. Contas de teste (usadas por todos os roteiros)

| Perfil | E-mail | id | Vendedor |
| --- | --- | --- | --- |
| admin | `admin@rotaperfumes.com.br` | 1 | - |
| normal com vendedor | `rafael.carvalho@rotaperfumes.com.br` | 5 | 4 - Rafael Carvalho (meta 55.000,00) |
| normal com vendedor desligado | `thiago.silva@rotaperfumes.com.br` | 4 | 3 - Thiago Silva (desligado em 2025-07-26) |
| normal sem vendedor / QA | `qa.<nome>@rotaperfumes.test` | criar | - |

- **Senhas:** o `make db-seed` usa `SEED_ADMIN_PASSWORD`/`SEED_USER_PASSWORD` do `backend/.env` (ou imprime uma aleatória). Todos nascem com `deve_trocar_senha = 1`: o 1º login leva a `/trocar-senha`.
- **Desligado:** o `sql/seed_vendedores.sql` grava `ativo = 0` nos usuários dos vendedores desligados (1, 3, 9, 30, 37, 38). Para testar o desligado **ativo**, só no dev: `UPDATE usuarios SET ativo = 1 WHERE id = 4;` e, no fim, `ativo = 0`.
- **Usuário QA** (em `backend/shared`): `go run ./cmd/resetpassword -email qa.x@rotaperfumes.test -password-prompt -role normal -nome "QA X"` (Git Bash: `winpty`). Sem vendedor por padrão.
- **Token no terminal:** copie o cookie em DevTools > Application > Cookies. O login pela API exige o Turnstile; prefira logar no navegador.
- **Rate limit do refresh:** 10 falhas por IP em 1 minuto, bloqueio de 5 min (429 com `Retry-After`). Se travar, espere ou reinicie a API.

## 1. Login, troca obrigatória e `/me`

| Passo | Ação | Esperado |
| --- | --- | --- |
| 1.1 | Login com usuário de `deve_trocar_senha = 1` | Vai para `/trocar-senha`. Qualquer rota protegida responde 403 `"troca de senha obrigatória"`, exceto `GET /api/auth/me` e `POST /api/auth/reset-password`. |
| 1.2 | Trocar a senha em `/trocar-senha` | "Senha alterada com sucesso!", os cookies caem e o destino final é `/login`; entrar com a senha nova funciona. |
| 1.3 | Senha errada, usuário inativo ou inexistente | 401 `"credenciais inválidas"` nos três casos (mesma resposta). |
| 1.4 | Login como normal ativo | Cai em `/pagamentos`. Menu com ERP (Clientes, Oportunidades, Visitas) e CRM (Pagamentos, Pedidos); sem Administração. |
| 1.5 | `GET /api/auth/me` | 200 com `id`, `role`, `id_vendedor`; o `auth_user` do Local Storage não guarda `vendedor_desligado`. |

## 2. Refresh e corrida entre abas

| Passo | Ação | Esperado |
| --- | --- | --- |
| 2.1 | Apagar só o `access_token` e clicar ">" numa listagem | `GET` 401 → `POST /api/auth/refresh` 200 → `GET` repetido 200, sem ir para `/login`. Vale em todas as listagens e no Dashboard. |
| 2.2 | Duas a cinco abas, apagar o `access_token` e paginar em todas em até 2 s | Nenhuma vai para `/login`, nenhum 429, nenhum `POST /api/auth/logout`. Refresh 401 de uma aba é seguido do `GET` repetido 200. |
| 2.3 | 15 refresh em paralelo com o mesmo token (laço abaixo) | Exatamente um 200 e catorze 401 `"refresh token revogado"`, nenhum 429; só um par novo no banco. |
| 2.4 | Repetir o mesmo token 12 vezes em menos de 30 s | 12 × 401, nenhum 429, nenhum `Set-Cookie` (janela de graça não conta no rate limit). |
| 2.5 | Apagar os dois cookies e clicar ">" | Refresh 400 `"refresh_token é obrigatório (body ou cookie)"`, logout e `/login`, sem repetir o `GET`. |

```bash
RT='<refresh_token>'
for i in $(seq 1 15); do curl -s -o /dev/null -w "req $i -> %{http_code}\n" -X POST http://localhost:8080/api/auth/refresh \
  -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$RT\"}" & done; wait
```

## 3. Reuso de token rotacionado (roubo) e alertas

Use um usuário QA. Para os e-mails, preencha `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` e `SECURITY_ALERT_EMAILS`; sem SMTP, o log diz que os alertas ficam só no log.

| Passo | Ação | Esperado |
| --- | --- | --- |
| 3.1 | Login, copiar o `refresh_token` como `RT1`, um refresh com ele (200) | `RT1` passa a `revoked_reason = 'rotacao'`. |
| 3.2 | Esperar 35 s e reusar o `RT1` | 401 com o mesmo corpo de sempre. Log `[auth][seguranca] ... revogando todas as sessões`. Tokens do usuário viram `revogacao_massa` e `tokens_validos_desde` é gravado. |
| 3.3 | Caixa do usuário | 1 e-mail com horário de Brasília, IP e navegador, sem `user_id`/`token_id`; cada `SECURITY_ALERT_EMAILS` recebe 1 cópia com eles. O log mascara o e-mail como `[email-usuario]`. |
| 3.4 | Novo login (2 s depois) e reusar o `RT1` de novo em menos de 30 min | 401 igual, log "reuso repetido ... sessões NÃO revogadas de novo"; a sessão nova segue válida, sem e-mail novo. |
| 3.5 | Controle: login, copiar `RT2`, clicar **Sair**, esperar 35 s e reusar | 401, log só "token revogado apresentado fora da janela de graça ... motivo=logout", sem `[auth][seguranca]` e sem e-mail. |
| 3.6 | Reuso fora da janela 11 vezes seguidas | As 10 primeiras 401 com corpo idêntico; a 11ª 429 com `Retry-After`. |
| 3.7 | `SELECT revoked_reason, COUNT(*) FROM refresh_tokens WHERE usuario_id = <QA> GROUP BY 1;` | Motivos possíveis: `rotacao`, `logout`, `revogacao_massa`, `senha`, `inativacao`. |

## 4. Corte de sessão (inativação, role, troca e reset de senha)

Janela A = admin; janela B (anônima) = QA. Teste o token direto com `curl -s -w ' <- %{http_code}\n' http://localhost:8080/api/auth/me -H "Authorization: Bearer $AT"`.

| Passo | Ação | Esperado |
| --- | --- | --- |
| 4.1 | QA criado como admin; em A, trocar a role dele para normal; em B, recarregar `/admin/usuarios` | `GET /api/usuarios` 403 sem novo login (a role vem do banco). Voltar para admin → 200. |
| 4.2 | Em A, inativar o QA; em B, qualquer ação | 401 `"usuário inativo"`, refresh 401, logout e `/login`; tokens com `inativacao`. |
| 4.3 | Em A, reativar o QA; `curl` com o token antigo | 401 `"sessão encerrada — faça login novamente"`; log "token anterior ao corte de sessão". Novo login funciona. |
| 4.4 | QA troca a própria senha em `/trocar-senha` | Token antigo de outra aba → 401 `"sessão encerrada ..."`; login com a senha nova → 200. |
| 4.5 | Admin usa **Resetar senha** no QA | Token do QA → 401 `"sessão encerrada ..."`; token do admin segue 200; a senha nova vai por e-mail. |

> Login no mesmo segundo do corte gera token já recusado (`iat <= corte`); espere 1 s e logue de novo.

## 5. Vendedor desligado

Login como `thiago.silva` (reativado e com a senha já trocada; o 403 de troca vem antes do de desligado).

| Passo | Ação | Esperado |
| --- | --- | --- |
| 5.1 | Menu | Sem ERP e CRM; ficam Dashboard, Trocar Senha, nome, papel e Sair. |
| 5.2 | Abrir `/admin/clientes`, `/admin/pedidos`, `/pagamentos`, `/admin/oportunidades`, `/admin/visitas` | Só o título, o aviso "Seu vendedor foi desligado; o acesso aos dados da carteira está bloqueado." e "Ir para o Dashboard". Sem loop de `/me`, sem refresh/logout. |
| 5.3 | No Console, `fetch` em `/api/clientes`, `/api/pedidos`, `/api/pagamentos`, `/api/oportunidades`, `/api/visitas`, `/api/vendedores/3/clientes` | Todos 403 `"acesso bloqueado: vendedor desligado"` (texto exato). `/api/auth/me` 200 com `"vendedor_desligado": true`. |
| 5.4 | `/dashboard` e `/trocar-senha` | Abrem; Dashboard com aviso e números zerados (ver `docs/roteiro-teste-dashboard.md`). |
| 5.5 | Bloqueio sem novo login: Rafael logado; `UPDATE vendedores SET data_desligamento = CURDATE() WHERE id = 4;`; trocar de aba e voltar | Menu esconde ERP/CRM e a tela vira o aviso. Clicar "Buscar" sem trocar de aba dá 403 e o aviso na hora. Desfaça com `= NULL` e volte o foco: tudo normal. |
| 5.6 | Desligar o vendedor pela tela de Vendedores (`DELETE /api/vendedores/{id}`) | Os usuários dele ficam `ativo = 0` e não logam mais; reativar o vendedor não reativa o usuário. Por isso o 5.5 usa SQL. |
| 5.7 | Admin com `id_vendedor` de vendedor desligado | Nunca é bloqueado nem zerado. |

## 6. Vínculo alterado pelo admin com o usuário logado

| Passo | Ação | Esperado |
| --- | --- | --- |
| 6.1 | Rafael em `/admin/pedidos`; admin troca o vendedor do usuário 5 para o 7; Rafael volta o foco | Novo Pedido mostra o vendedor #7 travado e a carteira do 7 (`GET /api/vendedores/7/clientes`), sem novo login; o mesmo com o modal já aberto, em Oportunidades e Visitas. |
| 6.2 | Vínculo para nenhum vendedor | Modais mostram o aviso de sem vendedor e o salvar desabilitado; Dashboard zerado com aviso. |
| 6.3 | Vínculo para um vendedor desligado (ex.: 3) | Comportamento da seção 5; o `ativo` do usuário não muda. Restaure o vínculo para o 4. |

## 7. Falhas de rede e erro de carga

| Passo | Ação | Esperado |
| --- | --- | --- |
| 7.1 | Bloquear `localhost:8080/api/auth/refresh` (DevTools > Block request URL), apagar o `access_token`, trocar o período do Dashboard | Refresh `(blocked)`, sem logout; mensagem "Não foi possível conectar ao servidor. Verifique sua conexão com a internet e tente novamente.". Desbloquear → refresh 200 e tudo carrega. |
| 7.2 | Editar o `refresh_token` para `invalido` e apagar o `access_token` | Refresh 401, logout, `/login`; o `auth_user` some ("Sessão expirada. Faça login novamente."). 429 e 500 no refresh também deslogam. |
| 7.3 | DevTools offline e clicar "Semana" no Dashboard | Mensagem de conexão, sem logout; online de novo, carrega. |
| 7.4 | Parar a API e abrir cada listagem por F5 | Só o alerta de conexão: sem "Nenhum ... cadastrado" e sem o contador do cabeçalho. |
| 7.5 | Com a página 1 carregada, parar a API e clicar ">" | Alerta; tabela, paginador e contador seguem na página 1. Subir a API e clicar ">" carrega a 2. |

## 8. Operação (terminal)

| Passo | Ação | Esperado |
| --- | --- | --- |
| 8.1 | Comentar `DB_SENHA` no `backend/.env` e rodar `make db-create` ou `make dev-api` | `make` para com "SEC-11: defina DB_USUARIO/DB_SENHA no .env" sem chamar o `mariadb`; a API não sobe e não mostra valores. `make help` e `make lint` funcionam. |
| 8.2 | `make -n db-up DB_SENHA=FICTICIA \| grep -c FICTICIA` | `0`: a senha não aparece no eco (vai por `MYSQL_PWD`). |
| 8.3 | `REFRESH_CLEANUP_INTERVAL=30s` (ou `REFRESH_TOKEN_RETENCAO=1h`) e subir a API | Não sobe (`0` desliga, mínimo 1m; retenção de 24h a 8760h). Padrões: 6h e 720h. |
| 8.4 | Subir a API normal e depois Ctrl+C | Log `[refresh] cleanup: limpeza periódica iniciada` e `N tokens removidos (corte=...)`; no fim, `shutdown solicitado` → `limpeza periódica encerrada` → `tarefas de fundo encerradas` → `bye`. |
| 8.5 | `resetpassword` com `-password-prompt`, `-password-stdin` e `-password` | Prompt sem eco e com confirmação; stdin funciona; `-password` funciona com aviso de depreciação; duas fontes juntas → erro. |
| 8.6 | `make gen-hash` | `git status sql/` sem mudanças e sem sobra de `tmp/seed`; sem `SEED_*_PASSWORD`, gera senha aleatória (as senhas do seed mudam). |

## 9. Limpeza

- Apague os usuários QA (`DELETE FROM usuarios WHERE email LIKE 'qa.%@rotaperfumes.test';`; tokens e histórico saem em cascata).
- Restaure `data_desligamento` do vendedor 4, o vínculo do usuário 5 e o `ativo = 0` do usuário 4; restaure o `backend/.env`.
- No DevTools: desbloqueie URLs, volte para "No throttling" e faça logout/login para limpar cookies editados.
