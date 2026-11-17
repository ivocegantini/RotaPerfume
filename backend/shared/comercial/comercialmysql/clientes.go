package comercialmysql

import (
	"context"
	"time"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/tz"
	"github.com/rotaperfumes/shared/vlog"
)

const arqClientes = "clientes.go"

// clienteColunas: bairro, cnpj/cpf (um dos dois é NULL, ECOM-05) e o
// endereço são NULLable e viram "" (COALESCE). Os da PF vêm no fim (ECOM-06).
const clienteColunas = `cliente_id_origem, COALESCE(cnpj, ''), razao_social, segmento, cidade, uf, COALESCE(bairro, ''), data_cadastro, ativo, created_at, updated_at, ` +
	`COALESCE(cpf, ''), COALESCE(endereco, ''), COALESCE(numero, ''), COALESCE(complemento, ''), COALESCE(cep, '')`

// clienteNaCarteiraCond restringe à carteira ATIVA de um vendedor; o
// vendedor_id vai sempre por placeholder.
const clienteNaCarteiraCond = "cliente_id_origem IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = ? AND data_fim IS NULL)"

// uqClientesCNPJ é o índice único de clientes.cnpj (NEG-01).
const uqClientesCNPJ = "uq_clientes_cnpj"

// Clientes é o repositório de clientes (porta comercialapp.Clientes).
type Clientes struct{ q db.Querier }

// NovoClientes liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoClientes(q db.Querier) *Clientes {
	if q == nil {
		panic("comercialmysql: NovoClientes exige Querier")
	}
	return &Clientes{q: q}
}

// Listar pagina os clientes do escopo que casam com o filtro. No escopo de
// vendedor, a carteira ativa entra no WHERE por Escopo.Filtro (O1).
func (r *Clientes) Listar(ctx context.Context, e kernel.Escopo, f comercial.FiltroCliente, p kernel.Pagina) (kernel.Resultado[comercial.Cliente], error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return kernel.Resultado[comercial.Cliente]{}, err
	}
	w, args := whereCliente(f)
	if filtra {
		w, args = append(w, clienteNaCarteiraCond), append(args, vendedorID)
	}
	res, err := paginar(ctx, r.q, "clientes", "clientes", clienteColunas, db.Where(w), ordemCliente(f.Ordenacao), args, p, scanCliente)
	if err != nil {
		return res, err
	}
	vlog.Printf(arqClientes, "Clientes.Listar", "SELECT clientes linhas=%d total=%d carteira=%t", len(res.Itens), res.Total, filtra)
	return res, nil
}

// whereCliente traduz o filtro (C-03). Busca com máscara de CNPJ procura o
// CNPJ sem máscara (TermoBuscaCNPJ). '%' e '_' da busca são literais
// (db.TermoContem, SEC-25).
func whereCliente(f comercial.FiltroCliente) ([]string, []any) {
	var conds []string
	var args []any
	if f.UF != "" {
		conds, args = append(conds, "uf = ?"), append(args, f.UF)
	}
	if f.Segmento != "" {
		conds, args = append(conds, "segmento = ?"), append(args, f.Segmento)
	}
	if f.Ativo != nil {
		conds, args = append(conds, "ativo = ?"), append(args, *f.Ativo)
	}
	if f.Busca != "" {
		termoCNPJ := comercial.TermoBuscaCNPJ(f.Busca)
		if termoCNPJ == "" {
			termoCNPJ = f.Busca
		}
		conds = append(conds, "(razao_social LIKE ? ESCAPE '!' OR cnpj LIKE ? ESCAPE '!')")
		args = append(args, db.TermoContem(f.Busca), db.TermoContem(termoCNPJ))
	}
	return conds, args
}

// PorID busca o cliente (ativo ou não); ErrClienteNaoEncontrado sem linha.
func (r *Clientes) PorID(ctx context.Context, id int64) (*comercial.Cliente, error) {
	q := "SELECT " + clienteColunas + " FROM clientes WHERE cliente_id_origem = ? LIMIT 1"
	c, err := scanCliente(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqClientes, "Clientes.PorID", "SELECT clientes linhas=1")
	return c, nil
}

