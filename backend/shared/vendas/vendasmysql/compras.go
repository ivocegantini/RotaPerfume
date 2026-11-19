package vendasmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

// comprasFrom liga o login ao cliente da loja (clientes.usuario_id, UNIQUE)
// e aos pedidos dele (todos os status; o cancelado aparece como tal).
const comprasFrom = ` FROM pedidos p
	JOIN clientes c ON c.cliente_id_origem = p.cliente_id
	WHERE c.usuario_id = ?`

const comprasCount = `SELECT COUNT(*)` + comprasFrom

const comprasSelect = `SELECT p.pedido_id_origem, p.data_pedido, p.status, p.valor_total` + comprasFrom

// ordemCompra monta o ORDER BY pela whitelist; o id desempata (paginação estável).
func ordemCompra(o kernel.Ordenacao[vendas.CampoCompra]) string {
	dir := db.DirecaoSQL(o.Direcao(), "DESC")
	return " ORDER BY " + colunaCompra(o.Campo()) + " " + dir + ", p.pedido_id_origem " + dir + " LIMIT ? OFFSET ?"
}

// colunaCompra é a whitelist: enum → coluna.
func colunaCompra(c vendas.CampoCompra) string {
	switch c {
	case vendas.OrdemCompraPedido:
		return "p.pedido_id_origem"
	case vendas.OrdemCompraTotal:
		return "p.valor_total"
	default:
		return "p.data_pedido"
	}
}

// ComprasDoUsuario pagina os pedidos do login (ECOM-16), com os nomes dos
// produtos de cada um (2ª query, só pelos ids já filtrados pelo usuário).
func (r *Pedidos) ComprasDoUsuario(ctx context.Context, usuarioID int64, o kernel.Ordenacao[vendas.CampoCompra], p kernel.Pagina) (kernel.Resultado[vendas.Compra], error) {
	res, err := paginar(ctx, r.q, "compras", comprasCount, comprasSelect+ordemCompra(o), []any{usuarioID}, p, scanCompra)
	if err != nil || len(res.Itens) == 0 {
		return res, err
	}
	if err := r.nomesDosProdutos(ctx, res.Itens); err != nil {
		return kernel.Resultado[vendas.Compra]{}, err
	}
	vlog.Printf(arqPedidos, "Pedidos.ComprasDoUsuario", "SELECT pedidos linhas=%d total=%d", len(res.Itens), res.Total)
	return res, nil
}

func scanCompra(s db.Linha) (*vendas.Compra, error) {
	var c vendas.Compra
	if err := s.Scan(&c.PedidoID, &c.Data, &c.Status, &c.ValorTotal); err != nil {
		return nil, fmt.Errorf("vendasmysql: scan compras: %w", err)
	}
	return &c, nil
}

// nomesDosProdutos preenche Produtos de cada compra, na ordem dos itens,
// sem repetir o mesmo produto no pedido.
func (r *Pedidos) nomesDosProdutos(ctx context.Context, compras []vendas.Compra) error {
	pos := make(map[int64]int, len(compras))
	args := make([]any, len(compras))
	for i, c := range compras {
		pos[c.PedidoID] = i
		args[i] = c.PedidoID
		compras[i].Produtos = make([]string, 0)
	}
	q := `SELECT i.pedido_id, i.produto_id, pr.nome FROM itens_pedido i JOIN produtos pr ON pr.id = i.produto_id
	WHERE i.pedido_id IN (?` + strings.Repeat(", ?", len(compras)-1) + `) ORDER BY i.pedido_id, i.item_id_origem`
	type linha struct {
		pedido, produto int64
		nome            string
	}
	linhas, err := db.Consultar(ctx, r.q, "vendasmysql: list produtos das compras", q, args, func(s db.Linha) (*linha, error) {
		var l linha
		if err := s.Scan(&l.pedido, &l.produto, &l.nome); err != nil {
			return nil, fmt.Errorf("vendasmysql: scan produtos das compras: %w", err)
		}
		return &l, nil
	})
	if err != nil {
		return err
	}
	vistos := make(map[[2]int64]bool, len(linhas))
	for _, l := range linhas {
		i, ok := pos[l.pedido]
		if !ok || vistos[[2]int64{l.pedido, l.produto}] {
			continue
		}
		vistos[[2]int64{l.pedido, l.produto}] = true
		compras[i].Produtos = append(compras[i].Produtos, l.nome)
	}
	return nil
}

// CompraDoUsuario abre um pedido do login (ECOM-16): cabeçalho, itens e
// pagamentos. Pedido de outra pessoa ou inexistente: ErrPedidoNaoEncontrado
// (o mesmo 404, sem revelar se o id existe).
func (r *Pedidos) CompraDoUsuario(ctx context.Context, usuarioID, pedidoID int64) (*vendas.CompraDetalhe, error) {
	const cab = `SELECT p.pedido_id_origem, p.data_pedido, p.status, p.valor_total` + comprasFrom + ` AND p.pedido_id_origem = ? LIMIT 1`
	var d vendas.CompraDetalhe
	err := r.q.QueryRowContext(ctx, cab, usuarioID, pedidoID).Scan(&d.PedidoID, &d.Data, &d.Status, &d.ValorTotal)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, vendas.ErrPedidoNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("vendasmysql: get compra: %w", err)
	}
	const itensQ = `SELECT pr.id, pr.sku, pr.nome, COALESCE(pr.imagem, ''), pr.ativo, i.quantidade, i.preco_praticado, i.desconto_pct, i.valor_bruto
	FROM itens_pedido i JOIN produtos pr ON pr.id = i.produto_id WHERE i.pedido_id = ? ORDER BY i.item_id_origem`
	d.Itens, err = db.Consultar(ctx, r.q, "vendasmysql: list itens da compra", itensQ, []any{pedidoID}, func(s db.Linha) (*vendas.ItemComprado, error) {
		var it vendas.ItemComprado
		if err := s.Scan(&it.ProdutoID, &it.SKU, &it.Nome, &it.Imagem, &it.Ativo, &it.Quantidade, &it.PrecoPraticado, &it.DescontoPct, &it.ValorBruto); err != nil {
			return nil, fmt.Errorf("vendasmysql: scan itens da compra: %w", err)
		}
		return &it, nil
	})
	if err != nil {
		return nil, err
	}
	const pagQ = `SELECT forma_pagamento, parcelas, valor, data_vencimento, data_pagamento, status_pagamento
	FROM pagamentos WHERE pedido_id = ? ORDER BY pagamento_id`
	d.Pagamentos, err = db.Consultar(ctx, r.q, "vendasmysql: list pagamentos da compra", pagQ, []any{pedidoID}, func(s db.Linha) (*vendas.PagamentoComprado, error) {
		var pg vendas.PagamentoComprado
		var pago sql.NullTime
		if err := s.Scan(&pg.Forma, &pg.Parcelas, &pg.Valor, &pg.DataVencimento, &pago, &pg.Status); err != nil {
			return nil, fmt.Errorf("vendasmysql: scan pagamentos da compra: %w", err)
		}
		if pago.Valid {
			pg.DataPagamento = &pago.Time
		}
		return &pg, nil
	})
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqPedidos, "Pedidos.CompraDoUsuario", "SELECT pedido=%d itens=%d pagamentos=%d", pedidoID, len(d.Itens), len(d.Pagamentos))
	return &d, nil
}
