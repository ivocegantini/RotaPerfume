package limpezaimagens_test

// SEC-48 (Lote 39): -reprocessar-webp reencoda os .webp já gravados (como o
// SEC-34 no upload) e troca produtos.imagem sem órfãos. Diretório temporário,
// banco em sqlmock e o normalizador/armazenamento reais (salvo nas falhas).

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/limpezaimagens"
)

const (
	webpOpaca        = "a1111111-1111-4111-8111-111111111111.webp"
	webpTransparente = "b2222222-2222-4222-8222-222222222222.webp"
	webpCorrompida   = "c3333333-3333-4333-8333-333333333333.webp"
	webpSemArquivo   = "d4444444-4444-4444-8444-444444444444.webp"
	webpSymlink      = "e5555555-5555-4555-8555-555555555555.webp"
)

// sec48WebP monta um WebP lossless (VP8L) 4x4 de cor única com o alfa dado.
func sec48WebP(alfa uint64) []byte {
	var buf []byte
	var acc uint64
	var n uint
	put := func(v uint64, bits uint) {
		acc |= v << n
		n += bits
		for n >= 8 {
			buf = append(buf, byte(acc))
			acc >>= 8
			n -= 8
		}
	}
	put(0x2f, 8)
	put(3, 14)
	put(3, 14)
	put(0, 7) // alpha_is_used, versão (3), sem transform, cache e meta
	for _, sym := range []uint64{0x40, 0x80, 0xC0, alfa, 0} {
		put(1, 1)
		put(0, 1)
		put(1, 1)
		put(sym, 8)
	}
	if n > 0 {
		buf = append(buf, byte(acc))
	}
	var c bytes.Buffer
	c.WriteString("VP8L")
	_ = binary.Write(&c, binary.LittleEndian, uint32(len(buf)))
	c.Write(buf)
	if len(buf)%2 == 1 {
		c.WriteByte(0)
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(4+c.Len()))
	out.WriteString("WEBP")
	out.Write(c.Bytes())
	return out.Bytes()
}

func sec48Escrever(t *testing.T, dir, nome string, dados []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, nome), dados, 0o640))
}

func sec48Arquivos(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	require.NoError(t, err)
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// nomeNovo é o argumento do UPDATE que guarda o nome gravado.
type nomeNovo struct{ nome *string }

func (a nomeNovo) Match(v driver.Value) bool {
	s, ok := v.(string)
	*a.nome = s
	return ok && catalogo.NomeImagemValido(s) && !strings.HasSuffix(s, ".webp")
}

type sec48Linha struct {
	id   int64
	nome string
}

func sec48Mock(t *testing.T, linhas ...sec48Linha) (cmdutil.Opener, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	rows := sqlmock.NewRows([]string{"id", "imagem"})
	for _, l := range linhas {
		rows.AddRow(l.id, l.nome)
	}
	mock.ExpectPing()
	mock.ExpectQuery(`SELECT id, imagem FROM produtos WHERE imagem LIKE '%\.webp'`).WillReturnRows(rows)
	return func() (cmdutil.Conn, error) { return db, nil }, mock
}

func TestSEC48_Reprocessar_ConverteETrocaAReferencia(t *testing.T) {
	buf := capturarLog(t)
	dir := t.TempDir()
	sec48Escrever(t, dir, webpOpaca, sec48WebP(0xFF))
	sec48Escrever(t, dir, webpTransparente, sec48WebP(0x10))
	open, mock := sec48Mock(t, sec48Linha{1, webpOpaca}, sec48Linha{2, webpTransparente})
	var novo1, novo2 string
	prep := mock.ExpectPrepare(`UPDATE produtos SET imagem = \? WHERE id = \? AND imagem = \?`)
	prep.ExpectExec().WithArgs(nomeNovo{&novo1}, int64(1), webpOpaca).WillReturnResult(sqlmock.NewResult(0, 1))
	prep.ExpectExec().WithArgs(nomeNovo{&novo2}, int64(2), webpTransparente).WillReturnResult(sqlmock.NewResult(0, 1))
	prep.WillBeClosed()
	mock.ExpectClose()

	r, err := limpezaimagens.Reprocessar(context.Background(), limpezaimagens.OpcoesReprocessar{Dir: dir}, open)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, limpezaimagens.RelatorioReprocessar{Encontradas: 2, Convertidas: 2}, r)
	assert.True(t, strings.HasSuffix(novo1, ".jpg"), "opaca → JPEG: %s", novo1)
	assert.True(t, strings.HasSuffix(novo2, ".png"), "transparente → PNG: %s", novo2)
	esperado := []string{novo1, novo2}
	sort.Strings(esperado)
	assert.Equal(t, esperado, sec48Arquivos(t, dir), "só os novos: sem .webp antigo nem .tmp-*")
	for _, n := range esperado {
		b, err := os.ReadFile(filepath.Join(dir, n))
		require.NoError(t, err)
		assert.Equal(t, catalogo.TipoDoNome(n), http.DetectContentType(b), "conteúdo = extensão")
	}
	assert.Contains(t, buf.String(), "reprocessar_webp dry_run=false encontradas=2 convertidas=2")
	assert.NotContains(t, buf.String(), ".webp", "log só com totais")
	assert.NotContains(t, buf.String(), dir)
}

