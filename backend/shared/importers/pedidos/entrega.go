package pedidos

import (
	"strconv"

	"github.com/rotaperfumes/shared/vendas"
)

// ECOM-17: o CSV do ERP não traz endereço de entrega (obrigatório desde a
// sql/schema.sql). Cada pedido importado recebe o mesmo endereço fictício que a
// migração dá aos antigos: capital = (id*31) mod 10, número 1..1999.
var capitaisFicticias = [10]vendas.Entrega{
	{CEP: "01310100", Endereco: "Avenida Paulista", Bairro: "Bela Vista", Cidade: "São Paulo", UF: "SP"},
	{CEP: "22070011", Endereco: "Avenida Atlântica", Bairro: "Copacabana", Cidade: "Rio de Janeiro", UF: "RJ"},
	{CEP: "30130010", Endereco: "Avenida Afonso Pena", Bairro: "Centro", Cidade: "Belo Horizonte", UF: "MG"},
	{CEP: "80020310", Endereco: "Rua XV de Novembro", Bairro: "Centro", Cidade: "Curitiba", UF: "PR"},
	{CEP: "90020004", Endereco: "Rua dos Andradas", Bairro: "Centro Histórico", Cidade: "Porto Alegre", UF: "RS"},
	{CEP: "40060001", Endereco: "Avenida Sete de Setembro", Bairro: "Vitória", Cidade: "Salvador", UF: "BA"},
	{CEP: "50050000", Endereco: "Rua da Aurora", Bairro: "Boa Vista", Cidade: "Recife", UF: "PE"},
	{CEP: "60025100", Endereco: "Rua Major Facundo", Bairro: "Centro", Cidade: "Fortaleza", UF: "CE"},
	{CEP: "70070120", Endereco: "SBS Quadra 2", Bairro: "Asa Sul", Cidade: "Brasília", UF: "DF"},
	{CEP: "74015010", Endereco: "Avenida Goiás", Bairro: "Setor Central", Cidade: "Goiânia", UF: "GO"},
}

// EntregaFicticia devolve o endereço fictício do pedido (determinístico).
func EntregaFicticia(pedidoID int64) vendas.Entrega {
	e := capitaisFicticias[((pedidoID*31)%10+10)%10]
	e.Numero = strconv.FormatInt(1+((pedidoID*7919)%1999+1999)%1999, 10)
	return e
}
