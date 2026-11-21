# Roteiro de teste: clientes, CNPJ e escopo da carteira

**Escopo:** tela `/admin/clientes` (+ `ClienteModal`), escopo por perfil, CNPJ (formato, DV, duplicidade, alfanumérico), `GET /api/vendedores` e `PATCH .../inativar`.
**Pré-requisitos:** `make dev-api`, `make dev-frontend`, banco de dev com clientes e carteiras. Contas e usuário QA sem vendedor em `docs/roteiro-teste-sessao-autenticacao.md`, seção 0.
**Tempo estimado:** 40 a 50 minutos.

Anote dois clientes: ID_DENTRO (carteira ativa do vendedor 4) e ID_FORA (fora dela):

```sql
SELECT c.cliente_id_origem FROM clientes c JOIN carteiras k ON k.cliente_id = c.cliente_id_origem
 WHERE k.vendedor_id = 4 AND k.data_fim IS NULL LIMIT 1;
SELECT cliente_id_origem FROM clientes WHERE cliente_id_origem NOT IN
 (SELECT cliente_id FROM carteiras WHERE vendedor_id = 4 AND data_fim IS NULL) LIMIT 1;
```

## 1. Contrato por perfil

| Perfil | Criar | Editar/inativar na carteira | Fora da carteira |
| --- | --- | --- | --- |
| admin | 201, sem vínculo de carteira | 200 | 200 |
| normal com vendedor | 201, já na carteira do vendedor | 200 | 404 `"cliente não encontrado"` |
| normal sem vendedor | 403 `"usuário sem vendedor vinculado"` | lista vazia | 404 |
| normal com vendedor desligado | 403 `"acesso bloqueado: vendedor desligado"` | 403 | 403 |

## 2. Tela por perfil

| Passo | Ação | Esperado |
| --- | --- | --- |
| 2.1 | Admin e Rafael: botão "+ Novo Cliente" | Habilitado, sem texto de motivo. Enquanto o `/me` carrega, a tela mostra só "Verificando autenticacao..."; o motivo errado nunca aparece. |
| 2.2 | QA sem vendedor | Botão desabilitado com "Usuario sem vendedor vinculado: solicite o vinculo a um administrador."; lista "Nenhum cliente cadastrado.". |
| 2.3 | Desligado | Só o aviso de vendedor desligado (ver roteiro de sessão, seção 5). |
| 2.4 | Rafael cria cliente (razão, CNPJ válido, segmento, cidade, UF) | "Cliente "<razão>" criado com sucesso.", modal fecha, novo `GET /api/clientes` e o cliente aparece na busca. No banco, `carteiras` com `vendedor_id = 4`, `data_inicio` = hoje, `data_fim` NULL. |
| 2.5 | Reabrir "+ Novo Cliente" | Vazio, com a data de cadastro de hoje. |
| 2.6 | Editar ID_DENTRO (só o bairro) | Abre preenchido; "Cliente "<razão>" atualizado com sucesso." (`PUT` 200). Exige CNPJ com DV válido (seção 4). |
| 2.7 | Status "Ativo" de ID_DENTRO, confirmar; depois reativar | "Cliente inativado com sucesso." (`PATCH` 200). Cancelar a confirmação não faz chamada. |
| 2.8 | Rafael com a tela aberta; admin encerra o vínculo de ID_DENTRO (Vendedores → Remover); Rafael salva a edição | Modal fecha, alerta "cliente não encontrado", lista recarregada sem ele. Igual no botão de status. Restaure o vínculo. |
| 2.9 | Rafael no Console: `PUT`/`PATCH` em ID_FORA | 404 `"cliente não encontrado"`; ID_FORA não muda no banco. |

```js
const API = "http://localhost:8080";
await fetch(`${API}/api/clientes/ID_FORA/inativar`, { method: "PATCH", credentials: "include",
  headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ativo: false }) })
  .then(async r => r.status + " " + await r.text());
```

## 3. Escopo de `GET /api/vendedores` e dos modais

| Passo | Perfil | Esperado |
| --- | --- | --- |
| 3.1 | admin | 200 com todos (bate com `SELECT COUNT(*) FROM vendedores`); filtros e selects de Oportunidades/Visitas com ativos e desligados. |
| 3.2 | Rafael | 200 com 1 item (id 4), campos `id, nome, regiao, uf, data_desligamento`, sem `meta_mensal`; select "Vendedor" travado em "Rafael Carvalho" e clientes de `GET /api/vendedores/4/clientes`. |
| 3.3 | QA sem vendedor | 200 com `"data": []`; modais com aviso, "Sem vendedor vinculado" e salvar desabilitado. |
| 3.4 | Desligado | 403 `"acesso bloqueado: vendedor desligado"`; o modal nem abre. |
| 3.5 | Rafael: `GET /api/vendedores/5/clientes` | 404 `"vendedor não encontrado"`. |

