package config_test

// TST-23 (Lote 39): catraca AST contra testes que misturam o relógio do
// banco (NOW(), CURDATE(), CURRENT_TIMESTAMP, UTC_TIMESTAMP()...) com o do Go.
// O DSN não fixa time_zone: o relógio do MariaDB segue o fuso do servidor.
//
// Regra por arquivo (_test.go de backend/shared e backend/rotaperfumes-api):
// um literal string com função de relógio SQL + uso de time.Now/Since/Until
// (fora o time.Now().UnixNano() dos sufixos únicos) = achado, salvo exceção.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reRelogioBancoTST23: funções de relógio do MariaDB num literal SQL. Os
// regex do sqlmock (`CURDATE\(\)`) não casam: o "(" vem escapado; texto
// como "time.Now (...)" num nome de subteste também não (precedido de ".").
var reRelogioBancoTST23 = regexp.MustCompile(`(?i)(?:^|[^.\w])(?:NOW|CURDATE|CURTIME|SYSDATE|UTC_TIMESTAMP|UTC_DATE|UTC_TIME|UNIX_TIMESTAMP|LOCALTIME|LOCALTIMESTAMP|CURRENT_TIMESTAMP|CURRENT_DATE|CURRENT_TIME)\s*\(|\bCURRENT_(?:TIMESTAMP|DATE|TIME)\b`)

// reDDLRelogioTST23: DEFAULT/ON UPDATE CURRENT_TIMESTAMP de um DDL só declara
// o default da coluna (não lê o relógio para comparar); some antes do match.
var reDDLRelogioTST23 = regexp.MustCompile(`(?i)\b(?:DEFAULT|ON\s+UPDATE)\s+(?:CURRENT_TIMESTAMP|NOW|LOCALTIMESTAMP|LOCALTIME)\b(?:\s*\(\s*\d*\s*\))?`)

// excecoesTST23: arquivos (relativos a backend/) que usam o relógio do banco
// e o do Go sem compará-los entre si; cada um com a justificativa.
var excecoesTST23 = map[string]string{
}

type achadoTST23 struct {
	arquivo           string
	linhaSQL, linhaGo int
}

// analisarTST23 devolve a linha do primeiro literal com relógio do banco e a
// do primeiro uso do relógio do Go (0 = ausente).
func analisarTST23(fset *token.FileSet, f *ast.File) (linhaSQL, linhaGo int) {
	nomesTime := map[string]bool{} // nomes locais do pacote time (com alias)
	for _, imp := range f.Imports {
		if caminho, _ := strconv.Unquote(imp.Path.Value); caminho == "time" {
			nome := "time"
			if imp.Name != nil {
				nome = imp.Name.Name
			}
			nomesTime[nome] = true
		}
	}
	// time.Now().UnixNano(): sufixo único de dados de teste, não é relógio.
	sufixos := map[ast.Node]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "UnixNano" {
				if interno, ok := sel.X.(*ast.CallExpr); ok {
					sufixos[interno.Fun] = true
				}
			}
		}
		return true
	})
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.BasicLit:
			if v.Kind != token.STRING || linhaSQL != 0 {
				return true
			}
			if s, err := strconv.Unquote(v.Value); err == nil && reRelogioBancoTST23.MatchString(reDDLRelogioTST23.ReplaceAllString(s, "")) {
				linhaSQL = fset.Position(v.Pos()).Line
			}
		case *ast.SelectorExpr:
			id, ok := v.X.(*ast.Ident)
			if !ok || !nomesTime[id.Name] || id.Obj != nil || linhaGo != 0 || sufixos[v] {
				return true
			}
			switch v.Sel.Name {
			case "Now", "Since", "Until":
				linhaGo = fset.Position(v.Pos()).Line
			}
		}
		return true
	})
	return linhaSQL, linhaGo
}

func analisarFonteTST23(t *testing.T, fonte string) (int, int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x_test.go", fonte, 0)
	require.NoError(t, err)
	return analisarTST23(fset, f)
}