func TestSEC48_Reprocessar_DryRunNaoTocaNada(t *testing.T) {
	buf := capturarLog(t)
	dir := t.TempDir()
	sec48Escrever(t, dir, webpOpaca, sec48WebP(0xFF))
	sec48Escrever(t, dir, webpCorrompida, []byte("RIFF\x10\x00\x00\x00WEBPVP8Lquebrado"))
	open, mock := sec48Mock(t, sec48Linha{1, webpOpaca}, sec48Linha{3, webpCorrompida})
	mock.ExpectClose() // sem Prepare/Exec: nenhum UPDATE

	r, err := limpezaimagens.Reprocessar(context.Background(), limpezaimagens.OpcoesReprocessar{Dir: dir, DryRun: true}, open)
	require.Error(t, err, "a corrompida é falha também no dry-run")
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, 1, r.Convertidas, "seriam convertidas")
	assert.Equal(t, 1, r.Falhas)
	assert.Equal(t, []string{webpOpaca, webpCorrompida}, sec48Arquivos(t, dir), "nada gravado nem apagado")
	assert.Contains(t, buf.String(), "reprocessar_webp dry_run=true")
}

func TestSEC48_Reprocessar_ReferenciasSemArquivoESymlink(t *testing.T) {
	capturarLog(t)
	dir := t.TempDir()
	externo := filepath.Join(t.TempDir(), "fora.webp")
	require.NoError(t, os.WriteFile(externo, sec48WebP(0xFF), 0o640))
	require.NoError(t, os.Symlink(externo, filepath.Join(dir, webpSymlink)))
	open, mock := sec48Mock(t, sec48Linha{4, webpSemArquivo}, sec48Linha{5, webpSymlink}, sec48Linha{6, "../fora.webp"})
	mock.ExpectPrepare(`UPDATE produtos`).WillBeClosed()
	mock.ExpectClose()

	r, err := limpezaimagens.Reprocessar(context.Background(), limpezaimagens.OpcoesReprocessar{Dir: dir}, open)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, limpezaimagens.RelatorioReprocessar{Encontradas: 3, SemArquivo: 2, Ignoradas: 1}, r)
	_, err = os.Stat(externo)
	assert.NoError(t, err, "symlink não é seguido")
	assert.Equal(t, []string{webpSymlink}, sec48Arquivos(t, dir))
}

// armazenamentoFalho falha no Gravar ou no Remover; o resto é em memória.
type armazenamentoFalho struct {
	dir                   string
	falhaGravar, falhaRem bool
}

func (a *armazenamentoFalho) Gravar(_ context.Context, ext string, dados []byte) (string, error) {
	if a.falhaGravar {
		return "", errors.New("disco cheio")
	}
	nome := "f6666666-6666-4666-8666-666666666666" + ext
	return nome, os.WriteFile(filepath.Join(a.dir, nome), dados, 0o640)
}

func (a *armazenamentoFalho) Remover(_ context.Context, nome string) error {
	if a.falhaRem {
		return errors.New("sem permissão")
	}
	return os.Remove(filepath.Join(a.dir, nome))
}

