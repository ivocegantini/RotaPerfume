package limpezaimagens

// SEC-48 (Lote 39): -reprocessar-webp reencoda os .webp de produto gravados
// antes do SEC-34 (mesmo normalizador do upload: PNG se tiver transparência,
// JPEG q90 se opaco) e troca produtos.imagem sem deixar órfãos:
// grava o novo (atômico) → UPDATE condicional (imagem ainda = antigo) → só
// então apaga o antigo. Qualquer falha antes do UPDATE mantém o original e
// apaga o novo; o log tem só totais.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogofs"
	"github.com/rotaperfumes/shared/catalogo/catalogoimagem"
	"github.com/rotaperfumes/shared/cmdutil"
)

const (
	sqlWebPs           = `SELECT id, imagem FROM produtos WHERE imagem LIKE '%.webp'`
	sqlTrocarImagem    = `UPDATE produtos SET imagem = ? WHERE id = ? AND imagem = ?`
	extensaoWebPOrigem = ".webp"
)

// errFalhasAoReprocessar: a execução terminou, mas alguma imagem ficou como
// estava (o total está no log).
var errFalhasAoReprocessar = errors.New("houve falhas ao reprocessar imagens (ver falhas no log)")

// Normalizador é a porta do reencode (catalogoimagem.Normalizador).
type Normalizador interface {
	Normalizar(dados []byte) (catalogo.ImagemNormalizada, error)
}

// Armazenamento é a porta da gravação (catalogofs.Armazenamento).
type Armazenamento interface {
	Gravar(ctx context.Context, extensao string, dados []byte) (string, error)
	Remover(ctx context.Context, nome string) error
}

// OpcoesReprocessar configura o -reprocessar-webp. Normalizador e
// Armazenamento nil usam os adapters do upload.
type OpcoesReprocessar struct {
	Dir           string
	DryRun        bool
	Normalizador  Normalizador
	Armazenamento Armazenamento
}

// RelatorioReprocessar são os totais (o único conteúdo logado).
type RelatorioReprocessar struct {
	Encontradas    int // produtos.imagem terminando em .webp
	Convertidas    int // trocadas (ou que seriam, no dry-run)
	SemArquivo     int // referência sem arquivo regular no diretório (só reporta)
	Ignoradas      int // nome fora do padrão <uuid-v4>.webp
	Conflitos      int // produto mudou de imagem durante a execução (novo apagado)
	Falhas         int // leitura, reencode, gravação ou UPDATE (original mantido)
	FalhasAoApagar int // arquivo que não pôde ser apagado (a limpeza normal o pega depois)
}

type webpProduto struct {
	id   int64
	nome string
}

// Reprocessar abre o banco (open), lê os produtos com .webp e troca cada um.
func Reprocessar(ctx context.Context, opts OpcoesReprocessar, open cmdutil.Opener) (RelatorioReprocessar, error) {
	conn, err := open()
	if err != nil {
		return RelatorioReprocessar{}, fmt.Errorf("abrir banco: %w", err)
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		return RelatorioReprocessar{}, fmt.Errorf("conectar ao banco: %w", err)
	}
	lista, err := webpsReferenciadas(conn)
	if err != nil {
		return RelatorioReprocessar{}, err
	}
	root, err := os.OpenRoot(opts.Dir)
	if err != nil {
		return RelatorioReprocessar{}, fmt.Errorf("abrir PRODUTO_IMAGENS_DIR: %w", err)
	}
	defer root.Close()
	p := &reprocessador{opts: opts, root: root, r: RelatorioReprocessar{Encontradas: len(lista)}}
	if p.opts.Normalizador == nil {
		p.opts.Normalizador = catalogoimagem.Novo()
	}
	if !opts.DryRun && len(lista) > 0 {
		if err := p.prepararEscrita(conn); err != nil {
			return RelatorioReprocessar{}, err
		}
		defer p.fechar()
	}
	for _, w := range lista {
		if err := ctx.Err(); err != nil {
			return p.r, err
		}
		p.processar(ctx, w)
	}
	logarReprocessar(p.r, opts.DryRun)
	if p.r.Falhas > 0 || p.r.FalhasAoApagar > 0 {
		return p.r, errFalhasAoReprocessar
	}
	return p.r, nil
}

