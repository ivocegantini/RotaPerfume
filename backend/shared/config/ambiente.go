package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Ambiente é o APP_ENV da API (ECOM-18): dev, qa ou producao.
type Ambiente string

// Ambientes aceitos no APP_ENV.
const (
	AmbienteDev      Ambiente = "dev"
	AmbienteQA       Ambiente = "qa"
	AmbienteProducao Ambiente = "producao"
)

// PermiteSimuladorPagamento: só dev e qa aceitam o simulador de pagamento
// (POST /api/loja/pagamentos/{id}/simular); em produção ele responde 404.
func (a Ambiente) PermiteSimuladorPagamento() bool { return a == AmbienteDev || a == AmbienteQA }

// LoadAmbiente lê o APP_ENV. Ausente = producao (fail-closed: o simulador
// fica desligado); valor fora da lista derruba a subida.
func LoadAmbiente() (Ambiente, error) {
	v := Ambiente(strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))))
	switch v {
	case "":
		return AmbienteProducao, nil
	case AmbienteDev, AmbienteQA, AmbienteProducao:
		return v, nil
	}
	return "", fmt.Errorf("config: APP_ENV inválido (%q): use dev, qa ou producao", string(v))
}

// Padrão e mínimo do LOJA_EXPIRACAO_INTERVAL (ECOM-19).
const (
	lojaExpiracaoPadrao = time.Hour
	lojaExpiracaoMin    = time.Minute
)

// LoadExpiracaoLoja lê o LOJA_EXPIRACAO_INTERVAL: intervalo do job que cancela
// pedidos da loja com pagamento vencido (ECOM-19). Ausente = 1h; "0" desliga;
// outro valor precisa ser >= 1m.
func LoadExpiracaoLoja() (time.Duration, error) {
	const chave = "LOJA_EXPIRACAO_INTERVAL"
	raw := strings.TrimSpace(os.Getenv(chave))
	if raw == "" {
		return lojaExpiracaoPadrao, nil
	}
	if raw == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s inválido (%q): %w", chave, raw, err)
	}
	if d == 0 {
		return 0, nil
	}
	if d < lojaExpiracaoMin {
		return 0, fmt.Errorf("config: %s inválido (%q): esperado 0 (desativa) ou >= %s", chave, raw, lojaExpiracaoMin)
	}
	return d, nil
}
