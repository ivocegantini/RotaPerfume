package vendasapp

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqCheckout = "checkout.go"

// FinalizarCompra fecha o carrinho da loja (ECOM-14): o cliente vem só do
// token; preços e total, do banco. Mesma chave → o mesmo pedido (Repetida).
// Pedido "Em separação" + pagamento "Em aberto" numa única tx.
func (c *Casos) FinalizarCompra(ctx context.Context, p kernel.Principal, ler Leitor[vendas.DadosCheckout]) (res *vendas.CompraLoja, err error) {
	defer vlog.Trace(arqCheckout, "Vendas.FinalizarCompra", "user_id=%d", p.UserID())(&err)
	if !p.Valido() {
		return nil, vendas.ErrCheckoutSemCliente
	}
	d, err := ler()
	if err != nil {
		return nil, err
	}
	v, err := d.Validar()
	if err != nil {
		return nil, err
	}
	quando := c.d.Relogio()
	err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error {
		res, err = r.FinalizarCompra(ctx, p.UserID(), v, quando)
		return err
	})
	if err != nil {
		return nil, err
	}
	if res.Pedido, err = c.detalhe(ctx, res.Pedido.PedidoIDOrigem); err != nil {
		return nil, err
	}
	log.Printf("[loja] compra finalizada: pedido_id=%d usuario_id=%d itens=%d repetida=%t",
		res.Pedido.PedidoIDOrigem, p.UserID(), len(res.Pedido.Itens), res.Repetida)
	return res, nil
}

// FinalizarCompra é o passo na tx: trava o cliente do login, devolve o
// pedido da chave se já existir, senão lê vendedor e preços, monta pelo
// domínio e grava pedido, chave e pagamento.
func (r TxPedido) FinalizarCompra(ctx context.Context, usuarioID int64, v vendas.CheckoutValido, quando time.Time) (*vendas.CompraLoja, error) {
	if r.Loja == nil {
		return nil, errors.New("vendasapp: TxPedido sem a porta Loja")
	}
	clienteID, err := r.Loja.ClienteDoUsuario(ctx, usuarioID)
	if err != nil {
		return nil, err
	}
	pedidoID, dono, achou, err := r.Loja.PedidoPorChave(ctx, v.Chave)
	if err != nil {
		return nil, err
	}
	if achou {
		if dono != clienteID {
			vlog.Printf(arqCheckout, "TxPedido.FinalizarCompra", "decisão chave_de_outro_cliente")
			return nil, vendas.ErrCheckoutChaveEmUso
		}
		pag, err := r.Loja.PagamentoDoPedido(ctx, pedidoID)
		if err != nil {
			return nil, err
		}
		vlog.Printf(arqCheckout, "TxPedido.FinalizarCompra", "decisão repetida pedido_id=%d", pedidoID)
		return &vendas.CompraLoja{Pedido: &vendas.Pedido{PedidoIDOrigem: pedidoID}, Pagamento: pag, Repetida: true}, nil
	}
	vendedorID, err := r.Loja.VendedorLoja(ctx)
	if err != nil {
		return nil, err
	}
	ofertas, err := r.Loja.OfertasAtivas(ctx, v.ProdutoIDs())
	if err != nil {
		return nil, err
	}
	ped, pag, err := vendas.MontarCompraLoja(v, clienteID, vendedorID, ofertas, quando)
	if err != nil {
		vlog.Printf(arqCheckout, "TxPedido.FinalizarCompra", "decisão recusado sem_estoque=%t ativos=%d itens=%d",
			errors.Is(err, vendas.ErrCheckoutSemEstoque), len(ofertas), len(v.Itens))
		return nil, err
	}
	if err = r.Pedidos.Inserir(ctx, ped); err != nil {
		return nil, err
	}
	if err = r.Loja.GravarChave(ctx, ped.PedidoIDOrigem, v.Chave); err != nil {
		return nil, err
	}
	pag.PedidoID = ped.PedidoIDOrigem
	if err = r.Pagamentos.Inserir(ctx, pag); err != nil {
		return nil, err
	}
	return &vendas.CompraLoja{Pedido: ped, Pagamento: pag}, nil
}
