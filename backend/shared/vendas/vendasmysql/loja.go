package vendasmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqLoja = "loja.go"

// uqPedidosChaveCheckout é o UNIQUE de sql/35 (ECOM-14).
const uqPedidosChaveCheckout = "uq_pedidos_chave_checkout"

// nomeVendedorLoja é o vendedor do canal Loja (sql/33).
const nomeVendedorLoja = "Loja Online"

// Loja é o adapter das leituras do checkout (porta vendasapp.LojaNaTx),
// sempre ligado ao *sql.Tx do UoW do pedido.
type Loja struct{ q db.Querier }

// NovaLoja liga o adapter à tx. q nil: panic.
func NovaLoja(q db.Querier) *Loja {
	if q == nil {
		panic("vendasmysql: NovaLoja exige Querier")
	}
	return &Loja{q: q}
}

// ClienteDoUsuario trava o cliente do login (clientes.usuario_id, UNIQUE).
func (r *Loja) ClienteDoUsuario(ctx context.Context, usuarioID int64) (int64, error) {
	var id int64
	err := r.q.QueryRowContext(ctx, `SELECT cliente_id_origem FROM clientes WHERE usuario_id = ? FOR UPDATE`, usuarioID).Scan(&id)
	vlog.Printf(arqLoja, "Loja.ClienteDoUsuario", "SELECT clientes FOR UPDATE encontrado=%t", err == nil)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, vendas.ErrCheckoutSemCliente
	}
	if err != nil {
		return 0, fmt.Errorf("vendasmysql: cliente do usuario: %w", err)
	}
	return id, nil
}

// VendedorLoja lê o id do vendedor "Loja Online" (o menor, se houver dois).
func (r *Loja) VendedorLoja(ctx context.Context) (int64, error) {
	var id int64
	err := r.q.QueryRowContext(ctx, `SELECT id FROM vendedores WHERE nome = ? ORDER BY id LIMIT 1`, nomeVendedorLoja).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, vendas.ErrVendedorLojaAusente
	}
	if err != nil {
		return 0, fmt.Errorf("vendasmysql: vendedor da loja: %w", err)
	}
	return id, nil
}

// SQL das ofertas (SEC-79), todas constantes (TST-07). Leituras com lock
// enxergam o último commit (não o snapshot da tx): com o produto travado, o
// segundo checkout do mesmo produto espera o primeiro e vê a reserva dele.
const (
	sqlProdutoAtivoTravado = `SELECT preco_tabela, sku FROM produtos WHERE id = ? AND ativo = 1 FOR UPDATE`
	sqlUltimoSaldo         = `SELECT saldo FROM estoque WHERE sku = ? ORDER BY data_snapshot DESC, id DESC LIMIT 1 LOCK IN SHARE MODE`
	sqlReservadoNaLoja     = `SELECT COALESCE(SUM(i.quantidade), 0) FROM itens_pedido i JOIN pedidos p ON p.pedido_id_origem = i.pedido_id
		WHERE i.produto_id = ? AND p.canal = 'Loja' AND p.status = 'Em separação' LOCK IN SHARE MODE`
)

// OfertasAtivas trava cada produto ativo (ids em ordem crescente: dois
// carrinhos com os mesmos produtos não se travam em ciclo) e calcula o
// disponível = saldo do último snapshot (sem snapshot: 0) − reservado em
// pedidos da Loja Em separação (ainda sem baixa de estoque). Nunca negativo.
func (r *Loja) OfertasAtivas(ctx context.Context, ids []int64) (map[int64]vendas.OfertaLoja, error) {
	ordenados := slices.Clone(ids)
	slices.Sort(ordenados)
	ofertas := make(map[int64]vendas.OfertaLoja, len(ids))
	for _, id := range ordenados {
		var o vendas.OfertaLoja
		var sku string
		err := r.q.QueryRowContext(ctx, sqlProdutoAtivoTravado, id).Scan(&o.Preco, &sku)
		if errors.Is(err, sql.ErrNoRows) {
			continue // inativo ou inexistente: fica fora do mapa
		}
		if err != nil {
			return nil, fmt.Errorf("vendasmysql: produto da loja: %w", err)
		}
		var saldo, reservado int
		if err = r.q.QueryRowContext(ctx, sqlUltimoSaldo, sku).Scan(&saldo); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("vendasmysql: saldo da loja: %w", err)
		}
		if err = r.q.QueryRowContext(ctx, sqlReservadoNaLoja, id).Scan(&reservado); err != nil {
			return nil, fmt.Errorf("vendasmysql: reservado na loja: %w", err)
		}
		o.Disponivel = max(saldo-reservado, 0)
		ofertas[id] = o
	}
	vlog.Printf(arqLoja, "Loja.OfertasAtivas", "SELECT produtos FOR UPDATE pedidos=%d ativos=%d", len(ids), len(ofertas))
	return ofertas, nil
}

// PedidoPorChave lê o pedido e o cliente da chave do checkout.
func (r *Loja) PedidoPorChave(ctx context.Context, chave string) (pedidoID, clienteID int64, achou bool, err error) {
	err = r.q.QueryRowContext(ctx, `SELECT pedido_id_origem, cliente_id FROM pedidos WHERE chave_checkout = ?`, chave).Scan(&pedidoID, &clienteID)
	vlog.Printf(arqLoja, "Loja.PedidoPorChave", "SELECT pedidos encontrado=%t", err == nil)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("vendasmysql: pedido por chave: %w", err)
	}
	return pedidoID, clienteID, true, nil
}

// GravarChave grava a chave do checkout no pedido recém-criado.
func (r *Loja) GravarChave(ctx context.Context, pedidoID int64, chave string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE pedidos SET chave_checkout = ? WHERE pedido_id_origem = ?`, chave, pedidoID)
	if db.ChaveDuplicada(err, uqPedidosChaveCheckout) {
		return vendas.ErrCheckoutChaveEmUso
	}
	if err != nil {
		return wrap("update pedidos chave_checkout", err)
	}
	vlog.Printf(arqLoja, "Loja.GravarChave", "UPDATE pedidos id=%d", pedidoID)
	return nil
}

// PagamentoDoPedido lê o primeiro pagamento do pedido (nil sem linha).
func (r *Loja) PagamentoDoPedido(ctx context.Context, pedidoID int64) (*vendas.Pagamento, error) {
	q := "SELECT " + pagamentoColunas + " FROM pagamentos WHERE pedido_id = ? ORDER BY pagamento_id LIMIT 1"
	p, err := scanPagamento(r.q.QueryRowContext(ctx, q, pedidoID))
	if errors.Is(err, vendas.ErrPagamentoNaoEncontrado) {
		return nil, nil
	}
	return p, err
}
