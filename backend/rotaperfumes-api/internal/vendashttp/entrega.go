package vendashttp

import "github.com/rotaperfumes/shared/vendas"

// entregaJSON é o endereço de entrega (ECOM-17), igual na requisição e na
// resposta. cep sem máscara na resposta; complemento "" quando não houver.
type entregaJSON struct {
	CEP         string `json:"cep"`
	Endereco    string `json:"endereco"`
	Numero      string `json:"numero"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Cidade      string `json:"cidade"`
	UF          string `json:"uf"`
}

func novoEntregaJSON(e vendas.Entrega) entregaJSON { return entregaJSON(e) }

func dadosEntrega(e entregaJSON) vendas.DadosEntrega { return vendas.DadosEntrega(e) }
