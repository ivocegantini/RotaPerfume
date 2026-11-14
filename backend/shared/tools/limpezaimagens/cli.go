package limpezaimagens

import (
	"context"
	"errors"

	"github.com/rotaperfumes/shared/cmdutil"
)

// errFalhasAoApagar: a execução terminou, mas algum arquivo não pôde ser
// apagado (o total está no log).
var errFalhasAoApagar = errors.New("houve falhas ao apagar arquivos (ver falhas_ao_apagar no log)")

// CLI é o comando cmd/limpezaimagens (flags -dry-run e -reprocessar-webp,
// SEC-48); o main só chama cmdutil.Executar. O diretório vem de
// PRODUTO_IMAGENS_DIR (config.Load).
func CLI(args []string, amb cmdutil.Ambiente) error {
	fs := cmdutil.NovasFlags(Tag, amb)
	dryRun := fs.Bool("dry-run", false, "só relata os totais, sem apagar nem gravar nada")
	webp := fs.Bool("reprocessar-webp", false, "SEC-48: reencoda os .webp de produto (PNG/JPEG) e atualiza produtos.imagem, em vez de limpar")
	if err := cmdutil.LerFlags(fs, args); err != nil {
		return err
	}
	cfg, err := cmdutil.Configurar(amb, false)
	if err != nil {
		return err
	}
	if *webp {
		_, err := Reprocessar(context.Background(), OpcoesReprocessar{Dir: cfg.ProdutoImagensDir, DryRun: *dryRun}, amb.Abrir(cfg.DSN()))
		return err
	}
	r, err := Run(Options{Dir: cfg.ProdutoImagensDir, DryRun: *dryRun}, amb.Abrir(cfg.DSN()))
	if err != nil {
		return err
	}
	if r.FalhasAoApagar > 0 {
		return errFalhasAoApagar
	}
	return nil
}
