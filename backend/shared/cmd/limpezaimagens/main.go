// Command limpezaimagens apaga as imagens de produto órfãs (sem referência
// em produtos.imagem, com mais de 1 h) e os .tmp-* com mais de 24 h do
// PRODUTO_IMAGENS_DIR (CHORE-09). A lógica fica em tools/limpezaimagens;
// este main é só a composition root.
//
// Uso (a partir de backend/shared):
//
//	go run ./cmd/limpezaimagens -dry-run
//	go run ./cmd/limpezaimagens
//	go run ./cmd/limpezaimagens -reprocessar-webp -dry-run   # SEC-48
//	go run ./cmd/limpezaimagens -reprocessar-webp
//
// Roda com a conta da API (SELECT em produtos; o -reprocessar-webp também
// faz UPDATE produtos.imagem). O log tem só totais.
package main

import (
	"log"
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/limpezaimagens"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	os.Exit(cmdutil.Executar(limpezaimagens.Tag, limpezaimagens.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
