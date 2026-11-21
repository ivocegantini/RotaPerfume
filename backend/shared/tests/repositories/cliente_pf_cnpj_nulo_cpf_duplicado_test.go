package repositories_test

// ECOM-06: o repositório grava a pessoa física com cnpj NULL e traduz o 1062
// do CPF sem ecoar o valor.

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
)

func clientePF() *comercial.Cliente {
	return &comercial.Cliente{CPF: "52998224725", RazaoSocial: "Ana", Segmento: comercial.SegmentoLojaOnline, Cidade: "Curitiba",
		UF: "PR", DataCadastro: time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local), Ativo: true, Endereco: "Rua XV", Numero: "10"}
}

func TestECOM06_Inserir_PFGravaCNPJNulo(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	c := clientePF()
	mock.ExpectExec(reInsertClienteRepo).
		WithArgs(nil, "Ana", comercial.SegmentoLojaOnline, "Curitiba", "PR", "", c.DataCadastro, true, "52998224725", "Rua XV", "10", nil, nil).
		WillReturnResult(sqlmock.NewResult(42, 1))
	require.NoError(t, comercialmysql.NovoClientes(db).Inserir(context.Background(), c))
	assert.Equal(t, int64(42), c.ClienteIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM06_Atualizar_CPFDuplicado(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec(`UPDATE clientes`).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry '52998224725' for key 'clientes.uq_clientes_cpf'"})
	err = comercialmysql.NovoClientes(db).Atualizar(context.Background(), 7, clientePF())
	assert.ErrorIs(t, err, comercial.ErrCPFDuplicado)
	assert.NotContains(t, err.Error(), "52998224725")
}