func TestSEC48_Reprocessar_FalhaSemPerda(t *testing.T) {
	errBanco := errors.New("falha-do-banco")
	casos := []struct {
		nome       string
		arm        func(dir string) limpezaimagens.Armazenamento
		exec       func(*sqlmock.ExpectedExec)
		corrompida bool
		comErro    bool
		arquivos   func(novo string) []string
		rel        limpezaimagens.RelatorioReprocessar
	}{
		{"webp corrompida: original mantido", nil, nil, true, true,
			func(string) []string { return []string{webpOpaca} },
			limpezaimagens.RelatorioReprocessar{Encontradas: 1, Falhas: 1}},
		{"gravação falha: original mantido", func(d string) limpezaimagens.Armazenamento { return &armazenamentoFalho{dir: d, falhaGravar: true} },
			nil, false, true,
			func(string) []string { return []string{webpOpaca} },
			limpezaimagens.RelatorioReprocessar{Encontradas: 1, Falhas: 1}},
		{"UPDATE falha: novo apagado, original mantido", nil,
			func(e *sqlmock.ExpectedExec) { e.WillReturnError(errBanco) }, false, true,
			func(string) []string { return []string{webpOpaca} },
			limpezaimagens.RelatorioReprocessar{Encontradas: 1, Falhas: 1}},
		{"produto mudou (0 linhas): novo apagado, original mantido", nil,
			func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, false, false,
			func(string) []string { return []string{webpOpaca} },
			limpezaimagens.RelatorioReprocessar{Encontradas: 1, Conflitos: 1}},
		{"apagar o antigo falha: referência trocada, erro relatado",
			func(d string) limpezaimagens.Armazenamento { return &armazenamentoFalho{dir: d, falhaRem: true} },
			func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, false, true,
			func(novo string) []string { return []string{webpOpaca, novo} },
			limpezaimagens.RelatorioReprocessar{Encontradas: 1, Convertidas: 1, FalhasAoApagar: 1}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t)
			dir := t.TempDir()
			original := sec48WebP(0xFF)
			if c.corrompida {
				original = []byte("RIFF\x10\x00\x00\x00WEBPVP8Lquebrado")
			}
			sec48Escrever(t, dir, webpOpaca, original)
			open, mock := sec48Mock(t, sec48Linha{1, webpOpaca})
			prep := mock.ExpectPrepare(`UPDATE produtos`)
			var novo string
			if c.exec != nil {
				c.exec(prep.ExpectExec().WithArgs(nomeNovo{&novo}, int64(1), webpOpaca))
			}
			prep.WillBeClosed()
			mock.ExpectClose()
			opts := limpezaimagens.OpcoesReprocessar{Dir: dir}
			if c.arm != nil {
				opts.Armazenamento = c.arm(dir)
			}

			r, err := limpezaimagens.Reprocessar(context.Background(), opts, open)
			assert.Equal(t, c.comErro, err != nil, "erro: %v", err)
			require.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, c.rel, r)
			esperado := c.arquivos(novo)
			sort.Strings(esperado)
			assert.Equal(t, esperado, sec48Arquivos(t, dir))
			b, err := os.ReadFile(filepath.Join(dir, webpOpaca))
			require.NoError(t, err)
			assert.Equal(t, original, b, "original intacto")
		})
	}
}

func TestSEC48_Reprocessar_ErroNoSelectNaoTocaNada(t *testing.T) {
	dir := t.TempDir()
	sec48Escrever(t, dir, webpOpaca, sec48WebP(0xFF))
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	mock.ExpectPing()
	mock.ExpectQuery(`SELECT id, imagem FROM produtos`).WillReturnError(errors.New("falha-do-banco"))
	mock.ExpectClose()
	_, err = limpezaimagens.Reprocessar(context.Background(), limpezaimagens.OpcoesReprocessar{Dir: dir},
		func() (cmdutil.Conn, error) { return db, nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ler produtos.imagem")
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, []string{webpOpaca}, sec48Arquivos(t, dir))
}

func TestSEC48_CLI_ReprocessarWebPDryRun(t *testing.T) {
	dir := t.TempDir()
	sec48Escrever(t, dir, webpOpaca, sec48WebP(0xFF))
	open, mock := sec48Mock(t, sec48Linha{1, webpOpaca})
	mock.ExpectClose()
	conn, err := open()
	require.NoError(t, err)
	amb, _ := ambienteCLI(t, dir, nil, conn)
	buf := capturarLog(t)
	assert.Equal(t, cmdutil.SaidaOK, cmdutil.Executar(limpezaimagens.Tag, limpezaimagens.CLI, []string{"-reprocessar-webp", "-dry-run"}, amb))
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, buf.String(), "reprocessar_webp dry_run=true encontradas=1 convertidas=1")
	assert.Equal(t, []string{webpOpaca}, sec48Arquivos(t, dir))
}

// SecBrain (ressalva do Lote 39): o arquivo é aberto uma vez; symlink para um
// .webp DENTRO do diretório não é seguido e acima do teto é falha sem perda.
func TestSEC48_Reprocessar_SymlinkInternoETeto(t *testing.T) {
	capturarLog(t)
	dir := t.TempDir()
	alvo := "f7777777-7777-4777-8777-777777777777.txt" // fora do padrão: a limpeza ignora
	sec48Escrever(t, dir, alvo, sec48WebP(0xFF))
	require.NoError(t, os.Symlink(alvo, filepath.Join(dir, webpSymlink)))
	grande := append(sec48WebP(0xFF), bytes.Repeat([]byte{0}, catalogo.MaxImagemBytes)...)
	sec48Escrever(t, dir, webpOpaca, grande)
	open, mock := sec48Mock(t, sec48Linha{5, webpSymlink}, sec48Linha{1, webpOpaca})
	mock.ExpectPrepare(`UPDATE produtos`).WillBeClosed()
	mock.ExpectClose()

	r, err := limpezaimagens.Reprocessar(context.Background(), limpezaimagens.OpcoesReprocessar{Dir: dir}, open)
	require.Error(t, err, "acima do teto conta como falha")
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, limpezaimagens.RelatorioReprocessar{Encontradas: 2, SemArquivo: 1, Falhas: 1}, r)
	assert.Equal(t, []string{webpOpaca, webpSymlink, alvo}, sec48Arquivos(t, dir), "nada gravado nem apagado")
	b, err := os.ReadFile(filepath.Join(dir, webpOpaca))
	require.NoError(t, err)
	assert.Equal(t, grande, b, "original intacto")
}
