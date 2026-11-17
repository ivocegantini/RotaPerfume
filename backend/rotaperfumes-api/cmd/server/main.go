// Command server inicia a API rotaperfumes-api na porta 8080.
//
// Só carrega o ambiente e a config; a montagem (composition root), o
// servidor e o shutdown ficam em internal/bootstrap.
package main

import (
	"log"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[server] iniciando rotaperfumes-api")
	// ENV-01: backend/.env (subindo a partir do cwd); sem ele, só o ambiente.
	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[server] config: %v", err)
	}
	// SEC-37: secret de teste do Turnstile não sobe em produção sem a válvula.
	if err := bootstrap.ValidarCaptcha(cfg); err != nil {
		log.Fatalf("[server] config: %v", err)
	}
	vlog.SetEnabled(cfg.Verbose)
	if vlog.Enabled() {
		log.Printf("[server] WARN: VERBOSE ativo — logs de rastreamento ligados; não use em produção")
	}
	log.Printf("[server] config OK: db=%s@%s jwt_issuer=%s verbose=%v",
		cfg.DBName, cfg.DBHost, cfg.JWTIssuer, cfg.Verbose)
	// SEC-29: estado do TLS com o banco (só caminho da CA e ServerName).
	log.Printf("[server] TLS do banco: %s", cfg.DBTLS.Descricao())

	bootstrap.Run(cfg)
}
