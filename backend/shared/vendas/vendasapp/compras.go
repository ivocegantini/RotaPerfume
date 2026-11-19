package vendasapp

import (
	"context"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqCompras = "compras.go"

// MinhasCompras pagina os pedidos do usuário logado (ECOM-12/16).
// O usuário vem só do token (nunca da query); Principal inválido devolve
// lista vazia sem SQL. Login sem cliente da loja (equipe) também fica vazio.
func (c *Casos) MinhasCompras(ctx context.Context, p kernel.Principal, o kernel.Ordenacao[vendas.CampoCompra], pg kernel.Pagina) (r kernel.Resultado[vendas.Compra], err error) {
	defer vlog.Trace(arqCompras, "Vendas.MinhasCompras", "user_id=%d page=%d limit=%d ordem=%s", p.UserID(), pg.Numero(), pg.Limite(), o.Campo())(&err)
	if !p.Valido() {
		return vazio[vendas.Compra](pg), nil
	}
	r, err = c.d.Pedidos.ComprasDoUsuario(ctx, p.UserID(), o, pg)
	if err == nil && r.Itens == nil {
		r.Itens = make([]vendas.Compra, 0) // "data": [] em vez de null
	}
	return r, err
}

// MinhaCompra abre um pedido do usuário logado (ECOM-16): itens e
// pagamentos. Pedido de outro login, inexistente ou Principal inválido:
// ErrPedidoNaoEncontrado (404 igual; o inválido nem chega ao SQL).
func (c *Casos) MinhaCompra(ctx context.Context, p kernel.Principal, pedidoID int64) (d *vendas.CompraDetalhe, err error) {
	defer vlog.Trace(arqCompras, "Vendas.MinhaCompra", "user_id=%d pedido=%d", p.UserID(), pedidoID)(&err)
	if !p.Valido() || pedidoID <= 0 {
		return nil, vendas.ErrPedidoNaoEncontrado
	}
	d, err = c.d.Pedidos.CompraDoUsuario(ctx, p.UserID(), pedidoID)
	if err == nil {
		if d.Itens == nil {
			d.Itens = make([]vendas.ItemComprado, 0)
		}
		if d.Pagamentos == nil {
			d.Pagamentos = make([]vendas.PagamentoComprado, 0)
		}
	}
	return d, err
}