// webpsReferenciadas lê (id, imagem) dos produtos com imagem .webp.
func webpsReferenciadas(db cmdutil.DB) ([]webpProduto, error) {
	rows, err := db.Query(sqlWebPs)
	if err != nil {
		return nil, fmt.Errorf("ler produtos.imagem: %w", err)
	}
	defer rows.Close()
	var out []webpProduto
	for rows.Next() {
		var w webpProduto
		if err := rows.Scan(&w.id, &w.nome); err != nil {
			return nil, fmt.Errorf("ler produtos.imagem: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ler produtos.imagem: %w", err)
	}
	return out, nil
}

type reprocessador struct {
	opts   OpcoesReprocessar
	root   *os.Root
	update *sql.Stmt
	fechar func()
	r      RelatorioReprocessar
}

// prepararEscrita abre o armazenamento (se não injetado) e prepara o UPDATE.
func (p *reprocessador) prepararEscrita(db cmdutil.DB) error {
	fechar := func() {}
	if p.opts.Armazenamento == nil {
		a, err := catalogofs.Novo(p.opts.Dir)
		if err != nil {
			return err
		}
		p.opts.Armazenamento = a
		fechar = func() { _ = a.Close() }
	}
	stmt, err := db.Prepare(sqlTrocarImagem)
	if err != nil {
		fechar()
		return fmt.Errorf("preparar UPDATE produtos: %w", err)
	}
	p.update = stmt
	p.fechar = func() { _ = stmt.Close(); fechar() }
	return nil
}

// processar converte uma imagem. Nunca apaga o original antes do UPDATE.
func (p *reprocessador) processar(ctx context.Context, w webpProduto) {
	if !catalogo.NomeImagemValido(w.nome) || catalogo.TipoDoNome(w.nome) != catalogo.TipoImagemWebP {
		p.r.Ignoradas++
		return
	}
	dados, err := p.ler(w.nome)
	if errors.Is(err, fs.ErrNotExist) {
		p.r.SemArquivo++
		return
	}
	if err != nil {
		p.r.Falhas++
		return
	}
	norm, err := p.opts.Normalizador.Normalizar(dados)
	if err != nil || norm.Extensao == extensaoWebPOrigem || norm.Extensao == "" {
		p.r.Falhas++
		return
	}
	if p.opts.DryRun {
		p.r.Convertidas++
		return
	}
	novo, err := p.opts.Armazenamento.Gravar(ctx, norm.Extensao, norm.Dados)
	if err != nil {
		p.r.Falhas++
		return
	}
	if ok, err := p.trocar(w, novo); !ok {
		if err != nil {
			p.r.Falhas++
		} else {
			p.r.Conflitos++
		}
		p.remover(ctx, novo)
		return
	}
	p.r.Convertidas++
	p.remover(ctx, w.nome)
}

// ler abre o arquivo UMA vez e decide pelo Stat do descritor (regular, mesmo
// inode do Lstat: symlink não é seguido; troca no meio é recusada) e lê com
// io.LimitReader até o teto do upload + 1 byte.
func (p *reprocessador) ler(nome string) ([]byte, error) {
	f, err := p.root.Open(nome)
	if err != nil {
		if l, e := p.root.Lstat(nome); e == nil && !l.Mode().IsRegular() {
			return nil, fs.ErrNotExist // symlink para fora do diretório
		}
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	link, err := p.root.Lstat(nome)
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(info, link) {
		return nil, fs.ErrNotExist
	}
	dados, err := io.ReadAll(io.LimitReader(f, catalogo.MaxImagemBytes+1))
	if err != nil {
		return nil, err
	}
	if len(dados) > catalogo.MaxImagemBytes {
		return nil, catalogo.ErrImagemMuitoGrande
	}
	return dados, nil
}

// trocar faz o UPDATE condicional: false sem erro = produto mudou (conflito).
func (p *reprocessador) trocar(w webpProduto, novo string) (bool, error) {
	res, err := p.update.Exec(novo, w.id, w.nome)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (p *reprocessador) remover(ctx context.Context, nome string) {
	if err := p.opts.Armazenamento.Remover(ctx, nome); err != nil {
		p.r.FalhasAoApagar++
	}
}

// logarReprocessar escreve só os totais (nunca nomes nem caminhos).
func logarReprocessar(r RelatorioReprocessar, dryRun bool) {
	log.Printf("%s: reprocessar_webp dry_run=%t encontradas=%d convertidas=%d sem_arquivo=%d ignoradas=%d conflitos=%d falhas=%d falhas_ao_apagar=%d",
		Tag, dryRun, r.Encontradas, r.Convertidas, r.SemArquivo, r.Ignoradas, r.Conflitos, r.Falhas, r.FalhasAoApagar)
}
