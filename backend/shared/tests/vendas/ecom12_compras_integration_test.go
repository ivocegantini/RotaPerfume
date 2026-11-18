package vendas_test

// ECOM-12: integração (INTEGRATION=1; senão t.Skip; SEC-50: integracao.Abrir
// recusa a produção) do Pedidos.ComprasDoUsuario no MariaDB real, com
// ONLY_FULL_GROUP_BY na sessão: soma por produto, pedido Cancelado fora,
// ordenação e paginação, e o login de outra pessoa não vê nada.
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

type idsECOM12 struct{ usuario, outro, cliente, vendedor, prodA, prodB, prodC int64 }

func inserirECOM12(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(q, args...)
	require.NoError(t, err, q)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

// dadosECOM12: A em 2 pedidos (3 + 2 un.), B em 1 pedido (1 un., produto
// inativo), C só num pedido Cancelado (não aparece).
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
	pedido := func(dia, status string) int64 {
		return inserirECOM12(t, db, `INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total) VALUES (?, ?, ?, 'Loja', ?, '0.00')`, ids.cliente, ids.vendedor, dia, status)
	}
	item := func(pedidoID, produtoID int64, qtd int, bruto string) {
		inserirECOM12(t, db, `INSERT INTO itens_pedido (pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto) VALUES (?, ?, ?, '10.00', 0, ?)`, pedidoID, produtoID, qtd, bruto)
	}
	p1, p2, p3 := pedido("2026-10-01", "Entregue"), pedido("2026-11-10", "Faturado"), pedido("2026-11-15", "Cancelado")
	item(p1, ids.prodA, 3, "30.00")
	item(p1, ids.prodB, 1, "10.00")
	item(p2, ids.prodA, 2, "20.00")
	item(p3, ids.prodC, 9, "90.00")

	t.Cleanup(func() {
		for _, c := range []struct {
			q   string
			arg int64
		}{
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

func TestIntegracaoECOM12_ComprasDoUsuario(t *testing.T) {
	db := integracao.Abrir(t)
	ctx := context.Background()
	ids := dadosECOM12(t, db)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = conn.ExecContext(ctx, "SET SESSION sql_mode = CONCAT_WS(',', NULLIF(@@SESSION.sql_mode, ''), 'ONLY_FULL_GROUP_BY')")
	require.NoError(t, err)
	repo := vendasmysql.NovoPedidos(conn)

	res, err := repo.ComprasDoUsuario(ctx, ids.usuario, vendas.OrdenacaoCompraPadrao(), kernel.NovaPagina(1, 20))
	require.NoError(t, err)
	assert.Equal(t, 2, res.Total, "C só está em pedido Cancelado")
	require.Len(t, res.Itens, 2)
	a, b := res.Itens[0], res.Itens[1] // última compra desc: A (10/11) antes de B (01/10)
	assert.Equal(t, ids.prodA, a.ProdutoID)
	assert.Equal(t, 5, a.Quantidade)
	assert.Equal(t, 2, a.Pedidos)
	assert.Equal(t, "50.00", a.TotalGasto.String())
	assert.Equal(t, "2026-11-10", a.UltimaCompra.Format("2006-01-02"))
	assert.True(t, a.Ativo)
	assert.Equal(t, ids.prodB, b.ProdutoID)
	assert.Equal(t, 1, b.Quantidade)
	assert.False(t, b.Ativo)

	asc := kernel.ParseOrdenacao("quantidade", "asc", vendas.CamposOrdenacaoCompra(), vendas.OrdenacaoCompraPadrao())
	res, err = repo.ComprasDoUsuario(ctx, ids.usuario, asc, kernel.NovaPagina(1, 1))
	require.NoError(t, err)
	assert.Equal(t, 2, res.Total)
	require.Len(t, res.Itens, 1)
	assert.Equal(t, ids.prodB, res.Itens[0].ProdutoID, "menor quantidade primeiro, 1 por página")

	res, err = repo.ComprasDoUsuario(ctx, ids.outro, vendas.OrdenacaoCompraPadrao(), kernel.NovaPagina(1, 20))
	require.NoError(t, err)
	assert.Equal(t, 0, res.Total, "outro login não vê as compras alheias")
	assert.Empty(t, res.Itens)
}
