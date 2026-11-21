# Roteiro de teste: Dashboard

**Escopo:** `/dashboard` (KPIs, gráfico, metas, ranking/desempenho, clientes e funil) por perfil: admin, normal com vendedor, sem vendedor e desligado.
**Pré-requisitos:** `make dev-api`, `make dev-frontend`, banco de dev com pedidos, carteiras e oportunidades (`make db-seed-mes-atual` gera movimento no mês corrente). Contas em `docs/roteiro-teste-sessao-autenticacao.md`, seção 0.
**Tempo estimado:** 30 a 40 minutos.

Os números mudam com o tempo: confira sempre com as queries (exemplo do vendedor 4, período "Mês"):

```sql
SELECT COUNT(*) pedidos, SUM(valor_total) total FROM pedidos WHERE vendedor_id = 4 AND status <> 'Cancelado'
   AND YEAR(data_pedido) = YEAR(CURDATE()) AND MONTH(data_pedido) = MONTH(CURDATE());
SELECT meta_mensal FROM vendedores WHERE id = 4;
SELECT COUNT(*) total, SUM(ativo) ativos FROM clientes WHERE cliente_id_origem IN
  (SELECT cliente_id FROM carteiras WHERE vendedor_id = 4 AND data_fim IS NULL);
```

## 1. Normal com vendedor (Rafael, vendedor 4)

| Passo | Conferir | Esperado |
| --- | --- | --- |
| 1.1 | Cabeçalho e KPIs | "Suas metricas de vendas"; "Minhas Vendas", "Meus Pedidos", "Meu Ticket Medio", "Minha Meta" (com "Atingido: N%"); sem "Ranking Vendedores". |
| 1.2 | Valores do mês | Batem com a query do vendedor 4, não com o total da empresa; meta = só a dele. |
| 1.3 | Gráfico | "Minhas Vendas nos Últimos N Dias", N acompanha o select 7/14/30/60; dias sem venda com barra zerada. |
| 1.4 | Metas e desempenho | Card "Minha Meta" só com ele; "Meu Desempenho" ("Seus indicadores no ranking de vendas") com total, pedidos, ticket, meta e atingimento. |
| 1.5 | Clientes | "Clientes da minha carteira" / "Somente clientes vinculados ao seu vendedor"; total e ativos batem com a query; "por Segmento" e "por UF" somam o mesmo total. |
| 1.6 | Hoje / Semana / Mês | Recarregam os números; sem aviso amarelo. |

## 2. Admin

| Passo | Conferir | Esperado |
| --- | --- | --- |
| 2.1 | Cabeçalho e KPIs | "Visao geral das metricas de vendas"; "Total de Vendas", "Pedidos", "Ticket Medio", "Ranking Vendedores" ("Lider: <nome>"). |
| 2.2 | Valores | Empresa inteira; meta mensal = soma das metas dos vendedores ativos. |
| 2.3 | Ranking e metas | "Ranking de Vendedores" ("Top 10 por volume de vendas no periodo") com 10 linhas; "Acompanhamento de Metas" com vários vendedores. |
| 2.4 | Clientes | Rótulos globais ("Total de Clientes"), igual a `SELECT COUNT(*) FROM clientes`; sem "Clientes da minha carteira". |
| 2.5 | Gráfico | "Vendas nos Últimos 30 Dias" por padrão; muda com o select (`GET /api/dashboard/vendas?dias=N`). |
| 2.6 | Centavos | Valores com centavos no ranking e nas metas; percentual sobre o valor com centavos (ex.: 74.558,44 / 100.000 = 74,56%). |

## 3. Sem vendedor e vendedor desligado