## 4. CNPJ: validação e duplicidade

A API valida formato e DV antes da duplicidade; toda gravação (criar ou editar) exige DV válido. Gere CNPJs válidos num gerador (ex.: 4devs).

| Passo | CNPJ | Esperado |
| --- | --- | --- |
| 4.1 | Válido com máscara (ex.: `11.444.777/0001-61`) | Salva só com dígitos (`11444777000161`). |
| 4.2 | `123`, DV errado, `00.000.000/0000-00`, `11111111111111` | "CNPJ invalido." no modal, sem requisição; pela API, 400 `"cnpj inválido"`. |
| 4.3 | CNPJ válido já cadastrado (de outro vendedor), com e sem máscara, também como admin | 409 `"cnpj já cadastrado"`, sem id, razão social ou vendedor do existente no corpo. |
| 4.4 | Editar um cliente legado com DV inválido mantendo o CNPJ | A tela barra com "CNPJ invalido." (sem `PUT`); o `PUT` direto responde 400 e não grava. |
| 4.5 | Trocar o CNPJ de um cliente pelo de outro com DV válido | 409 genérico. |
| 4.6 | Inativar/reativar um cliente legado com DV inválido | 200: o `/inativar` não valida CNPJ. |
| 4.7 | `SHOW INDEX FROM clientes WHERE Column_name = 'cnpj';` | Só `uq_clientes_cnpj`, `Non_unique = 0`; coluna `char(14)`, `utf8mb4_unicode_ci`. |

> A maioria dos CNPJs importados é fictícia e tem DV inválido; nos casos de 409 use um CNPJ criado no 4.1. Voltar um cliente ao CNPJ legado só por SQL.

## 5. CNPJ alfanumérico

| Passo | Ação | Esperado |
| --- | --- | --- |
| 5.1 | Campo CNPJ do modal | Placeholder `Ex: 12.ABC.345/01DE-35`, ajuda "Aceita letras e numeros; os 2 ultimos caracteres (DV) sao numericos.", teclado de texto. |
| 5.2 | Digitar ou colar `12abc34501de35` | Vira `12.ABC.345/01DE-35`; `#`, `_` e acentos são descartados; letra nas 2 últimas posições é ignorada. |
| 5.3 | Salvar | `POST` com `"cnpj":"12ABC34501DE35"`, 201 com o mesmo valor; tabela mostra a máscara. |
| 5.4 | `12ABC34501DE36` (DV errado) | "CNPJ invalido.", sem requisição. |
| 5.5 | Criar de novo com `12.abc.345/01de-35` | 409 `"cnpj já cadastrado"`. |
| 5.6 | Buscar (admin) `12ABC34501DE35`, `12abc34501de35`, `12.ABC.345/01DE-35`, `12.abc.345` | Encontra em todas; o `q` vai como digitado e a API tira a máscara. Busca por razão social com hífen segue funcionando. |
| 5.7 | Editar para `rp.5lo.te0/0002-22` | 200; tabela mostra `RP.5LO.TE0/0002-22`; igual no modal do vendedor (clientes vinculados). |
| 5.8 | API direta: `RP5LOTE0000141` (201), repetido (409), `12ABC34501DEA5`, `12ABC34501DE3#`, `12ÁBC34501DE35` (400) | Status conforme indicado; no banco `SELECT COUNT(*) FROM clientes WHERE BINARY cnpj <> UPPER(cnpj);` = 0. |

## 6. `PATCH /{recurso}/{id}/inativar` (clientes, produtos, usuários)

| Body | Esperado |
| --- | --- |
| `{"ativo":"false"}`, `{"ativo":1}`, `{"ativo":` , `{"ativo":true} lixo` | 400 `"body JSON inválido"`, estado inalterado. |
| sem body, `null`, `{}` | 200 e alterna o estado. |
| `{"ativo":false}` duas vezes / `{"ativo":true}` | 200, define o valor (não alterna). |
| Normal, cliente fora da carteira, body inválido | 404 (escopo antes do body). |

Requisições prontas na pasta Clientes/Produtos/Usuários da collection `postman/collection.json`.

## 7. Limpeza

- Apague os clientes de teste (as carteiras saem em cascata) e restaure vínculos encerrados (`UPDATE carteiras SET data_fim = NULL WHERE vendedor_id = 4 AND cliente_id = ID_DENTRO AND data_fim = CURDATE();`).
- Restaure estados `ativo` alterados na seção 6 e remova o usuário QA.
