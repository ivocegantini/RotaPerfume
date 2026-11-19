package vendas_test

// ECOM-12/16: integração (INTEGRATION=1; senão t.Skip; SEC-50: integracao.Abrir
// recusa a produção) do Pedidos.ComprasDoUsuario/CompraDoUsuario no MariaDB
// real: um pedido por linha com os nomes dos produtos (sem repetir), ordenação
// e paginação, detalhe com itens e pagamento, e o outro login não vê nada.
// Dados "ZZ-TEST-ECOM12-*", apagados no t.Cleanup.

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/testkit/integracao"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

type idsECOM12 struct{ usuario, outro, cliente, vendedor, prodA, prodB, prodC, p1, p2, p3 int64 }

func inserirECOM12(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(q, args...)
	require.NoError(t, err, q)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

// dadosECOM12: p1 (01/10, A duas vezes + B inativo, R$ 40), p2 (10/11, A,
// R$ 20, com pagamento) e p3 (15/11, Cancelado, C, R$ 90).
func dadosECOM12(t *testing.T, db *sql.DB) idsECOM12 {
	t.Helper()
	n := time.Now().UnixNano()
	tag := fmt.Sprintf("ZZ-TEST-ECOM12-%d", n)
	var ids idsECOM12
	ids.usuario = inserirECOM12(t, db, `INSERT INTO usuarios (nome, email, password_hash, role) VALUES (?, ?, 'x', 'usuario')`, tag, fmt.Sprintf("ecom12-%d@teste.invalid", n))
	ids.outro = inserirECOM12(t, db, `INSERT INTO usuarios (nome, email, password_hash, role) VALUES (?, ?, 'x', 'usuario')`, tag, fmt.Sprintf("ecom12-outro-%d@teste.invalid", n))
	ids.vendedor = inserirECOM12(t, db, `INSERT INTO vendedores (nome, regiao, uf, data_admissao, meta_mensal) VALUES (?, 'Teste', 'PR', '2024-01-01', 0)`, tag)
	// Pessoa física (chk_clientes_documento: cnpj ou cpf); cpf UNIQUE, sem DV no banco.
	ids.cliente = inserirECOM12(t, db, `INSERT INTO clientes (cnpj, cpf, razao_social, segmento, cidade, uf, endereco, numero, data_cadastro, usuario_id) VALUES (NULL, ?, ?, 'Loja Online', 'Curitiba', 'PR', 'Rua Teste', 's/n', '2026-01-01', ?)`,
		fmt.Sprintf("%011d", n%1e11), tag, ids.usuario)
	produto := func(sufixo string, ativo bool) int64 {
		return inserirECOM12(t, db, `INSERT INTO produtos (sku, nome, categoria, marca, preco_tabela, custo_unitario, unidade, ativo) VALUES (?, ?, 'Teste', 'Teste', '10.00', '5.00', 'UN', ?)`,
			fmt.Sprintf("E12-%d-%s", n%1e12, sufixo), tag+"-"+sufixo, ativo)
	}
	ids.prodA, ids.prodB, ids.prodC = produto("A", true), produto("B", false), produto("C", true)
	pedido := func(dia, status, total string) int64 {
		return inserirECOM12(t, db, `INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total, entrega_cep, entrega_endereco, entrega_numero, entrega_bairro, entrega_cidade, entrega_uf) VALUES (?, ?, ?, 'Loja', ?, ?, '01310100', 'Avenida Paulista', '1000', 'Bela Vista', 'São Paulo', 'SP')`, ids.cliente, ids.vendedor, dia, status, total)
	}
	item := func(pedidoID, produtoID int64, qtd int, bruto string) {
		inserirECOM12(t, db, `INSERT INTO itens_pedido (pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto) VALUES (?, ?, ?, '10.00', 0, ?)`, pedidoID, produtoID, qtd, bruto)
	}
	p1, p2, p3 := pedido("2026-10-01", "Entregue", "40.00"), pedido("2026-11-10", "Faturado", "20.00"), pedido("2026-11-15", "Cancelado", "90.00")
	ids.p1, ids.p2, ids.p3 = p1, p2, p3
	item(p1, ids.prodA, 2, "20.00")
	item(p1, ids.prodB, 1, "10.00")
	item(p1, ids.prodA, 1, "10.00") // mesmo produto em 2 itens: o nome sai uma vez
	inserirECOM12(t, db, `INSERT INTO pagamentos (pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento) VALUES (?, 'PIX', 1, '20.00', '1.00', '19.80', '2026-11-10', NULL, 'Em aberto')`, p2)
	item(p2, ids.prodA, 2, "20.00")
	item(p3, ids.prodC, 9, "90.00")

	t.Cleanup(func() {
		for _, c := range []struct {
			q   string
			arg int64
		}{
			{"DELETE FROM pagamentos WHERE pedido_id IN (SELECT pedido_id_origem FROM pedidos WHERE cliente_id = ?)", ids.cliente},
			{"DELETE FROM pedidos WHERE cliente_id = ?", ids.cliente}, // itens vão em cascata
			{"DELETE FROM clientes WHERE cliente_id_origem = ?", ids.cliente},
			{"DELETE FROM vendedores WHERE id = ?", ids.vendedor},
			{"DELETE FROM produtos WHERE id IN (?, ?, ?)", 0},
			{"DELETE FROM usuarios WHERE id IN (?, ?)", 0},
		} {
			var err error
			switch {
			case strings.Contains(c.q, "produtos"):
				_, err = db.Exec(c.q, ids.prodA, ids.prodB, ids.prodC)
			case strings.Contains(c.q, "usuarios"):
				_, err = db.Exec(c.q, ids.usuario, ids.outro)
			default:
				_, err = db.Exec(c.q, c.arg)
			}
			assert.NoError(t, err, c.q)
		}
	})
	return ids
}

func TestIntegracaoECOM16_ComprasDoUsuario(t *testing.T) {
	db := integracao.Abrir(t)
	ctx := context.Background()
	ids := dadosECOM12(t, db)

	repo := vendasmysql.NovoPedidos(db)

	res, err := repo.ComprasDoUsuario(ctx, ids.usuario, vendas.OrdenacaoCompraPadrao(), kernel.NovaPagina(1, 20))
	require.NoError(t, err)
	assert.Equal(t, 3, res.Total)
	require.Len(t, res.Itens, 3)
	// data desc: p3 (15/11), p2 (10/11), p1 (01/10); o cancelado aparece com o status.
	assert.Equal(t, []int64{ids.p3, ids.p2, ids.p1}, []int64{res.Itens[0].PedidoID, res.Itens[1].PedidoID, res.Itens[2].PedidoID})
	assert.Equal(t, "Cancelado", res.Itens[0].Status)
	p1 := res.Itens[2]
	assert.Equal(t, "2026-10-01", p1.Data.Format("2006-01-02"))
	assert.Equal(t, "40.00", p1.ValorTotal.String())
	require.Len(t, p1.Produtos, 2, "A repetido sai uma vez")
	assert.True(t, strings.HasSuffix(p1.Produtos[0], "-A"))
	assert.True(t, strings.HasSuffix(p1.Produtos[1], "-B"))

	asc := kernel.ParseOrdenacao("valor_total", "asc", vendas.CamposOrdenacaoCompra(), vendas.OrdenacaoCompraPadrao())
	res, err = repo.ComprasDoUsuario(ctx, ids.usuario, asc, kernel.NovaPagina(1, 1))
	require.NoError(t, err)
	assert.Equal(t, 3, res.Total)
	require.Len(t, res.Itens, 1)
	assert.Equal(t, ids.p2, res.Itens[0].PedidoID, "menor total primeiro, 1 por página")

	res, err = repo.ComprasDoUsuario(ctx, ids.outro, vendas.OrdenacaoCompraPadrao(), kernel.NovaPagina(1, 20))
	require.NoError(t, err)
	assert.Equal(t, 0, res.Total, "outro login não vê as compras alheias")
	assert.Empty(t, res.Itens)

	d, err := repo.CompraDoUsuario(ctx, ids.usuario, ids.p1)
	require.NoError(t, err)
	require.Len(t, d.Itens, 3)
	assert.Equal(t, ids.prodA, d.Itens[0].ProdutoID)
	assert.Equal(t, 2, d.Itens[0].Quantidade)
	assert.Equal(t, "20.00", d.Itens[0].ValorBruto.String())
	assert.False(t, d.Itens[1].Ativo)
	assert.Empty(t, d.Pagamentos)

	d, err = repo.CompraDoUsuario(ctx, ids.usuario, ids.p2)
	require.NoError(t, err)
	require.Len(t, d.Pagamentos, 1)
	assert.Equal(t, "PIX", d.Pagamentos[0].Forma)
	assert.Equal(t, "Em aberto", d.Pagamentos[0].Status)
	assert.Nil(t, d.Pagamentos[0].DataPagamento)

	_, err = repo.CompraDoUsuario(ctx, ids.outro, ids.p1)
	assert.ErrorIs(t, err, vendas.ErrPedidoNaoEncontrado, "pedido alheio = 404")
}
