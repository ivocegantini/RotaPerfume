// Package cmdutil reúne os utilitários comuns aos comandos de linha de
// comando de backend/shared/cmd (importadores e ferramentas): localizar a raiz
// do repositório e carregar o backend/.env.
package cmdutil

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/rotaperfumes/shared/projroot"
)

// maxNiveis limita quantos diretórios acima do cwd são inspecionados.
const maxNiveis = 6

// FindProjectRoot sobe a árvore de diretórios a partir do cwd até achar a
// raiz do repositório (identificada por backend/shared/go.mod). Delegado a
// projroot.Find, que o pacote config também usa (SEC-29).
func FindProjectRoot() (string, error) {
	return projroot.Find()
}

// EnvRelativo é o caminho do .env a partir da raiz do projeto (ENV-01: fica
// em backend/, não mais na raiz).
const EnvRelativo = "backend/.env"

// LoadEnvFromCwd tenta carregar o backend/.env subindo diretórios a partir do
// working directory. Não retorna erro — se não achar, segue sem .env.
func LoadEnvFromCwd() {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	dir := wd
	for i := 0; i < maxNiveis; i++ {
		arquivo := filepath.Join(dir, filepath.FromSlash(EnvRelativo))
		if _, err := os.Stat(arquivo); err == nil {
			_ = godotenv.Load(arquivo)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}
