# Roteiro de teste: smoke geral das telas

**Escopo:** passada rápida em todas as telas (carga, paginação, ordenação, filtros, modais e respostas fora de ordem), antes de um deploy ou depois de mudanças amplas no frontend.
**Pré-requisitos:** `make dev-api`, `make dev-frontend`, banco de dev com dados; logado como admin (e como Rafael onde indicado). Contas em `docs/roteiro-teste-sessao-autenticacao.md`, seção 0.
**Tempo estimado:** 15 a 20 minutos.

## 1. Checagem padrão (em cada tela, com DevTools > Network aberto)

| Passo | Ação | Esperado |
| --- | --- | --- |
| 1.1 | Abrir | Spinner e depois a lista, sem erro vermelho. |
| 1.2 | ">" e "<" | Troca de página com spinner; "Pagina N de M" acompanha. |
| 1.3 | Clicar num cabeçalho, de novo | Ordena; o segundo clique inverte a seta (ordenação feita pela API). |
| 1.4 | Filtrar estando na página 2 | Uma requisição com `page=1`; paginador volta para 1. |
| 1.5 | Modal "Novo": digitar, fechar, reabrir | Abre vazio de novo. |
| 1.6 | "Editar" e depois "Novo" | Editar abre preenchido; "Novo" não traz nada do registro anterior. |
| 1.7 | Com "Slow 3G", clicar ">" três vezes rápido; depois ">" e "<" | A tabela mostra a mesma página do paginador (vale a última resposta). |
| 1.8 | Clicar ">" logo depois de abrir | Fica na página 2. |

> Em `next dev` (`reactStrictMode`) cada listagem pode fazer 2 `GET` idênticos ao abrir; num build (`next build && next start`) é 1. Nenhum `GET` extra depois de 1 s.

## 2. Telas

| # | Tela | URL | Além da seção 1, conferir |
| --- | --- | --- | --- |
| 1 | Pedidos | `/admin/pedidos` | "Itens"/"Ocultar itens"; cascata vendedor → cliente no modal. Detalhes em `docs/roteiro-teste-pedidos-pagamentos.md`. |
| 2 | Pagamentos | `/pagamentos` | Busca de pedido no modal com pausa na digitação; reabrir vem vazio. |
| 3 | Clientes | `/admin/clientes` | Ver `docs/roteiro-teste-clientes.md`. |
| 4 | Oportunidades | `/admin/oportunidades` | Cascata vendedor → cliente; "Fechado perdido" exige o motivo. Como Rafael, sem filtro de vendedor e só a carteira dele. |
| 5 | Visitas | `/admin/visitas` | Mesmas checagens do item 4. |
| 6 | Estoque | `/admin/estoque` | "+ Novo registro" para o admin (papel vindo do `/api/auth/me`); editar altera só o saldo. |
| 7 | Produtos | `/admin/produtos` | Inativar/reativar; imagem em `docs/roteiro-teste-produtos-imagens.md`. |
| 8 | Usuários | `/admin/usuarios` | Mudar "Perfil" na página 2 volta à 1 com um único `GET /api/usuarios`; na página 1 o filtro é local (sem requisição). |
| 9 | Vendedores | `/admin/vendedores` | Lista carrega uma vez (paginação local); Editar abre com admissão e meta do banco e mostra "Clientes vinculados"; "Novo" não traz a lista anterior. |
| 10 | Auditoria de Senha | `/admin/senha-historico` | "Tipo de Reset" na página 2 volta à 1 com uma requisição; Usuario, Resetado Por e IP Origem ordenam pela API (`order_by`); `created_at` nulo mostra `-`. |
| 11 | Dashboard | `/dashboard` | Hoje/Semana/Mês, select de dias e janela do funil recarregam. Ver `docs/roteiro-teste-dashboard.md`. |
| 12 | Loja | `/`, `/produtos/{id}`, `/checkout`, `/minhas-compras` | Vitrine com filtros e imagens; carrinho e checkout; selo do ambiente no rodapé em dev/qa. Ver a seção 5 de pedidos-pagamentos. |

## 3. Transversais

| Passo | Ação | Esperado |
| --- | --- | --- |
| 3.1 | Datas em todas as telas | `dd/mm/aaaa` (hora onde houver), iguais às do banco; vazio é `-`, nunca "Invalid Date". |
| 3.2 | Criar com a recarga da lista bloqueada (ex.: bloquear `localhost:8080/api/produtos?*`) | `POST` 201 e um único alerta "<sucesso> Porem, nao foi possivel recarregar a lista (...). Os dados exibidos podem estar desatualizados.", que não some sozinho. |
| 3.3 | API parada | Telas mostram só o alerta de conexão (detalhes no roteiro de sessão, seção 7). |

Cobertura automatizada: `make test-frontend` (listagens, modais, cascata e `useResetOnOpen` em `frontend/tests/`).