| Passo | Perfil | Esperado |
| --- | --- | --- |
| 3.1 | QA sem vendedor | Aviso "Usuario sem vendedor vinculado. As metricas de vendas sao exibidas apenas para usuarios vinculados a um vendedor; ...", acima dos cards. |
| 3.2 | Desligado (Thiago) | Aviso "Vendedor desligado. O vendedor vinculado ao seu usuario possui data de desligamento, ..."; o de sem vendedor não aparece. |
| 3.3 | Ambos | KPIs visíveis e zerados (R$ 0,00 / 0 / R$ 0,00 / "-" "Sem meta cadastrada"); gráfico "Total: R$ 0,00" ou "Nenhum dado de vendas no periodo"; clientes 0 com "Sem dados disponiveis"; "Nenhum dado de desempenho encontrado". |
| 3.4 | Ambos | Todas as chamadas 200 com zeros e listas `[]` (nunca `null`); nenhum "NaN" nem erro vermelho. Desligado: `metrics` com `"vendedor_desligado": true`, mesmo com clientes em carteira. |
| 3.5 | Admin com `id_vendedor` desligado | Nenhum aviso; números normais. |

## 4. Zeros forçados pelo frontend

Com o DevTools > Network > **Override content** em `metrics?periodo=month`, troque o corpo por um preenchido, mantendo o envelope:

```json
{"success":true,"data":{"total_vendas":99999.99,"total_pedidos":42,"ticket_medio":2380.95,"total_clientes":7,"meta_mes":50000,"atingimento_meta":199.99,"periodo":"month","vendedor_desligado":false,"top_vendedores":[],"metas_vendedores":[]}}
```

| Passo | Perfil | Esperado |
| --- | --- | --- |
| 4.1 | Sem vendedor ou desligado | Tudo zerado; nenhum número do override aparece. |
| 4.2 | Rafael com `"vendedor_desligado":true` | Aviso de desligado e tudo zerado, inclusive clientes. |
| 4.3 | Rafael com `false` (controle) | Os números do override aparecem (R$ 99.999,99, 42 pedidos). |
| 4.4 | Admin com `true` | Continua com os números (admin nunca é zerado). |
| 4.5 | Hoje → Semana → Mês rápido com "Slow 3G" | Fica o último filtro clicado; "Atualizando dados..." some ao terminar. |
| 4.6 | Bloquear `metrics`, recarregar; desbloquear e "Atualizar" | Alerta vermelho; depois some e os números voltam. |

## 5. Funil de vendas

Card entre gráfico/metas e ranking: "Funil de vendas" (admin) ou "Meu funil de vendas" (normal), subtítulo "Oportunidades por etapa (cumulativo)". Janelas: 30 dias, 90 dias, 12 meses, Tudo (padrão).

```sql
SELECT etapa, COUNT(*) qtd, SUM(valor_estimado) valor FROM oportunidades
 WHERE data_abertura >= DATE_SUB(CURDATE(), INTERVAL 89 DAY)   -- 29 = 30d, 364 = 12 meses; sem WHERE = Tudo
 GROUP BY etapa;                                               -- normal: AND vendedor_id = 4
```

| Passo | Ação | Esperado |
| --- | --- | --- |
| 5.1 | Admin, Tudo | "Oportunidades" = `SELECT COUNT(*) FROM oportunidades` (inclui perdidas). |
| 5.2 | Admin, 90 dias | Fechado ganho bate com a query; cada etapa acima soma as seguintes (Prospecção → Qualificação → Proposta enviada → Negociação → Fechado ganho); "Perdidas" = "Fechado perdido". Barras nunca crescem; a 1ª não mostra "% da etapa anterior". |
| 5.3 | Taxa de ganho | ganhas ÷ (ganhas + perdidas), 1 casa e ponto (7 e 13 → "35.0%"). |
| 5.4 | Janela sem oportunidades | "Nenhuma oportunidade na janela selecionada", sem erro; trocar a janela não mexe nos outros blocos. |
| 5.5 | Rafael | Números da query com `vendedor_id = 4`. Sem vendedor/desligado: funil zerado com o aviso da página. |
| 5.6 | Bloquear `funil` e trocar a janela | "Nao foi possivel carregar o funil de vendas. Tente novamente." com "Exibindo os dados de: <janela>."; desbloquear e "Atualizar" recarrega mantendo a janela. |
| 5.7 | `GET /api/dashboard/funil?janela=7d` | 400 `"janela deve ser '30d', '90d', '365d' ou 'all'"`. |

## 6. Limpeza

- Desative os Local Overrides e os bloqueios de URL do DevTools; remova o usuário QA.
