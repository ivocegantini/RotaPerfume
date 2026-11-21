package kernel_test

// ARQ-09: Dinheiro em centavos int64. Entrada exata (sem float64), até 2
// casas, sem negativo, NaN/Inf ou estouro; JSON igual ao float64 de antes.

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
)

func TestARQ09_ParseDinheiro(t *testing.T) {
	validos := map[string]int64{
		"0": 0, "-0": 0, "0.00": 0, "0.1": 10, "0.01": 1, "123.45": 12345, "100": 10000,
		"10.500": 1050, "1e2": 10000, "1.5E+1": 1500, "12345e-2": 12345, "0e999999999999": 0,
		"9999999999999.99": kernel.LimiteCentavos,
	}
	for s, c := range validos {
		d, err := kernel.ParseDinheiro(s)
		require.NoError(t, err, s)
		assert.Equal(t, c, d.Centavos(), s)
	}
	recusados := map[string]error{
		"-0.01": kernel.ErrDinheiroInvalido, "-1": kernel.ErrDinheiroInvalido,
		"0.001": kernel.ErrDinheiroCasasDecimais, "1.005": kernel.ErrDinheiroCasasDecimais,
		"1e-3": kernel.ErrDinheiroCasasDecimais, "1e-999999999999": kernel.ErrDinheiroCasasDecimais,
		"10000000000000": kernel.ErrDinheiroForaDoLimite, "1e400": kernel.ErrDinheiroForaDoLimite,
		"99999999999999999999": kernel.ErrDinheiroForaDoLimite,
		"":                     kernel.ErrDinheiroInvalido, "NaN": kernel.ErrDinheiroInvalido, "Inf": kernel.ErrDinheiroInvalido,
		"+1": kernel.ErrDinheiroInvalido, "01": kernel.ErrDinheiroInvalido, ".5": kernel.ErrDinheiroInvalido,
		"5.": kernel.ErrDinheiroInvalido, "1,5": kernel.ErrDinheiroInvalido, " 1": kernel.ErrDinheiroInvalido,
		"1e": kernel.ErrDinheiroInvalido, "0x10": kernel.ErrDinheiroInvalido, `"1"`: kernel.ErrDinheiroInvalido,
	}
	for s, e := range recusados {
		_, err := kernel.ParseDinheiro(s)
		assert.ErrorIs(t, err, e, "%q", s)
	}
}

func TestARQ09_LerDinheiro_ComSinalEAteInt64(t *testing.T) {
	d, err := kernel.LerDinheiro("-1000.20")
	require.NoError(t, err)
	assert.Equal(t, int64(-100020), d.Centavos())
	assert.Equal(t, "-1000.20", d.String())
	d, err = kernel.LerDinheiro("92233720368547758.07") // MaxInt64 centavos
	require.NoError(t, err)
	assert.Equal(t, int64(math.MaxInt64), d.Centavos())
	_, err = kernel.LerDinheiro("92233720368547758.08")
	assert.ErrorIs(t, err, kernel.ErrDinheiroForaDoLimite)
	_, err = kernel.LerDinheiro("1.234")
	assert.ErrorIs(t, err, kernel.ErrDinheiroCasasDecimais, "nunca arredonda na leitura")
}

func TestARQ09_DinheiroDeCentavos(t *testing.T) {
	d, err := kernel.DinheiroDeCentavos(1)
	require.NoError(t, err)
	assert.Equal(t, "0.01", d.String())
	_, err = kernel.DinheiroDeCentavos(-1)
	assert.ErrorIs(t, err, kernel.ErrDinheiroInvalido)
	_, err = kernel.DinheiroDeCentavos(kernel.LimiteCentavos + 1)
	assert.ErrorIs(t, err, kernel.ErrDinheiroForaDoLimite)
}

func TestARQ09_String(t *testing.T) {
	casos := map[int64]string{0: "0.00", 5: "0.05", 12340: "123.40", -150: "-1.50", math.MaxInt64: "92233720368547758.07"}
	for c, s := range casos {
		d, err := kernel.LerDinheiro(s)
		require.NoError(t, err)
		assert.Equal(t, c, d.Centavos())
		assert.Equal(t, s, d.String())
	}
}

// O JSON sai byte a byte igual ao float64 de antes do ARQ-09.
func TestARQ09_MarshalJSON_IgualAoFloat64(t *testing.T) {
	for _, s := range []string{"0", "0.01", "0.1", "0.3", "1.5", "10", "123.4", "123.45", "100", "-1.5", "9999999999999.99", "89.99"} {
		d, err := kernel.LerDinheiro(s)
		require.NoError(t, err)
		f, _ := strconv.ParseFloat(s, 64)
		esperado, _ := json.Marshal(f)
		obtido, err := json.Marshal(d)
		require.NoError(t, err)
		assert.Equal(t, string(esperado), string(obtido), s)
	}
}

func TestARQ09_UnmarshalJSON(t *testing.T) {
	var v struct {
		D kernel.Dinheiro `json:"d"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"d":12.34}`), &v))
	assert.Equal(t, int64(1234), v.D.Centavos())
	require.NoError(t, json.Unmarshal([]byte(`{"d":null}`), &v))
	assert.Equal(t, int64(1234), v.D.Centavos(), "null não altera, como no float64")
	for _, b := range []string{`{"d":-1}`, `{"d":1.234}`, `{"d":"12"}`, `{"d":true}`, `{"d":1e400}`} {
		assert.Error(t, json.Unmarshal([]byte(b), &v), b)
	}
}

