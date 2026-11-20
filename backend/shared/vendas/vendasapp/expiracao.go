package vendasapp

import (
	"context"
	"log"
	"time"
)

// timeoutExpiracao limita uma rodada do job de expiração (ECOM-19).
const timeoutExpiracao = 2 * time.Minute

// IniciarExpiracaoPeriodica roda ExpirarPedidosLoja na subida e a cada
// intervalo (LOJA_EXPIRACAO_INTERVAL; 0 desliga) até ctx acabar. O canal
// fecha quando a goroutine termina (shutdown, I-10).
func IniciarExpiracaoPeriodica(ctx context.Context, c *Casos, intervalo time.Duration) <-chan struct{} {
	done := make(chan struct{})
	if intervalo <= 0 {
		log.Printf("[loja] expiração: job desativado (LOJA_EXPIRACAO_INTERVAL=0)")
		close(done)
		return done
	}
	go func() {
		defer close(done)
		log.Printf("[loja] expiração: job iniciado (intervalo=%s)", intervalo)
		executarExpiracao(ctx, c)
		ticker := time.NewTicker(intervalo)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("[loja] expiração: job encerrado")
				return
			case <-ticker.C:
				executarExpiracao(ctx, c)
			}
		}
	}()
	return done
}

func executarExpiracao(ctx context.Context, c *Casos) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[loja] expiração: panic recuperado: %v", r)
		}
	}()
	if ctx.Err() != nil {
		return
	}
	execCtx, cancel := context.WithTimeout(ctx, timeoutExpiracao)
	defer cancel()
	n, err := c.ExpirarPedidosLoja(execCtx)
	if err != nil {
		log.Printf("[loja] expiração: erro: %v", err)
	}
	if n > 0 {
		log.Printf("[loja] expiração: %d pedido(s) cancelado(s) por falta de pagamento", n)
	}
}