// O analisador pega a mistura dos relógios e não acusa os usos legítimos.
func TestTST23_Analisador(t *testing.T) {
	cab := "package x_test\nimport (\n\t\"time\"\n\trel \"time\"\n)\nvar _ = rel.Now\n"
	casos := []struct {
		nome  string
		corpo string
		acha  bool
	}{
		{"NOW() + time.Now()", "func f() { _ = \"UPDATE t SET c = NOW() - INTERVAL 5 MINUTE\"; _ = time.Now() }", true},
		{"CURDATE() + Format de hoje", "func f() { _ = \"INSERT INTO t VALUES (CURDATE())\"; _ = time.Now().Format(\"2006-01-02\") }", true},
		{"CURRENT_TIMESTAMP sem parênteses", "func f() { _ = `SELECT CURRENT_TIMESTAMP`; _ = time.Now() }", true},
		{"UTC_TIMESTAMP() também (driver lê em time.Local)", "func f() { _ = \"SELECT UTC_TIMESTAMP()\"; _ = time.Now() }", true},
		{"minúsculas e espaço", "func f() { _ = \"select now ()\"; _ = time.Now() }", true},
		{"time.Since", "func f() { _ = \"SELECT SYSDATE()\"; _ = time.Since(time.Time{}) }", true},
		{"time.Now como valor", "func f() { _ = \"SELECT CURDATE()\"; g(time.Now) }\nfunc g(func() time.Time) {}", true},
		{"alias do pacote time", "func f() { _ = \"SELECT NOW()\" }", true}, // var _ = rel.Now no cabeçalho
		{"só UnixNano (sufixo único): ok", "func f() { _ = \"SELECT NOW()\"; _ = time.Now().UnixNano() }", false},
		{"regex do sqlmock (parêntese escapado): ok", "func f() { _ = `DATE_SUB\\(CURDATE\\(\\), INTERVAL 6 DAY\\)`; _ = time.Now() }", false},
		{"valor do Go no placeholder: ok", "func f() { _ = \"UPDATE t SET c = ?\"; _ = time.Now() }", false},
		{"identificador now_x não é função: ok", "func f() { _ = \"SELECT nowhere, knows(1)\"; _ = time.Now() }", false},
		{"nome de subteste citando time.Now (: ok", "func f() { _ = \"nil usa time.Now (ainda válido)\"; _ = time.Now() }", false},
		{"DDL com DEFAULT/ON UPDATE CURRENT_TIMESTAMP: ok", "func f() { _ = `c TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP(6)`; _ = time.Now() }", false},
		{"DDL + SELECT NOW() no mesmo literal", "func f() { _ = `c DATETIME DEFAULT CURRENT_TIMESTAMP; SELECT NOW()`; _ = time.Now() }", true},
		{"variável local chamada time: ok", "func f() { time := struct{ Now func() }{}; time.Now(); _ = \"SELECT NOW()\" }", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			fonte := cab + c.corpo + "\n"
			if c.nome != "alias do pacote time" {
				fonte = strings.Replace(fonte, "var _ = rel.Now\n", "var _ = rel.Duration(0)\n", 1)
			}
			sqlL, goL := analisarFonteTST23(t, fonte)
			assert.Equal(t, c.acha, sqlL != 0 && goL != 0, "sql=%d go=%d", sqlL, goL)
		})
	}
}

// Varredura real: nenhum _test.go novo mistura os dois relógios.
func TestTST23_CatracaRelogioBancoVsGo(t *testing.T) {
	backend := filepath.Join(raizProjeto(t), "backend")
	fset := token.NewFileSet()
	var achados []achadoTST23
	vistos := 0
	for _, mod := range []string{"shared", "rotaperfumes-api"} {
		err := filepath.WalkDir(filepath.Join(backend, mod), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "testdata", "node_modules", "vendor", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				return err
			}
			vistos++
			rel := filepath.ToSlash(strings.TrimPrefix(p, backend+string(filepath.Separator)))
			if sqlL, goL := analisarTST23(fset, f); sqlL != 0 && goL != 0 {
				achados = append(achados, achadoTST23{rel, sqlL, goL})
			}
			return nil
		})
		require.NoError(t, err)
	}
	assert.Greater(t, vistos, 100, "a varredura achou os _test.go")

	usadas := map[string]bool{}
	var novos []string
	for _, a := range achados {
		if _, ok := excecoesTST23[a.arquivo]; ok {
			usadas[a.arquivo] = true
			continue
		}
		novos = append(novos, a.arquivo+": relógio do banco (linha "+strconv.Itoa(a.linhaSQL)+
			") + relógio do Go (linha "+strconv.Itoa(a.linhaGo)+")")
	}
	sort.Strings(novos)
	assert.Empty(t, novos, "TST-23: grave o instante pelo Go (placeholder ?) ou compare só com o relógio do banco; o time_zone do servidor não é fixo")
	for arq, j := range excecoesTST23 {
		assert.NotEmpty(t, strings.TrimSpace(j), "exceção %s sem justificativa", arq)
		assert.True(t, usadas[arq], "exceção obsoleta (tire da lista): %s", arq)
	}
}
