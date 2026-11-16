# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| DEPLOY-10 | ALTA | migrações 27, 28/29, 30 e 31 + `APP_URL` e `GOOGLE_CLIENT_ID` na produção antes do deploy | 🌸 DataBrain |
| ECOM-06 | MÉDIA | pendente | 🟡 BackBrain / 🟢 FrontBrain |
| SEC-60 | MÉDIA | decisão do usuário | 🟣 SecBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## DEPLOY-10: migrações 27 e 28/29 (loja) na produção — ALTA
**Ação:** CasaOS :3307 já aplicado (27, 28/29). Na produção: `make db-fix-role-usuario`, `make db-fix-confirmacao-email` `make db-fix-clientes-loja` e `make db-fix-clientes-numero`, com `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>`, **antes** de subir a API do ECOM-01. Google: criar o Client ID com a origem https de produção, pôr `GOOGLE_CLIENT_ID` no `api.env` e no build do frontend (compose já lê; falta o parâmetro no `Jenkinsfile`/`setup-servidor.sh`) (sem ela, o cadastro falha no INSERT).

## ECOM-06: cliente pessoa física no CRM e pedidos da loja — MÉDIA
**Ação:** a edição de cliente no admin ainda exige CNPJ (um cliente da loja não salva por lá); mostrar CPF/endereço e permitir editar PF. Para o checkout: pedidos exigem vendedor e canal (`App/Telefone/Visita/WhatsApp`) → criar vendedor "Loja Online" e canal `Loja`.

## SEC-60: PII do cliente da loja na conta `casaos_ro` — MÉDIA
**Ação:** `clientes` está em `TABELAS_RO` e agora guarda CPF e endereço de pessoas físicas. Decidir se a `casaos_ro` perde o SELECT em `clientes` (ou ganha uma view sem `cpf`/`endereco`/`cep`).

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-15):** reconferido; upstream sem correção (`typescript-eslint` 8.72.2 vulnerável, plugins sem `eslint` 10). Patches aplicados: tailwind 4.3.5, postcss 8.5.33, user-event 14.6.10. 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (o 9 avisa "no longer supported"), até os plugins react/import/jsx-a11y aceitarem o 10.
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 15.5 (fora da linha do `next` 16); o `typescript-eslint` mais novo (8.72.2) ainda é vulnerável.
