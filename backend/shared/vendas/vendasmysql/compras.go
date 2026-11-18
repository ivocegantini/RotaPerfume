package vendasmysql

import (
	"context"
	"fmt"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

// comprasFrom liga o login ao cliente da loja (clientes.usuario_id, UNIQUE) e
// aos itens dos pedidos dele; pedido Cancelado não conta como compra.
const comprasFrom = ` FROM itens_pedido i
	JOIN pedidos p ON p.pedido_id_origem = i.pedido_id
	JOIN clientes c ON c.cliente_id_origem = p.cliente_id
	JOIN produtos pr ON pr.id = i.produto_id
	WHERE c.usuario_id = ? AND p.status <> 'Cancelado'`

const comprasCount = `SELECT COUNT(DISTINCT i.produto_id)` + comprasFrom

const comprasSelect = `SELECT pr.id, pr.sku, pr.nome, COALESCE(pr.imagem, ''), pr.ativo,
	SUM(i.quantidade) AS quantidade, COUNT(DISTINCT p.pedido_id_origem) AS pedidos, SUM(i.valor_bruto) AS total_gasto,
	MAX(p.data_pedido) AS ultima_compra` + comprasFrom + `
	GROUP BY pr.id, pr.sku, pr.nome, pr.imagem, pr.ativo`

// ordemCompra monta o ORDER BY pela whitelist; pr.id desempata (paginação estável).
func ordemCompra(o kernel.Ordenacao[vendas.CampoCompra]) string {
	dir := db.DirecaoSQL(o.Direcao(), "DESC")
	return " ORDER BY " + colunaCompra(o.Campo()) + " " + dir + ", pr.id " + dir + " LIMIT ? OFFSET ?"
}

// colunaCompra é a whitelist: enum → alias/coluna.
func colunaCompra(c vendas.CampoCompra) string {
	switch c {
	case vendas.OrdemCompraNome:
		return "pr.nome"
	case vendas.OrdemCompraQuantidade:
		return "quantidade"
	case vendas.OrdemCompraPedidos:
		return "pedidos"
	case vendas.OrdemCompraTotal:
		return "total_gasto"
	default:
		return "ultima_compra"
	}
}

// ComprasDoUsuario pagina os produtos comprados pelo login (ECOM-12), na
// ordenação pedida. Login sem cliente da loja: lista vazia.
func (r *Pedidos) ComprasDoUsuario(ctx context.Context, usuarioID int64, o kernel.Ordenacao[vendas.CampoCompra], p kernel.Pagina) (kernel.Resultado[vendas.ProdutoComprado], error) {
	res, err := paginar(ctx, r.q, "compras", comprasCount, comprasSelect+ordemCompra(o), []any{usuarioID}, p, scanComprado)
	if err != nil {
		return res, err
	}
	vlog.Printf(arqPedidos, "Pedidos.ComprasDoUsuario", "SELECT itens_pedido agrupado linhas=%d total=%d", len(res.Itens), res.Total)
	return res, nil
}

func scanComprado(s db.Linha) (*vendas.ProdutoComprado, error) {
	var c vendas.ProdutoComprado
	if err := s.Scan(&c.ProdutoID, &c.SKU, &c.Nome, &c.Imagem, &c.Ativo, &c.Quantidade, &c.Pedidos, &c.TotalGasto, &c.UltimaCompra); err != nil {
		return nil, fmt.Errorf("vendasmysql: scan compras: %w", err)
	}
	return &c, nil
}
