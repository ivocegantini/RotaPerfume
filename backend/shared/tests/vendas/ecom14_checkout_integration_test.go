package vendas_test

// ECOM-14: integração (INTEGRATION=1; senão t.Skip; SEC-50) do checkout no
// MariaDB real: pedido + itens + pagamento numa tx com os preços do banco,
// chave repetida devolve o mesmo pedido, produto inativo desfaz tudo e
// login sem cliente é recusado. Dados "ZZ-TEST-ECOM14-*", apagados no Cleanup.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/testkit/integracao"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

func TestIntegracaoECOM14_Checkout(t *testing.T) {
	conn := integracao.Abrir(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	tag := fmt.Sprintf("ZZ-TEST-ECOM14-%d", n)

	usuario := inserirECOM12(t, conn, `INSERT INTO usuarios (nome, email, password_hash, role) VALUES (?, ?, 'x', 'usuario')`, tag, fmt.Sprintf("ecom14-%d@teste.invalid", n))
	semCliente := inserirECOM12(t, conn, `INSERT INTO usuarios (nome, email, password_hash, role) VALUES (?, ?, 'x', 'usuario')`, tag, fmt.Sprintf("ecom14-sc-%d@teste.invalid", n))
	cliente := inserirECOM12(t, conn, `INSERT INTO clientes (cnpj, cpf, razao_social, segmento, cidade, uf, endereco, numero, data_cadastro, usuario_id) VALUES (NULL, ?, ?, 'Loja Online', 'Curitiba', 'PR', 'Rua Teste', 's/n', '2026-01-01', ?)`,
		fmt.Sprintf("%011d", (n+7)%1e11), tag, usuario)
	// SEC-79: segundo cliente, para a disputa pela última unidade.
	usuario2 := inserirECOM12(t, conn, `INSERT INTO usuarios (nome, email, password_hash, role) VALUES (?, ?, 'x', 'usuario')`, tag, fmt.Sprintf("ecom14-2-%d@teste.invalid", n))
	cliente2 := inserirECOM12(t, conn, `INSERT INTO clientes (cnpj, cpf, razao_social, segmento, cidade, uf, endereco, numero, data_cadastro, usuario_id) VALUES (NULL, ?, ?, 'Loja Online', 'Curitiba', 'PR', 'Rua Teste', 's/n', '2026-01-01', ?)`,
		fmt.Sprintf("%011d", (n+13)%1e11), tag, usuario2)
	var vendedorCriado int64
	if err := conn.QueryRow(`SELECT id FROM vendedores WHERE nome = 'Loja Online' LIMIT 1`).Scan(new(int64)); err == sql.ErrNoRows {
		vendedorCriado = inserirECOM12(t, conn, `INSERT INTO vendedores (nome, regiao, uf, data_admissao, meta_mensal) VALUES ('Loja Online', 'Online', 'SP', '2026-01-01', 0)`)
	}
	produto := func(sufixo, preco string, ativo bool) int64 {
		return inserirECOM12(t, conn, `INSERT INTO produtos (sku, nome, categoria, marca, preco_tabela, custo_unitario, unidade, ativo) VALUES (?, ?, 'Teste', 'Teste', ?, '1.00', 'UN', ?)`,
			fmt.Sprintf("E14-%d-%s", n%1e12, sufixo), tag+"-"+sufixo, preco, ativo)
	}
	prodA, prodB, inativo := produto("A", "100.25", true), produto("B", "0.10", true), produto("I", "5.00", false)
	prodC := produto("C", "9.90", true) // SEC-79: última unidade disputada
	// SEC-79: estoque de A tem um snapshot antigo (999) e o último (5); B tem 1.
	skuDe := func(id int64) string {
		var sku string
		require.NoError(t, conn.QueryRow(`SELECT sku FROM produtos WHERE id = ?`, id).Scan(&sku))
		return sku
	}
	for _, e := range []struct {
		id    int64
		dia   string
		saldo int
	}{{prodA, "2026-01-01", 999}, {prodA, "2026-02-01", 5}, {prodB, "2026-02-01", 1}, {prodC, "2026-02-01", 1}} {
		inserirECOM12(t, conn, `INSERT INTO estoque (data_snapshot, sku, saldo, ruptura) VALUES (?, ?, ?, 0)`, e.dia, skuDe(e.id), e.saldo)
	}
	t.Cleanup(func() {
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"DELETE pg FROM pagamentos pg JOIN pedidos p ON p.pedido_id_origem = pg.pedido_id WHERE p.cliente_id IN (?, ?)", []any{cliente, cliente2}},
			{"DELETE FROM pedidos WHERE cliente_id IN (?, ?)", []any{cliente, cliente2}},
			{"DELETE FROM clientes WHERE cliente_id_origem IN (?, ?)", []any{cliente, cliente2}},
			{"DELETE e FROM estoque e JOIN produtos p ON p.sku = e.sku WHERE p.id IN (?, ?, ?, ?)", []any{prodA, prodB, inativo, prodC}},
			{"DELETE FROM produtos WHERE id IN (?, ?, ?, ?)", []any{prodA, prodB, inativo, prodC}},
			{"DELETE FROM usuarios WHERE id IN (?, ?, ?)", []any{usuario, semCliente, usuario2}},
			{"DELETE FROM vendedores WHERE id = ?", []any{vendedorCriado}},
		} {
			_, err := conn.Exec(q.sql, q.args...)
			assert.NoError(t, err, q.sql)
		}
	})

	tx := db.NewTransactor(conn, func(q db.Querier) vendasapp.TxPedido {
		return vendasapp.TxPedido{Pedidos: vendasmysql.NovoPedidos(q), Pagamentos: vendasmysql.NovoPagamentos(q), Loja: vendasmysql.NovaLoja(q)}
	})
	finalizar := func(usuarioID int64, chave string, itens ...vendas.ItemCheckout) (*vendas.CompraLoja, error) {
		v, err := vendas.DadosCheckout{Chave: chave, Forma: "boleto", Itens: itens, Entrega: entregaTeste}.Validar()
		require.NoError(t, err)
		var res *vendas.CompraLoja
		err = tx.WithTx(ctx, func(r vendasapp.TxPedido) error {
			res, err = r.FinalizarCompra(ctx, usuarioID, v, time.Now())
			return err
		})
		return res, err
	}
	chave := fmt.Sprintf("%08x-0000-4000-8000-%012x", n%0xffffffff, n%0xffffffffffff)
	itens := []vendas.ItemCheckout{{ProdutoID: prodA, Quantidade: 3}, {ProdutoID: prodB, Quantidade: 1}}

	res, err := finalizar(usuario, chave, itens...)
	require.NoError(t, err)
	assert.False(t, res.Repetida)
	id := res.Pedido.PedidoIDOrigem
	var canal, status, total, chaveGravada, forma, statusPag, valorPag string
	require.NoError(t, conn.QueryRow(`SELECT canal, status, valor_total, chave_checkout FROM pedidos WHERE pedido_id_origem = ?`, id).Scan(&canal, &status, &total, &chaveGravada))
	var cep, uf string // ECOM-17: o endereço de entrega foi gravado
	require.NoError(t, conn.QueryRow(`SELECT entrega_cep, entrega_uf FROM pedidos WHERE pedido_id_origem = ?`, id).Scan(&cep, &uf))
	assert.Equal(t, "01310100", cep)
	assert.Equal(t, "SP", uf)
	assert.Equal(t, []string{"Loja", "Em separação", "300.85", chave}, []string{canal, status, total, chaveGravada})
	var nItens int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM itens_pedido WHERE pedido_id = ?`, id).Scan(&nItens))
	assert.Equal(t, 2, nItens)
	require.NoError(t, conn.QueryRow(`SELECT forma_pagamento, status_pagamento, valor FROM pagamentos WHERE pedido_id = ?`, id).Scan(&forma, &statusPag, &valorPag))
	assert.Equal(t, []string{"Boleto 14 dias", "Em aberto", "300.85"}, []string{forma, statusPag, valorPag})

	// Reenvio com a mesma chave: o mesmo pedido, sem gravar outro.
	res, err = finalizar(usuario, chave, itens...)
	require.NoError(t, err)
	assert.True(t, res.Repetida)
	assert.Equal(t, id, res.Pedido.PedidoIDOrigem)
	require.NotNil(t, res.Pagamento)
	assert.Equal(t, "Boleto 14 dias", res.Pagamento.FormaPagamento)

	// Produto inativo: 409 e nada gravado (rollback).
	outra := fmt.Sprintf("%08x-0000-4000-8000-%012x", (n+1)%0xffffffff, (n+1)%0xffffffffffff)
	_, err = finalizar(usuario, outra, vendas.ItemCheckout{ProdutoID: prodA, Quantidade: 1}, vendas.ItemCheckout{ProdutoID: inativo, Quantidade: 1})
	assert.ErrorIs(t, err, vendas.ErrCheckoutProdutoIndisponivel)
	var nPedidos int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM pedidos WHERE cliente_id = ?`, cliente).Scan(&nPedidos))
	assert.Equal(t, 1, nPedidos, "o inativo não gravou nada")

	// SEC-79: A tinha 5 e o 1º pedido (Em separação) reservou 3: sobram 2.
	terceira := fmt.Sprintf("%08x-0000-4000-8000-%012x", (n+2)%0xffffffff, (n+2)%0xffffffffffff)
	_, err = finalizar(usuario, terceira, vendas.ItemCheckout{ProdutoID: prodA, Quantidade: 3})
	assert.ErrorIs(t, err, vendas.ErrCheckoutSemEstoque, "5 - 3 reservados = 2 < 3")
	// B tinha 1 e o 1º pedido levou 1: zerado.
	_, err = finalizar(usuario, terceira, vendas.ItemCheckout{ProdutoID: prodB, Quantidade: 1})
	assert.ErrorIs(t, err, vendas.ErrCheckoutSemEstoque)
	res, err = finalizar(usuario, terceira, vendas.ItemCheckout{ProdutoID: prodA, Quantidade: 2})
	require.NoError(t, err, "exatamente o disponível")
	assert.False(t, res.Repetida)
	// Cancelado não reserva: cancelar o último devolve as 2 unidades.
	_, err = conn.Exec(`UPDATE pedidos SET status = 'Cancelado' WHERE pedido_id_origem = ?`, res.Pedido.PedidoIDOrigem)
	require.NoError(t, err)
	quarta := fmt.Sprintf("%08x-0000-4000-8000-%012x", (n+3)%0xffffffff, (n+3)%0xffffffffffff)
	_, err = finalizar(usuario, quarta, vendas.ItemCheckout{ProdutoID: prodA, Quantidade: 2})
	require.NoError(t, err)

	// SEC-79: dois clientes ao mesmo tempo pela última unidade de C: o lock
	// do produto serializa e só um leva (o outro: sem estoque).
	var wg sync.WaitGroup
	erros := make([]error, 2)
	for i, u := range []int64{usuario, usuario2} {
		wg.Add(1)
		go func(i int, u int64) {
			defer wg.Done()
			ch := fmt.Sprintf("%08x-0000-4000-8000-%012x", (n+10+int64(i))%0xffffffff, (n+10+int64(i))%0xffffffffffff)
			v, err := vendas.DadosCheckout{Chave: ch, Forma: "pix", Itens: []vendas.ItemCheckout{{ProdutoID: prodC, Quantidade: 1}}, Entrega: entregaTeste}.Validar()
			if err != nil {
				erros[i] = err
				return
			}
			erros[i] = tx.WithTx(ctx, func(r vendasapp.TxPedido) error {
				_, err := r.FinalizarCompra(ctx, u, v, time.Now())
				return err
			})
		}(i, u)
	}
	wg.Wait()
	ok, semEstoque := 0, 0
	for _, e := range erros {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, vendas.ErrCheckoutSemEstoque):
			semEstoque++
		default:
			t.Errorf("erro inesperado na disputa: %v", e)
		}
	}
	assert.Equal(t, []int{1, 1}, []int{ok, semEstoque}, "uma venda e uma recusa")

	// Login sem cliente da loja: 403 (ErrCheckoutSemCliente).
	_, err = finalizar(semCliente, outra, itens...)
	assert.ErrorIs(t, err, vendas.ErrCheckoutSemCliente)
}
