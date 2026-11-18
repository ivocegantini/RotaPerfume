# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| IMG-02 | ALTA | o site não mostra nenhuma imagem de produto (volume vazio em produção) | 🟡 BackBrain |
| LOTE-43 | MÉDIA | BACKUP-03/04/05/07, SEC-73, TST-28 (`tarefas/lote-43-spec.md`) | 🟣 SecBrain |
| DEPLOY-11 | BAIXA | fase 2 do `--dev-ip`: recusar a opção e remover a transição | 🟡 BackBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## DEPLOY-11: fase 2 do `--dev-ip` — BAIXA
**Contexto:** a porta 3306 foi fechada no servidor em 2026-11-18 (`--recriar` sem `--dev-ip`); a transição acabou.
**Ação:** recusar o `--dev-ip` no `mariadb-docker.sh` (como o `--rede-lan`) e remover o código e a unit de firewall da transição; atualizar `docs/deploy-servidor.md` §3.3 e os testes.

## IMG-02: site em produção sem imagens de produto — ALTA
**Sintoma (2026-11-18):** nenhuma imagem aparece no site; o backup acusou o volume `rotaperfumes_produto_imagens` vazio (0 arquivos), e já estava vazio antes do `--recriar` do banco.
**Causa (confirmada):** 292 produtos, 0 com `produtos.imagem`: as imagens nunca foram geradas em produção (só no dev, IMG-01). Nada se perdeu. Volume único `rotaperfumes_produto_imagens`.
**Restrição:** a 3306 está fechada (DEPLOY-10) e a imagem da API só traz o `server`: o `make db-gerar-imagens-produtos` não alcança a produção.
**Opções (decisão do usuário):** (a) `deploy/gerar-imagens-producao.sh`: compila o `gerarimagens` num container (como o `seed-producao.sh`), roda na rede `rotaperfumes-db` com o volume (uid 65532) e a conta admin .20 via TLS, com confirmação e backup antes; (b) fotos reais pelo upload do admin.

## LOTE-43: spec da `lote-42` ainda não implementada — MÉDIA
**Ação:** implementar conforme `tarefas/lote-43-spec.md`: BACKUP-03 (tolerância de relógio 300 s), BACKUP-04 (`parcial-*` na guarda), BACKUP-05 (retenção relê os pares), BACKUP-07 (`--trocar-dados`), SEC-73 (fingerprint da chave apt da Cloudflare por 2º canal), TST-28 (`-timeout=30m`). Depois do MERGE-01.

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-15):** reconferido; upstream sem correção (`typescript-eslint` 8.72.2 vulnerável, plugins sem `eslint` 10). Patches aplicados: tailwind 4.3.5, postcss 8.5.33, user-event 14.6.10. 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (o 9 avisa "no longer supported"), até os plugins react/import/jsx-a11y aceitarem o 10.
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 15.5 (fora da linha do `next` 16); o `typescript-eslint` mais novo (8.72.2) ainda é vulnerável.
