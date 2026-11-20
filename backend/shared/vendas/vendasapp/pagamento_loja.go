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

const arqPagamentoLoja = "pagamento_loja.go"

// ConfirmarPagamentoLoja aplica o resultado do pagamento de um pedido da loja
// (ECOM-18): aprovado → Em separação; recusado → Cancelado. Hoje quem chama é
// o simulador de dev/qa; o webhook do gateway (ECOM-15) usará o mesmo caso.
// Só o admin ou o dono do pedido; de outro login → 404 (não revela o pedido).
func (c *Casos) ConfirmarPagamentoLoja(ctx context.Context, p kernel.Principal, pedidoID int64, ler Leitor[string]) (res *vendas.CompraLoja, err error) {
	defer vlog.Trace(arqPagamentoLoja, "Vendas.ConfirmarPagamentoLoja", "user_id=%d pedido=%d", p.UserID(), pedidoID)(&err)
	if !p.Valido() || pedidoID <= 0 {
		return nil, vendas.ErrPedidoNaoEncontrado
	}
	bruto, err := ler()
	if err != nil {
		return nil, err
	}
	resultado, err := vendas.NovoResultadoPagamento(bruto)
	if err != nil {
		return nil, err
	}
	quando := c.d.Relogio()
	var pag *vendas.Pagamento
	err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error {
		pag, err = r.ConfirmarPagamentoLoja(ctx, p, pedidoID, resultado, quando)
		return err
	})
	if err != nil {
		return nil, err
	}
	ped, err := c.detalhe(ctx, pedidoID)
	if err != nil {
		return nil, err
	}
	log.Printf("[loja] pagamento %s: pedido_id=%d status=%s por user_id=%d", resultado, pedidoID, ped.Status, p.UserID())
	return &vendas.CompraLoja{Pedido: ped, Pagamento: pag}, nil
}

// ConfirmarPagamentoLoja é o passo na tx: trava pedido e pagamento, confere
// a posse, decide pelo domínio e grava os dois status.
func (r TxPedido) ConfirmarPagamentoLoja(ctx context.Context, p kernel.Principal, pedidoID int64, res vendas.ResultadoPagamento, quando time.Time) (*vendas.Pagamento, error) {
	if r.Loja == nil {
		return nil, errors.New("vendasapp: TxPedido sem a porta Loja")
	}
	ped, err := r.Loja.TravarPedidoLoja(ctx, pedidoID)
	if err != nil {
		return nil, err
	}
	if !p.EhAdmin() && ped.UsuarioID != p.UserID() {
		vlog.Printf(arqPagamentoLoja, "TxPedido.ConfirmarPagamentoLoja", "decisão pedido_de_outro_login id=%d", pedidoID)
		return nil, vendas.ErrPedidoNaoEncontrado
	}
	pag, err := r.Loja.TravarPagamentoDoPedido(ctx, pedidoID)
	if err != nil {
		return nil, err
	}
	status, err := vendas.AplicarPagamentoLoja(ped, pag, res, quando)
	if err != nil {
		vlog.Printf(arqPagamentoLoja, "TxPedido.ConfirmarPagamentoLoja", "decisão não_pendente id=%d canal=%s status=%s", pedidoID, ped.Canal, ped.Status)
		return nil, err
	}
	if err = r.Loja.GravarPagamentoLoja(ctx, pedidoID, status, pag); err != nil {
		return nil, err
	}
	return pag, nil
}

// ExpirarPedidosLoja (ECOM-19) cancela os pedidos da loja cujo pagamento
// venceu sem confirmação: cada um na sua tx, travado e conferido de novo
// (pode ter sido pago entre a leitura e o lock). Devolve quantos cancelou.
func (c *Casos) ExpirarPedidosLoja(ctx context.Context) (n int, err error) {
	defer vlog.Trace(arqPagamentoLoja, "Vendas.ExpirarPedidosLoja", "")(&err)
	hoje := c.d.Relogio()
	ids, err := c.d.Pedidos.LojaVencidos(ctx, hoje, vendas.MaxExpiracoesPorRodada)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return n, ctx.Err()
		}
		err = c.d.TxPedido.WithTx(ctx, func(r TxPedido) error { return r.ExpirarPedidoLoja(ctx, id, hoje) })
		switch {
		case err == nil:
			n++
			log.Printf("[loja] pedido expirado sem pagamento: pedido_id=%d → Cancelado", id)
		case errors.Is(err, vendas.ErrPagamentoNaoPendente), errors.Is(err, vendas.ErrPagamentoNaoVencido):
			vlog.Printf(arqPagamentoLoja, "Vendas.ExpirarPedidosLoja", "decisão ignorado id=%d (mudou antes do lock)", id)
		default:
			log.Printf("[loja] expiração: pedido_id=%d: %v", id, err)
		}
	}
	return n, nil
}

// ExpirarPedidoLoja é o passo na tx de um pedido vencido.
func (r TxPedido) ExpirarPedidoLoja(ctx context.Context, pedidoID int64, hoje time.Time) error {
	if r.Loja == nil {
		return errors.New("vendasapp: TxPedido sem a porta Loja")
	}
	ped, err := r.Loja.TravarPedidoLoja(ctx, pedidoID)
	if err != nil {
		return err
	}
	pag, err := r.Loja.TravarPagamentoDoPedido(ctx, pedidoID)
	if err != nil {
		return err
	}
	status, err := vendas.ExpirarPagamentoLoja(ped, pag, hoje)
	if err != nil {
		return err
	}
	return r.Loja.GravarPagamentoLoja(ctx, pedidoID, status, pag)
}
