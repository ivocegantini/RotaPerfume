package catalogomysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// ARQ-05: as escritas de estoque que eram do repositories.EstoqueRepository
// legado. A baixa por faturamento é consumida por Vendas pela porta
// vendasapp.BaixaEstoque, com o Estoques ligado ao MESMO *sql.Tx do pedido
// (bootstrap): faturar o pedido e baixar o estoque são atômicos (§5.2). O
// SQL é o do legado, salvo a ordem das atribuições do upsert (BUG-21).

const arqFaturamento = "faturamento.go"

// errProdutoDaBaixa: o item faturado aponta para um produto inexistente
// (inconsistência; o FK garante o produto). É erro de infraestrutura (500),
// nunca o 404 de catalogo.ErrProdutoNaoEncontrado, como no legado.
var errProdutoDaBaixa = errors.New("catalogomysql: produto do item não encontrado para baixa de estoque")

// SKUDoProduto devolve o sku do produto (a tabela estoque referencia
// produtos por sku, não por id).
func (r *Estoques) SKUDoProduto(ctx context.Context, produtoID int64) (string, error) {
	var sku string
	err := r.q.QueryRowContext(ctx, `SELECT sku FROM produtos WHERE id = ? LIMIT 1`, produtoID).Scan(&sku)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errProdutoDaBaixa
	}
	if err != nil {
		return "", fmt.Errorf("catalogomysql: sku por produto_id: %w", db.SemValorDuplicado(err))
	}
	vlog.Printf(arqFaturamento, "Estoques.SKUDoProduto", "SELECT produtos linhas=1")
	return sku, nil
}

// SQL da baixa por faturamento (BUG-32), todas constantes.
const (
	// sqlTravarProduto: a linha do produto é o mutex da baixa (e do checkout,
	// SEC-79). Travar só o último snapshot não basta: o 2º faturamento que
	// esperava o lock seguiria com a linha antiga e colidiria no UNIQUE.
	sqlTravarProduto = `SELECT id FROM produtos WHERE sku = ? FOR UPDATE`
	// sqlUltimoSnapshotTravado: a última posição do sku até o dia, lida
	// DEPOIS do lock do produto (leitura com lock: vê o último commit).
	sqlUltimoSnapshotTravado  = `SELECT id, data_snapshot, saldo FROM estoque WHERE sku = ? AND data_snapshot <= ? ORDER BY data_snapshot DESC, id DESC LIMIT 1 FOR UPDATE`
	sqlAtualizarSnapshotDoDia = `UPDATE estoque SET saldo = ?, ruptura = ? WHERE id = ?`
	sqlCriarSnapshotDoDia     = `INSERT INTO estoque (data_snapshot, sku, saldo, ruptura) VALUES (?, ?, ?, ?)`
)

// AjustarSaldoPorFaturamento dá baixa de delta unidades do sku no dia, para
// uso EXCLUSIVO do faturamento de pedidos (BUG-32, decisão do usuário):
// parte do último saldo até o dia (sem snapshot: 0); o saldo novo nunca é
// negativo (saldo < delta → catalogo.ErrEstoqueInsuficienteFaturamento, 409,
// e a tx do pedido desfaz tudo). Último snapshot no próprio dia → UPDATE;
// anterior → cria o dia com o saldo carregado menos a baixa. ruptura =
// saldo resultante <= 0. Tem de rodar na tx do pedido.
func (r *Estoques) AjustarSaldoPorFaturamento(ctx context.Context, sku string, dia time.Time, delta int) error {
	data := dia.Format(kernel.LayoutData)
	var produtoID int64
	err := r.q.QueryRowContext(ctx, sqlTravarProduto, sku).Scan(&produtoID)
	if errors.Is(err, sql.ErrNoRows) {
		return errProdutoDaBaixa
	}
	if err != nil {
		return fmt.Errorf("catalogomysql: lock produto para ajuste de faturamento: %w", db.SemValorDuplicado(err))
	}
	var id int64
	var ultimo time.Time
	var saldo int
	err = r.q.QueryRowContext(ctx, sqlUltimoSnapshotTravado, sku, data).Scan(&id, &ultimo, &saldo)
	semSnapshot := errors.Is(err, sql.ErrNoRows)
	if err != nil && !semSnapshot {
		return fmt.Errorf("catalogomysql: lock estoque para ajuste de faturamento: %w", db.SemValorDuplicado(err))
	}
	novo := saldo - delta
	if novo < 0 {
		vlog.Printf(arqFaturamento, "Estoques.AjustarSaldoPorFaturamento", "decisão estoque_insuficiente delta=%d", delta)
		return catalogo.ErrEstoqueInsuficienteFaturamento
	}
	if !semSnapshot && ultimo.Format(kernel.LayoutData) == data {
		_, err = r.q.ExecContext(ctx, sqlAtualizarSnapshotDoDia, novo, novo <= 0, id)
	} else {
		_, err = r.q.ExecContext(ctx, sqlCriarSnapshotDoDia, data, sku, novo, novo <= 0)
	}
	if err != nil {
		return fmt.Errorf("catalogomysql: ajustar saldo estoque por faturamento: %w", db.SemValorDuplicado(err))
	}
	vlog.Printf(arqFaturamento, "Estoques.AjustarSaldoPorFaturamento", "baixa estoque delta=%d dia_existente=%t", delta, !semSnapshot && ultimo.Format(kernel.LayoutData) == data)
	return nil
}