func scanCliente(s db.Linha) (*comercial.Cliente, error) {
	return db.LerUma(s, comercial.ErrClienteNaoEncontrado, "comercialmysql: ler cliente", func(c *comercial.Cliente) []any {
		return []any{&c.ClienteIDOrigem, &c.CNPJ, &c.RazaoSocial, &c.Segmento, &c.Cidade, &c.UF, &c.Bairro,
			&c.DataCadastro, &c.Ativo, &c.CreatedAt, &c.UpdatedAt, &c.CPF, &c.Endereco, &c.Numero, &c.Complemento, &c.CEP}
	})
}

// Existe informa se há cliente com o id (ativo ou não).
func (r *Clientes) Existe(ctx context.Context, id int64) (bool, error) {
	return db.Existe(ctx, r.q, arqClientes, "Clientes.Existe", "comercialmysql: existe cliente", `SELECT 1 FROM clientes WHERE cliente_id_origem = ? LIMIT 1`, id)
}

// TravarParaVinculo trava a linha do cliente até o fim da tx (BUG-14:
// serializa vinculações concorrentes do mesmo cliente). Só faz sentido com
// um Querier de tx; ErrClienteNaoEncontrado sem linha.
func (r *Clientes) TravarParaVinculo(ctx context.Context, id int64) error {
	const q = `SELECT cliente_id_origem FROM clientes WHERE cliente_id_origem = ? FOR UPDATE`
	_, err := db.LerUma(r.q.QueryRowContext(ctx, q, id), comercial.ErrClienteNaoEncontrado, "comercialmysql: travar cliente", func(travado *int64) []any {
		return []any{travado}
	})
	vlog.Printf(arqClientes, "Clientes.TravarParaVinculo", "SELECT FOR UPDATE clientes encontrado=%t", err == nil)
	return err
}

