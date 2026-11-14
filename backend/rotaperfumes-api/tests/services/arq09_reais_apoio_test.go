package services_test

import (
	"strconv"

	"github.com/rotaperfumes/shared/kernel"
)

// reaisT converte o literal de teste em Dinheiro exato (ARQ-09); literal
// inválido é erro do próprio teste.
func reaisT(s string) kernel.Dinheiro {
	d, err := kernel.LerDinheiro(s)
	if err != nil {
		panic("reaisT(" + s + "): " + err.Error())
	}
	return d
}

// decT simula o driver: DECIMAL chega como texto, nunca float64 (ARQ-09).
func decT(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
