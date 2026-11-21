package config_test

// SEC-56 (Lote 37): formas numéricas não canônicas do IPv4 (decimal, hex,
// curta, octal, zeros à esquerda) contam como produção no Makefile e no Go.
// SEC-57 (Lote 37): porta com espaço é inválida nos dois lados, com mensagem
// própria no Makefile (não a de "PRODUCAO"). Paridade pela mesma tabela.

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

func TestSEC56_IPv4NaoCanonicoEProducao_Paridade(t *testing.T) {
	casos := []struct {
		host, porta string
		producao    bool
	}{
		{"3232278634", "3306", true},          // decimal
		{"3232278634", "3307", true},          // em qualquer porta (fail-closed)
		{"0xc0a8a86a", "3306", true},          // hex
		{"0XC0A8A86A", "", true},              // hex maiúsculo
		{"192.168.43114", "3306", true},       // curta
		{"192.11053418", "3306", true},        // curta (2 partes)
		{"0300.0250.0250.0152", "3306", true}, // octal
		{"192.168.168.0106", "3306", true},    // zero à esquerda
		{"0xc0.0xa8.0xa8.0x6a", "3306", true}, // hex por octeto
		{"192.168.168.106.", "3306", true},    // canônico (LAN)
		{"192.168.168.256", "3306", true},     // octeto > 255
		{"2130706433", "3306", true},          // 127.0.0.1 em decimal: não é loopback textual
		{"0x7f000001", "3306", true},
		{"127.1", "3306", true},
		{"192.168.168.107", "3306", false}, // canônico, fora da lista
		{"10.0.0.1", "3307", false},
		{"0.0.0.0", "3306", false},
		{"127.0.0.1", "3306", false},
		{"192.168.168.106", "3307", false}, // CasaOS
		{"1password-db", "3306", false},    // nome com letras fora de a-f
		{"db1.exemplo", "3306", false},
	}
	for _, c := range casos {
		t.Run(c.host+":"+c.porta, func(t *testing.T) {
			args := []string{"DB_HOST=" + c.host, "DB_PORT=" + c.porta, "DB_NAME=rotaperfumes"}
			out, errMake := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, args...)
			errGo := integracao.ValidarNaoProducao(c.host, c.porta)
			assert.Equal(t, c.producao, errMake != nil, "Makefile: %s", out)
			assert.Equal(t, c.producao, errors.Is(errGo, integracao.ErrBancoProducao), "Go: %v", errGo)
			if c.producao {
				_, errInt := makeAlvo(t, copiaDoMakefile(t), "db-guard-integracao", nil, args...)
				assert.Error(t, errInt, "integração recusa")
				assert.Error(t, integracao.ValidarPermitidoIntegracao(c.host, c.porta)) // allowlist também recusa
			}
		})
	}
}

func TestSEC57_PortaComEspacoInvalida_Paridade(t *testing.T) {
	for _, porta := range []string{"3307 ", " 3307", "33 07", "3306\t", "+ 3306"} {
		for _, host := range []string{"192.168.168.106", "127.0.0.1", "outro-host"} {
			t.Run(host+":"+porta, func(t *testing.T) {
				args := []string{"DB_HOST=" + host, "DB_PORT=" + porta, "DB_NAME=rotaperfumes"}
				// O make tira o espaço à esquerda de toda atribuição
				// ("DB_PORT= 3307" chega como "3307"): só o Go vê esse caso.
				guardas := []string{"db-guard-producao", "db-guard-sem-producao", "db-guard-integracao"}
				if strings.TrimLeft(porta, " \t") != porta {
					guardas = nil
				}
				for _, g := range guardas {
					out, err := makeAlvo(t, copiaDoMakefile(t), g, nil, args...)
					assert.Error(t, err, g)
					assert.Contains(t, out, "SEC-57: DB_PORT=", g)
					assert.Contains(t, out, "invalida", g)
					assert.NotContains(t, out, "PRODUCAO", "%s: mensagem de porta inválida, não de produção", g)
				}
				errGo := integracao.ValidarNaoProducao(host, porta)
				assert.Error(t, errGo)
				assert.NotErrorIs(t, errGo, integracao.ErrBancoProducao)
				assert.Contains(t, errGo.Error(), "não é numérico")
				assert.ErrorIs(t, integracao.ValidarPermitidoIntegracao(host, porta), integracao.ErrHostNaoPermitido)
			})
		}
	}
	t.Run("CONFIRMO_PRODUCAO não libera porta inválida", func(t *testing.T) {
		out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil,
			"DB_HOST=192.168.168.106", "DB_PORT=3306 ", "DB_NAME=rotaperfumes", "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes")
		assert.Error(t, err)
		assert.Contains(t, out, "SEC-57")
	})
}

// portaNumericaSEC57: vazia ou "+"? seguido de dígitos (o resto é inválido).
func portaNumericaSEC57(p string) bool {
	q := strings.TrimPrefix(p, "+")
	if p == "" {
		return true
	}
	if q == "" {
		return false
	}
	return strings.Trim(q, "0123456789") == ""
}