// Inserir grava o cliente e preenche ClienteIDOrigem. O 1062 do CNPJ/CPF
// vira ErrCNPJDuplicado/ErrCPFDuplicado SEM o erro do driver, que traz o
// documento (N-8, M-04). CNPJ ou CPF vazio grava NULL (ECOM-06).
func (r *Clientes) Inserir(ctx context.Context, c *comercial.Cliente) error {
	const q = `
		INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo, cpf, endereco, numero, complemento, cep)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	id, err := db.Inserir(ctx, r.q, "comercialmysql: inserir cliente", q, append([]any{nuloSeVazio(c.CNPJ), c.RazaoSocial, c.Segmento, c.Cidade, c.UF, c.Bairro, c.DataCadastro, c.Ativo}, argsPF(c)...)...)
	if err = documentoDuplicado(err); err != nil {
		return err
	}
	c.ClienteIDOrigem = id
	vlog.Printf(arqClientes, "Clientes.Inserir", "INSERT clientes id=%d", id)
	return nil
}

// Atualizar grava os campos editáveis (id e ativo não mudam por aqui).
func (r *Clientes) Atualizar(ctx context.Context, id int64, c *comercial.Cliente) error {
	const q = `
		UPDATE clientes
		SET cnpj = ?, razao_social = ?, segmento = ?, cidade = ?, uf = ?, bairro = ?, data_cadastro = ?,
		    cpf = ?, endereco = ?, numero = ?, complemento = ?, cep = ?
		WHERE cliente_id_origem = ?`
	args := append([]any{nuloSeVazio(c.CNPJ), c.RazaoSocial, c.Segmento, c.Cidade, c.UF, c.Bairro, c.DataCadastro}, argsPF(c)...)
	err := db.ExecPorID(ctx, r.q, arqClientes, "Clientes.Atualizar", "comercialmysql: atualizar cliente", comercial.ErrClienteNaoEncontrado,
		q, append(args, id)...)
	return documentoDuplicado(err)
}

// argsPF são cpf e endereço (opcionais: vazio grava NULL), na ordem do SQL.
func argsPF(c *comercial.Cliente) []any {
	return []any{nuloSeVazio(c.CPF), nuloSeVazio(c.Endereco), nuloSeVazio(c.Numero), nuloSeVazio(c.Complemento), nuloSeVazio(c.CEP)}
}

// documentoDuplicado traduz o 1062 dos índices únicos de cnpj e cpf.
func documentoDuplicado(err error) error {
	switch {
	case db.ChaveDuplicada(err, uqClientesCNPJ):
		return comercial.ErrCNPJDuplicado
	case db.ChaveDuplicada(err, uqClientesCPF):
		return comercial.ErrCPFDuplicado
	}
	return err
}

// AlternarAtivo inverte o status no próprio UPDATE (BUG-22): um único
// statement, sem ler antes, então dois toggles simultâneos se compõem.
func (r *Clientes) AlternarAtivo(ctx context.Context, id int64) error {
	res, err := r.q.ExecContext(ctx, `UPDATE clientes SET ativo = NOT ativo WHERE cliente_id_origem = ?`, id)
	if err != nil {
		return wrap("alternar ativo cliente", err)
	}
	n, _ := res.RowsAffected()
	vlog.Printf(arqClientes, "Clientes.AlternarAtivo", "UPDATE clientes ativo toggle afetadas=%d", n)
	if n == 0 {
		return comercial.ErrClienteNaoEncontrado
	}
	return nil
}

// DefinirAtivo ativa/inativa o cliente. Como no legado, um erro de
// RowsAffected conta como "nenhuma linha" (404).
func (r *Clientes) DefinirAtivo(ctx context.Context, id int64, ativo bool) error {
	res, err := r.q.ExecContext(ctx, `UPDATE clientes SET ativo = ? WHERE cliente_id_origem = ?`, ativo, id)
	if err != nil {
		return wrap("definir ativo cliente", err)
	}
	n, _ := res.RowsAffected()
	vlog.Printf(arqClientes, "Clientes.DefinirAtivo", "UPDATE clientes ativo afetadas=%d", n)
	if n == 0 {
		return comercial.ErrClienteNaoEncontrado
	}
	return nil
}

// uqClientesCPF é o índice único de clientes.cpf (ECOM-05).
const uqClientesCPF = "uq_clientes_cpf"

// InserirClienteLoja grava o cliente pessoa física do autocadastro, ligado
// ao login (ECOM-05): cnpj NULL, segmento "Loja Online", data de hoje. O
// 1062 do CPF vira ErrCPFDuplicado sem o erro do driver (traz o CPF).
// BUG-30: a data vem do Go em -03, não do CURDATE() do servidor (UTC).
func (r *Clientes) InserirClienteLoja(ctx context.Context, usuarioID int64, c *comercial.ClienteLoja) (int64, error) {
	const q = `
		INSERT INTO clientes (cnpj, cpf, razao_social, segmento, cidade, uf, bairro, endereco, numero, complemento, cep, data_cadastro, ativo, usuario_id)
		VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`
	id, err := db.Inserir(ctx, r.q, "comercialmysql: inserir cliente loja", q,
		c.CPF, c.Nome, comercial.SegmentoLojaOnline, c.Cidade, c.UF, nuloSeVazio(c.Bairro), c.Endereco, c.Numero,
		nuloSeVazio(c.Complemento), nuloSeVazio(c.CEP), time.Now().In(tz.Local).Format(time.DateOnly), usuarioID)
	if db.ChaveDuplicada(err, uqClientesCPF) {
		return 0, comercial.ErrCPFDuplicado
	}
	if err != nil {
		return 0, err
	}
	vlog.Printf(arqClientes, "Clientes.InserirClienteLoja", "INSERT clientes id=%d usuario_id=%d", id, usuarioID)
	return id, nil
}

// nuloSeVazio grava NULL para o campo opcional vazio (ECOM-09).
func nuloSeVazio(s string) any {
	if s == "" {
		return nil
	}
	return s
}
