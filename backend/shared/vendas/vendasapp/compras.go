package vendasapp

import (
	"context"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqCompras = "compras.go"

// MinhasCompras pagina os produtos comprados pelo usuário logado (ECOM-12).
// O usuário vem só do token (nunca da query); Principal inválido devolve
// lista vazia sem SQL. Login sem cliente da loja (equipe) também fica vazio.
func (c *Casos) MinhasCompras(ctx context.Context, p kernel.Principal, o kernel.Ordenacao[vendas.CampoCompra], pg kernel.Pagina) (r kernel.Resultado[vendas.ProdutoComprado], err error) {
	defer vlog.Trace(arqCompras, "Vendas.MinhasCompras", "user_id=%d page=%d limit=%d ordem=%s", p.UserID(), pg.Numero(), pg.Limite(), o.Campo())(&err)
	if !p.Valido() {
		return vazio[vendas.ProdutoComprado](pg), nil
	}
	r, err = c.d.Pedidos.ComprasDoUsuario(ctx, p.UserID(), o, pg)
	if err == nil && r.Itens == nil {
		r.Itens = make([]vendas.ProdutoComprado, 0) // "data": [] em vez de null
	}
	return r, err
}
