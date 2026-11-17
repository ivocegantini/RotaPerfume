package repositories_test

// BUG-30 (MERGE-01): o cliente da loja grava data_cadastro com a data do Go
// em -03, não com o CURDATE() do servidor (UTC adianta 1 dia entre 21h e 24h).

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/tz"
)

func TestBUG30_InserirClienteLoja_DataDoGoEmMenos3(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	c := &comercial.ClienteLoja{Nome: "Ana", CPF: "52998224725", Endereco: "Rua XV", Numero: "10", Cidade: "Curitiba", UF: "PR"}
	antes := time.Now().In(tz.Local).Format(time.DateOnly)
	mock.ExpectExec(`INSERT INTO clientes \(cnpj, cpf, .*data_cadastro, ativo, usuario_id\)\s+VALUES \(NULL, \?, \?, \?, \?, \?, \?, \?, \?, \?, \?, \?, 1, \?\)$`).
		WithArgs("52998224725", "Ana", comercial.SegmentoLojaOnline, "Curitiba", "PR", nil, "Rua XV", "10", nil, nil, sqlmock.AnyArg(), int64(9)).
		WillReturnResult(sqlmock.NewResult(5, 1))
	id, err := comercialmysql.NovoClientes(db).InserirClienteLoja(context.Background(), 9, c)
	require.NoError(t, err)
	assert.Equal(t, int64(5), id)
	assert.NoError(t, mock.ExpectationsWereMet())

	// A data passada é a de hoje em -03 (tolerância para a virada do dia).
	depois := time.Now().In(tz.Local).Format(time.DateOnly)
	mock.ExpectExec(`INSERT INTO clientes`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), diaEntre{antes, depois}, int64(9)).
		WillReturnResult(sqlmock.NewResult(6, 1))
	_, err = comercialmysql.NovoClientes(db).InserirClienteLoja(context.Background(), 9, c)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// diaEntre casa a string de data igual a um dos dois limites.
type diaEntre struct{ a, b string }

func (d diaEntre) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && (s == d.a || s == d.b)
}
