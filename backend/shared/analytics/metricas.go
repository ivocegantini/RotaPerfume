package analytics

import "github.com/rotaperfumes/shared/kernel"

// Metricas é o read model de GET /api/dashboard/metrics: os números de
// vendas do período, o top 10 de vendedores, as metas do mês e o aviso de
// vendedor desligado (D-01). As listas nunca são nil (o contrato é [],
// nunca null).
type Metricas struct {
	Periodo           Periodo
	TotalVendas       kernel.Dinheiro
	TotalVendasQtd    int
	TotalPedidos      int
	TicketMedio       float64
	TopVendedores     []VendedorRanking
	MetasVendedores   []MetaVendedor
	MetaMes           kernel.Dinheiro
	VendedorDesligado bool
}

// TotaisVendas são os agregados de vendas do período (pedidos que não
// estão cancelados nem devolvidos).
type TotaisVendas struct {
	Valor      kernel.Dinheiro
	Quantidade int
}

// NovasMetricas monta as métricas do escopo. O ticket médio é derivado dos
// totais já filtrados pelo escopo (0 sem vendas). Nunca sinaliza
// desligamento: quem tem dados não está bloqueado. O ticket é estatística
// (float64); os totais seguem exatos em centavos (ARQ-09).
func NovasMetricas(p Periodo, t TotaisVendas, totalPedidos int, top []VendedorRanking, metas []MetaVendedor, metaMes kernel.Dinheiro) Metricas {
	var ticket float64
	if t.Quantidade > 0 {
		ticket = t.Valor.Reais() / float64(t.Quantidade)
	}
	return Metricas{
		Periodo:         p,
		TotalVendas:     t.Valor,
		TotalVendasQtd:  t.Quantidade,
		TotalPedidos:    totalPedidos,
		TicketMedio:     ticket,
		TopVendedores:   naoNulo(top),
		MetasVendedores: naoNulo(metas),
		MetaMes:         metaMes,
	}
}

// MetricasZeradas é a resposta de quem não tem acesso (D-01): o mesmo
// formato, tudo zerado, listas vazias e o aviso de desligamento.
func MetricasZeradas(p Periodo, vendedorDesligado bool) Metricas {
	m := NovasMetricas(p, TotaisVendas{}, 0, nil, nil, kernel.Dinheiro{})
	m.VendedorDesligado = vendedorDesligado
	return m
}

// VendasDoMes são as vendas do mês corrente de um vendedor (total exato em
// centavos, ARQ-09).
type VendasDoMes struct {
	Total      kernel.Dinheiro
	Quantidade int
}

// VendedorRanking é uma linha do top de vendedores (por meta).
type VendedorRanking struct {
	ID               int64
	Nome             string
	TotalVendas      kernel.Dinheiro
	Meta             kernel.Dinheiro
	PercentualMeta   float64
	QuantidadeVendas int
}

// AplicarVendas registra as vendas do mês e o percentual da meta (sem meta,
// o percentual fica 0).
func (v *VendedorRanking) AplicarVendas(vm VendasDoMes) {
	v.TotalVendas = vm.Total
	v.QuantidadeVendas = vm.Quantidade
	v.PercentualMeta = percentual(v.TotalVendas, v.Meta, v.PercentualMeta)
}

// MetaVendedor é a meta de um vendedor ativo comparada com o realizado.
type MetaVendedor struct {
	ID               int64
	Nome             string
	Regiao           string
	UF               string
	Meta             kernel.Dinheiro
	Realizado        kernel.Dinheiro
	Percentual       float64
	QuantidadeVendas int
}

// AplicarVendas registra o realizado do mês e o percentual da meta (sem
// meta, o percentual fica 0).
func (m *MetaVendedor) AplicarVendas(vm VendasDoMes) {
	m.Realizado = vm.Total
	m.QuantidadeVendas = vm.Quantidade
	m.Percentual = percentual(m.Realizado, m.Meta, m.Percentual)
}

// percentual é (valor/meta)*100 quando há meta; senão mantém o atual. É
// estatística (float64), com os mesmos operandos de antes do ARQ-09.
func percentual(valor, meta kernel.Dinheiro, atual float64) float64 {
	if meta.Centavos() > 0 {
		return (valor.Reais() / meta.Reais()) * 100
	}
	return atual
}

// naoNulo troca nil por uma lista vazia (o contrato serializa []).
func naoNulo[T any](v []T) []T {
	if v == nil {
		return make([]T, 0)
	}
	return v
}
