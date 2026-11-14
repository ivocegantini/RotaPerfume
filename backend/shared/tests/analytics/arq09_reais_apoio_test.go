package analytics_test

import "github.com/rotaperfumes/shared/kernel"

// reaisT converte o literal de teste em Dinheiro exato (ARQ-09); literal
// inválido é erro do próprio teste.
func reaisT(s string) kernel.Dinheiro {
	d, err := kernel.LerDinheiro(s)
	if err != nil {
		panic("reaisT(" + s + "): " + err.Error())
	}
	return d
}
