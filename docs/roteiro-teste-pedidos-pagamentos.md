# Roteiro de teste: pedidos, pagamentos e compra na loja

**Escopo:** `PedidoModal` (`/admin/pedidos`), regras de status e exclusão, `/pagamentos` e o fluxo da loja (checkout, simulador de pagamento, "Minhas compras").
**Pré-requisitos:** `make dev-api` com `APP_ENV=dev` (ou `qa`), `make dev-frontend`, banco de dev com pedidos e carteiras. Contas em `docs/roteiro-teste-sessao-autenticacao.md`, seção 0; para a loja, uma conta de cliente criada em `/cadastro` (com o e-mail confirmado).
**Tempo estimado:** 45 a 60 minutos.

Crie os pedidos de teste como **"Em separação"** e sem pagamento: Faturado e Entregue não podem ser excluídos depois.

## 1. Admin: criar e editar pedido

| Passo | Ação | Esperado |
| --- | --- | --- |
| 1.1 | Novo Pedido | Select Vendedor habilitado, 1º item "Selecione um vendedor"; Cliente desabilitado com "Selecione um vendedor primeiro". |
| 1.2 | Escolher o vendedor 4 | "Carregando clientes..." e depois só a carteira ativa dele (`#id - Razão social`). Trocar o vendedor limpa o cliente e recarrega; voltar a "Selecione um vendedor" não trava o select. |
| 1.3 | Validações | Sem cliente: "Cliente e obrigatorio."; quantidade 0: "Quantidade deve ser maior que zero em todos os itens."; desconto 150: "Desconto deve estar entre 0 e 100 em todos os itens.". |
| 1.4 | Salvar com 1 item (qtd 1, preço 10,55), canal App | "Pedido #N criado com sucesso."; total 10,55; a data na lista e no detalhe é a digitada. |
| 1.5 | Editar: trocar o cliente e qtd 2 | Abre preenchido; salva com total 21,10. |
| 1.6 | Editar um pedido do vendedor 4 cujo cliente saiu da carteira (query abaixo) | Cliente aparece como `#id - Razão (fora da carteira)` e continua selecionado; o admin salva sem checagem de carteira. |
| 1.7 | "Itens" na lista | Abre o detalhe; "Ocultar itens" fecha. |

```sql
SELECT p.pedido_id_origem, p.cliente_id, p.status FROM pedidos p WHERE p.vendedor_id = 4
  AND p.cliente_id NOT IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = 4 AND data_fim IS NULL) LIMIT 5;
```

## 2. Regras de status e exclusão

Faturado só vai para Entregue; Entregue é terminal e só vem de Faturado; a baixa de estoque acontece uma vez, ao entrar em Faturado. Anote o saldo do SKU (tela Estoque) antes.

| Passo | Ação | Esperado |
| --- | --- | --- |
| 2.1 | Novo Pedido: select Status | "Em separação", "Faturado" e "Cancelado"; sem "Entregue" e sem "Aguardando pagamento". |
| 2.2 | Criar já como Faturado | Saldo do SKU cai na quantidade do item, uma vez. |
| 2.3 | Editar Em separação/Cancelado | Sem "Entregue"; `PUT` para Entregue → 400 `"pedido só pode ser marcado como Entregue depois de Faturado"`. |
| 2.4 | Editar Faturado | Só "Faturado" e "Entregue", ajuda "Pedido faturado so pode ser alterado para Entregue.", itens desabilitados. Salvar como Entregue não baixa de novo. |
| 2.5 | Editar Entregue | Status desabilitado ("Pedido entregue nao pode ter o status alterado."), itens desabilitados; salvar sem mudar funciona. |
| 2.6 | Lista: Excluir em Faturado/Entregue | Botão desabilitado ("Pedido faturado ou entregue nao pode ser excluido"). API: 409 `"pedido faturado não pode ser excluído; só pode ter o status alterado para Entregue"` / `"pedido entregue não pode ser excluído"`; com pagamento vinculado, 409 `"pedido possui pagamentos vinculados"` primeiro. |
| 2.7 | Status mudou para Faturado com o modal aberto (outra aba) e salvar "Em separação" | Alerta "Pedido faturado so pode ter o status alterado para Entregue."; itens diferentes: "Pedido faturado ou entregue: nao e possivel alterar os itens.". |
| 2.8 | Pedido da loja em "Aguardando pagamento" | O select inclui "Aguardando pagamento" só nesse pedido. |

## 3. Normal com vendedor e sem vendedor

| Passo | Perfil / ação | Esperado |
| --- | --- | --- |
| 3.1 | Rafael: Novo Pedido | Vendedor travado em "Rafael Carvalho"; clientes da carteira carregam sozinhos; criar funciona. |
| 3.2 | Rafael: `POST` com `vendedor_id` de outro | 201 com `vendedor_id` 4 (forçado ao próprio). |
| 3.3 | Rafael: cliente sai da carteira com o modal aberto e salvar | Alerta "O cliente selecionado nao pertence a carteira ativa do vendedor. Escolha um cliente da carteira e tente novamente." (API: 400 `"cliente não pertence à carteira deste vendedor"`). |
| 3.4 | Rafael: editar pedido com cliente fora da carteira sem trocar o cliente | Mesma mensagem; o registro não muda (comportamento aceito). Trocar por um da carteira some com a opção "(fora da carteira)". |
| 3.5 | QA sem vendedor | Aviso "Seu usuario nao esta vinculado a um vendedor, por isso nao e possivel registrar pedidos. Solicite o vinculo ao administrador.", Vendedor "Sem vendedor vinculado", Salvar desabilitado, lista vazia. API: `POST` 403 `"usuário sem vendedor vinculado"`, `PUT` 404. |
| 3.6 | Desligado | Aviso de vendedor desligado, sem "Novo Pedido" (roteiro de sessão, seção 5). |

