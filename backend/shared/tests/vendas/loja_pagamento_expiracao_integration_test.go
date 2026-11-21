package vendas_test

// ECOM-18/19: integração (INTEGRATION=1; senão t.Skip; SEC-50) no MariaDB real:
// o checkout nasce Aguardando pagamento; o simulador aprova (Em separação +
// Pago); o job cancela o vencido (Cancelado + Recusado) e libera a reserva.
// Dados "ZZ-TEST-ECOM19-*", apagados no Cleanup.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/testkit/integracao"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

func TestIntegracaoECOM19_PagamentoEExpiracao(t *testing.T) {
	conn := integracao.Abrir(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	tag := fmt.Sprintf("ZZ-TEST-ECOM19-%d", n)

	usuario := inserirECOM12(t, conn, `INSERT INTO usuarios (nome, email, password_hash, role) VALUES (?, ?, 'x', 'usuario')`, tag, fmt.Sprintf("ecom19-%d@teste.invalid", n))
	cliente := inserirECOM12(t, conn, `INSERT INTO clientes (cnpj, cpf, razao_social, segmento, cidade, uf, endereco, numero, data_cadastro, usuario_id) VALUES (NULL, ?, ?, 'Loja Online', 'Curitiba', 'PR', 'Rua Teste', 's/n', '2026-01-01', ?)`,
		fmt.Sprintf("%011d", (n+19)%1e11), tag, usuario)
	var vendedorCriado int64
	if err := conn.QueryRow(`SELECT id FROM vendedores WHERE nome = 'Loja Online' LIMIT 1`).Scan(new(int64)); err != nil {
		vendedorCriado = inserirECOM12(t, conn, `INSERT INTO vendedores (nome, regiao, uf, data_admissao, meta_mensal) VALUES ('Loja Online', 'Online', 'SP', '2026-01-01', 0)`)
	}
	sku := fmt.Sprintf("E19-%d", n%1e12)
	prod := inserirECOM12(t, conn, `INSERT INTO produtos (sku, nome, categoria, marca, preco_tabela, custo_unitario, unidade, ativo) VALUES (?, ?, 'Teste', 'Teste', '10.00', '1.00', 'UN', 1)`, sku, tag)
	inserirECOM12(t, conn, `INSERT INTO estoque (data_snapshot, sku, saldo, ruptura) VALUES ('2026-02-01', ?, 4, 0)`, sku)
	t.Cleanup(func() {
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"DELETE pg FROM pagamentos pg JOIN pedidos p ON p.pedido_id_origem = pg.pedido_id WHERE p.cliente_id = ?", []any{cliente}},
			{"DELETE FROM pedidos WHERE cliente_id = ?", []any{cliente}},
			{"DELETE FROM clientes WHERE cliente_id_origem = ?", []any{cliente}},
			{"DELETE FROM estoque WHERE sku = ?", []any{sku}},
			{"DELETE FROM produtos WHERE id = ?", []any{prod}},
			{"DELETE FROM usuarios WHERE id = ?", []any{usuario}},
			{"DELETE FROM vendedores WHERE id = ?", []any{vendedorCriado}},
		} {
			_, err := conn.Exec(q.sql, q.args...)
			assert.NoError(t, err, q.sql)
		}
	})

	tx := db.NewTransactor(conn, func(q db.Querier) vendasapp.TxPedido {
		return vendasapp.TxPedido{Pedidos: vendasmysql.NovoPedidos(q), Pagamentos: vendasmysql.NovoPagamentos(q),
			Estoque: catalogomysql.NovoEstoques(q), Loja: vendasmysql.NovaLoja(q)}
	})
	casos := vendasapp.Novo(vendasapp.Deps{Pedidos: vendasmysql.NovoPedidos(conn), Pagamentos: vendasmysql.NovoPagamentos(conn),
		Escopo: escopoFixo{e: kernel.Escopo{}}, TxPedido: tx, Relogio: time.Now})
	dono, err := kernel.NovoPrincipal(usuario, kernel.RoleDoBanco("usuario"))
	require.NoError(t, err)

	comprar := func(i int64) int64 {
		chave := fmt.Sprintf("%08x-0000-4000-8000-%012x", (n+i)%0xffffffff, (n+i)%0xffffffffffff)
		res, err := casos.FinalizarCompra(ctx, dono, ler(vendas.DadosCheckout{Chave: chave, Forma: "pix",
			Itens: []vendas.ItemCheckout{{ProdutoID: prod, Quantidade: 2}}, Entrega: entregaTeste}))
		require.NoError(t, err)
		assert.Equal(t, "Aguardando pagamento", res.Pedido.Status)
		return res.Pedido.PedidoIDOrigem
	}
	disponivel := func() int {
		var d int
		require.NoError(t, tx.WithTx(ctx, func(r vendasapp.TxPedido) error {
			o, err := r.Loja.OfertasAtivas(ctx, []int64{prod})
			d = o[prod].Disponivel
			return err
		}))
		return d
	}
	status := func(id int64) (ped, pag string) {
		require.NoError(t, conn.QueryRow(`SELECT p.status, g.status_pagamento FROM pedidos p JOIN pagamentos g ON g.pedido_id = p.pedido_id_origem WHERE p.pedido_id_origem = ?`, id).Scan(&ped, &pag))
		return
	}

	// ECOM-18: aprovado vai para separação (continua reservado).
	aprovado := comprar(1)
	res, err := casos.ConfirmarPagamentoLoja(ctx, dono, aprovado, ler("aprovado"))
	require.NoError(t, err)
	assert.Equal(t, "Em separação", res.Pedido.Status)
	ped, pag := status(aprovado)
	assert.Equal(t, []string{"Em separação", "Pago"}, []string{ped, pag})
	_, err = casos.ConfirmarPagamentoLoja(ctx, dono, aprovado, ler("recusado"))
	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoPendente, "não confirma duas vezes")

	// ECOM-19: o segundo pedido vence sem pagamento e o job o cancela.
	vencido := comprar(2)
	assert.Equal(t, 0, disponivel(), "4 em estoque − 2 em separação − 2 aguardando")
	_, err = conn.Exec(`UPDATE pagamentos SET data_vencimento = '2026-01-01' WHERE pedido_id = ?`, vencido)
	require.NoError(t, err)
	cancelados, err := casos.ExpirarPedidosLoja(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, cancelados, 1)
	ped, pag = status(vencido)
	assert.Equal(t, []string{"Cancelado", "Recusado"}, []string{ped, pag})
	assert.Equal(t, 2, disponivel(), "o cancelado libera a reserva")
	ped, _ = status(aprovado)
	assert.Equal(t, "Em separação", ped, "o pago não é tocado")
}