func TestARQ09_Decimal(t *testing.T) {
	var v struct {
		D kernel.Decimal `json:"d"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"d":0.10}`), &v))
	assert.Equal(t, kernel.Decimal("0.10"), v.D, "texto do número, sem float64")
	require.NoError(t, json.Unmarshal([]byte(`{"d":null}`), &v))
	assert.Equal(t, kernel.Decimal("0.10"), v.D)
	for _, b := range []string{`{"d":"1"}`, `{"d":true}`, `{"d":{}}`, `{"d":[1]}`} {
		assert.Error(t, json.Unmarshal([]byte(b), &v), b)
	}

	errInv := kernel.NovoErro(kernel.KindInvalido, "x inválido")
	errCasas := kernel.NovoErro(kernel.KindInvalido, "x casas")
	d, err := kernel.Decimal("").Dinheiro(errInv, errCasas)
	require.NoError(t, err, "campo ausente = 0")
	assert.Equal(t, int64(0), d.Centavos())
	d, err = kernel.Decimal("19.9").Dinheiro(errInv, errCasas)
	require.NoError(t, err)
	assert.Equal(t, int64(1990), d.Centavos())
	_, err = kernel.Decimal("1.999").Dinheiro(errInv, errCasas)
	assert.Same(t, errCasas, err)
	for _, s := range []string{"-1", "1e20", "abc"} {
		_, err = kernel.Decimal(s).Dinheiro(errInv, errCasas)
		assert.Same(t, errInv, err, s)
	}
}

func TestARQ09_Somar(t *testing.T) {
	a, _ := kernel.ParseDinheiro("0.1")
	b, _ := kernel.ParseDinheiro("0.2")
	s, err := a.Somar(b)
	require.NoError(t, err)
	assert.Equal(t, "0.30", s.String(), "0.1 + 0.2 exato")

	max, _ := kernel.DinheiroDeCentavos(kernel.LimiteCentavos)
	um, _ := kernel.DinheiroDeCentavos(1)
	_, err = max.Somar(um)
	assert.ErrorIs(t, err, kernel.ErrDinheiroForaDoLimite, "nunca wrap silencioso")
	enorme, _ := kernel.LerDinheiro("92233720368547758.07")
	_, err = enorme.Somar(enorme)
	assert.ErrorIs(t, err, kernel.ErrDinheiroForaDoLimite)
}

// Metade para longe do zero, nos dois sinais.
func TestARQ09_MultiplicarPor(t *testing.T) {
	casos := []struct {
		valor, fator, esperado string
	}{
		{"33.33", "27/10", "89.99"}, // 3 × 33.33 × 0,9 = 89,991
		{"0.05", "1/2", "0.03"},     // 0,025 → 0,03
		{"0.01", "1/2", "0.01"},     // 0,005 → 0,01
		{"0.01", "49/100", "0.00"},  // 0,0049 → 0,00
		{"-0.05", "1/2", "-0.03"},   // simétrico
		{"10.00", "0", "0.00"},
		{"19.90", "3", "59.70"},
	}
	for _, c := range casos {
		v, err := kernel.LerDinheiro(c.valor)
		require.NoError(t, err)
		f, ok := new(big.Rat).SetString(c.fator)
		require.True(t, ok)
		r, err := v.MultiplicarPor(f)
		require.NoError(t, err, c)
		assert.Equal(t, c.esperado, r.String(), c)
	}
	max, _ := kernel.DinheiroDeCentavos(kernel.LimiteCentavos)
	_, err := max.MultiplicarPor(big.NewRat(2, 1))
	assert.ErrorIs(t, err, kernel.ErrDinheiroForaDoLimite)
}

// Scan lê o DECIMAL como texto ([]byte do driver), exato; float64 é recusado.
func TestARQ09_Scan(t *testing.T) {
	var d kernel.Dinheiro
	require.NoError(t, d.Scan([]byte("1234.56")))
	assert.Equal(t, int64(123456), d.Centavos())
	require.NoError(t, d.Scan("-0.10"))
	assert.Equal(t, int64(-10), d.Centavos())
	require.NoError(t, d.Scan(int64(7)))
	assert.Equal(t, int64(700), d.Centavos())
	require.NoError(t, d.Scan(nil))
	assert.Equal(t, int64(0), d.Centavos(), "NULL de SUM vira zero")
	assert.Error(t, d.Scan(12.5), "float64 nunca")
	assert.Error(t, d.Scan(true))
	assert.Error(t, d.Scan([]byte("1.005")), "mais de 2 casas não arredonda")
	assert.Error(t, d.Scan(int64(math.MaxInt64)))
	// Valor ruim no banco é 500: o erro do Scan não carrega o kernel.Erro (400).
	var ke *kernel.Erro
	assert.False(t, errors.As(d.Scan([]byte("1.005")), &ke))
	assert.False(t, errors.As(d.Scan("-"), &ke))
}

func TestARQ09_Reais_SoParaEstatistica(t *testing.T) {
	d, _ := kernel.ParseDinheiro("123.45")
	assert.Equal(t, 123.45, d.Reais())
}

// Entrada gigante (corpo de 256 KB) não aloca nem trava.
func TestARQ09_ParseDinheiro_EntradaGigante(t *testing.T) {
	_, err := kernel.ParseDinheiro("1" + strings.Repeat("0", 200_000))
	assert.ErrorIs(t, err, kernel.ErrDinheiroForaDoLimite)
	d, err := kernel.ParseDinheiro("0." + strings.Repeat("0", 200_000))
	require.NoError(t, err)
	assert.Equal(t, int64(0), d.Centavos())
}