## 4. Pagamentos (`/pagamentos`)

| Passo | Ação | Esperado |
| --- | --- | --- |
| 4.1 | Modal "Novo" | A busca de pedido filtra depois de uma pausa; ao reabrir, busca e seleção vêm vazias. |
| 4.2 | Criar um pagamento "Em aberto" | "Pagamento #N criado com sucesso."; 201 com `created_at`/`updated_at` de agora. |
| 4.3 | Datas sem hora (vencimento, pagamento) | Iguais às do banco, sem um dia a menos; vazio aparece como `-`. |
| 4.4 | Excluir pagamento "Pago" ou "Pago com atraso" | 409 `"pagamento já quitado não pode ser excluído"`; usuário normal editando quitado: 409 `"pagamento já quitado não pode ser editado"`. |
| 4.5 | Excluir o "Em aberto" e paginar logo depois | A linha não reaparece e o total diminui 1. |

## 5. Loja: checkout e pagamento

Logado com a conta de cliente da loja; carrinho com 1 ou 2 produtos da vitrine (`/`).

| Passo | Ação | Esperado |
| --- | --- | --- |
| 5.1 | `/checkout` sem login | "Entre na sua conta para finalizar a compra."; carrinho vazio: "Seu carrinho esta vazio.". |
| 5.2 | Endereço: digitar o CEP | Rua, Número, Bairro, Cidade e Estado ficam bloqueados até a pesquisa; com 8 dígitos a ViaCEP preenche ("Endereco preenchido pelo CEP. Confira e informe o numero."). CEP inexistente: "CEP nao encontrado: preencha o endereco abaixo.". |
| 5.3 | "Finalizar pedido" com endereço incompleto | "Preencha o endereco de entrega para finalizar a compra.", sem requisição. API sem entrega: 400 (ex.: `"cep de entrega deve ter 8 dígitos"`). |
| 5.4 | Finalizar com Pix, Cartão ou Boleto | "Pedido #N recebido!", "Pagamento: Em aberto." e "O pedido vai para separação assim que o pagamento for confirmado."; instrução da forma (boleto com a data de vencimento). Carrinho limpo. |
| 5.5 | Pedido no admin (`/admin/pedidos`) | Canal "Loja", status "Aguardando pagamento". |
| 5.6 | Simulador (só `APP_ENV` dev/qa): "Aprovar pagamento" | Pedido "Em separação" e pagamento "Pago" com a data de hoje ("Pagamento confirmado! O pedido já está em separação."). |
| 5.7 | Outra compra: "Recusar pagamento" | Pedido "Cancelado" e pagamento "Recusado" ("Pagamento recusado: o pedido foi cancelado."); a reserva de estoque é liberada. |
| 5.8 | `POST /api/loja/pagamentos/{id}/simular` de novo no mesmo pedido | 409 `"o pedido não está aguardando pagamento"`; `resultado` diferente de `aprovado`/`recusado`: 400; pedido de outro login: 404. |
| 5.9 | API com `APP_ENV=producao` (ou sem a variável) | Simulador some da tela e a rota responde 404 `"recurso não encontrado"`. |
| 5.10 | Login da equipe (admin/normal) no checkout | 403 `"apenas clientes da loja podem finalizar compras"`. |

> Pedido "Aguardando pagamento" com pagamento vencido antes de hoje é cancelado pelo job da API (`LOJA_EXPIRACAO_INTERVAL`, padrão 1h; `0` desliga): vira "Cancelado" + "Recusado".

## 6. Loja: "Minhas compras"

| Passo | Ação | Esperado |
| --- | --- | --- |
| 6.1 | `/minhas-compras` sem login | "Entre na sua conta para ver as suas compras.", sem chamar a API. |
| 6.2 | Logado | Uma linha por pedido (Pedido, Data, Produtos, Status, Total), mais recente primeiro; ordenar por Pedido, Data e Total; 20 por página. Sem compras: "Voce ainda nao fez nenhuma compra.". |
| 6.3 | Clicar numa linha | Abre `/minhas-compras/{id}`: "Compra #N", status, data, itens (SKU, qtd x preço), total, endereço de entrega e pagamento (forma, valor, situação, vencimento, "Pago em"). |
| 6.4 | Detalhe de pedido "Aguardando pagamento" | Texto de espera e o simulador (dev/qa); aprovar ou recusar recarrega o detalhe com o novo status. |
| 6.5 | `/minhas-compras/<id de outro cliente>`, inexistente ou `abc` | "Compra nao encontrada." (API 404). |

## 7. Limpeza

- Exclua os pedidos de teste "Em separação" e os pagamentos "Em aberto"; anote os Faturado/Entregue e os da loja para limpar por SQL no dev.
- Restaure carteiras encerradas (`UPDATE carteiras SET data_fim = NULL WHERE vendedor_id = 4 AND cliente_id = <id> AND data_fim = CURDATE();`) e remova o usuário QA.
