package kernel

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Dinheiro é um valor monetário exato em centavos (ARQ-09). JSON e banco
// continuam em reais decimais (123.45); a conversão nunca passa por float64.
// O sinal é livre (o banco e os CSVs têm devoluções); a entrada da API não.
type Dinheiro struct {
	centavos int64
}

// LimiteCentavos é o maior valor de DECIMAL(15,2) (9999999999999.99): teto
// da entrada da API e dos resultados de Somar e MultiplicarPor.
const LimiteCentavos int64 = 999_999_999_999_999

// Erros de Dinheiro: o domínio os traduz nas mensagens públicas do campo.
var (
	ErrDinheiroInvalido      = NovoErro(KindInvalido, "valor monetário inválido")
	ErrDinheiroCasasDecimais = NovoErro(KindInvalido, "valor monetário com mais de 2 casas decimais")
	ErrDinheiroForaDoLimite  = NovoErro(KindInvalido, "valor monetário fora do limite")
)

// DinheiroDeCentavos valida 0 ≤ c ≤ LimiteCentavos.
func DinheiroDeCentavos(c int64) (Dinheiro, error) {
	if c < 0 {
		return Dinheiro{}, ErrDinheiroInvalido
	}
	if c > LimiteCentavos {
		return Dinheiro{}, ErrDinheiroForaDoLimite
	}
	return Dinheiro{centavos: c}, nil
}

// ParseDinheiro lê a entrada da API: número decimal (gramática JSON) ≥ 0,
// com no máximo 2 casas (zeros à direita não contam) e até LimiteCentavos.
func ParseDinheiro(s string) (Dinheiro, error) {
	d, err := LerDinheiro(s)
	if err != nil {
		return Dinheiro{}, err
	}
	return DinheiroDeCentavos(d.centavos)
}

// LerDinheiro lê um decimal com sinal do banco ou de CSV (ex.: "-12.30"),
// exato: mais de 2 casas é erro (nunca arredonda) e o teto é o int64.
func LerDinheiro(s string) (Dinheiro, error) {
	neg, digitos, exp, ok := decompor(s)
	if !ok {
		return Dinheiro{}, ErrDinheiroInvalido
	}
	c, err := centavos(digitos, exp)
	if err != nil {
		return Dinheiro{}, err
	}
	if neg {
		c = -c
	}
	return Dinheiro{centavos: c}, nil
}

// decompor valida s na gramática de número JSON e devolve valor =
// (-1)^neg × digitos × 10^exp.
func decompor(s string) (neg bool, digitos string, exp int, ok bool) {
	i := 0
	if i < len(s) && s[i] == '-' {
		neg, i = true, 1
	}
	ini := i
	switch {
	case i < len(s) && s[i] == '0':
		i++
	case i < len(s) && s[i] >= '1' && s[i] <= '9':
		i = pularDigitos(s, i)
	default:
		return false, "", 0, false
	}
	digitos = s[ini:i]
	if i < len(s) && s[i] == '.' {
		f := pularDigitos(s, i+1)
		if f == i+1 {
			return false, "", 0, false
		}
		digitos += s[i+1 : f]
		exp, i = -(f - i - 1), f
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		e, fim, okExp := expoente(s, i+1)
		if !okExp {
			return false, "", 0, false
		}
		exp, i = exp+e, fim
	}
	return neg, digitos, exp, i == len(s)
}

// expoente lê [+-]dígitos a partir de i; acima de 9 dígitos satura em
// ±1e9 (sobra para recusar como casas demais ou fora do limite).
func expoente(s string, i int) (e, fim int, ok bool) {
	sinal := 1
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sinal = -1
		}
		i++
	}
	fim = pularDigitos(s, i)
	if fim == i {
		return 0, 0, false
	}
	txt := strings.TrimLeft(s[i:fim], "0")
	if len(txt) > 9 {
		return sinal * 1_000_000_000, fim, true
	}
	n, _ := strconv.Atoi("0" + txt)
	return sinal * n, fim, true
}

func pularDigitos(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// centavos converte digitos × 10^exp em centavos exatos (sem sinal).
func centavos(digitos string, exp int) (int64, error) {
	digitos = strings.TrimLeft(digitos, "0")
	if digitos == "" {
		return 0, nil
	}
	for strings.HasSuffix(digitos, "0") {
		digitos, exp = digitos[:len(digitos)-1], exp+1
	}
	casas := exp + 2 // expoente em centavos
	if casas < 0 {
		return 0, ErrDinheiroCasasDecimais
	}
	if len(digitos)+casas > 19 {
		return 0, ErrDinheiroForaDoLimite
	}
	n, err := strconv.ParseUint(digitos+strings.Repeat("0", casas), 10, 64)
	if err != nil || n > math.MaxInt64 {
		return 0, ErrDinheiroForaDoLimite
	}
	return int64(n), nil
}

// Centavos devolve o valor em centavos.
func (d Dinheiro) Centavos() int64 { return d.centavos }

// String devolve o decimal com 2 casas ("123.40"), o argumento de SQL.
func (d Dinheiro) String() string {
	sinal, c := "", d.centavos
	if c < 0 {
		sinal = "-"
	}
	u := uint64(c)
	if c < 0 {
		u = uint64(-(c + 1)) + 1 // sem estouro em MinInt64
	}
	return sinal + strconv.FormatUint(u/100, 10) + "." + strconv.FormatUint(u%100+100, 10)[1:]
}

// Reais devolve o valor em float64 só para estatísticas (médias e
// percentuais); nunca para somar ou gravar dinheiro.
func (d Dinheiro) Reais() float64 { return float64(d.centavos) / 100 }

// Somar devolve d + o; fora de ±LimiteCentavos é ErrDinheiroForaDoLimite.
func (d Dinheiro) Somar(o Dinheiro) (Dinheiro, error) {
	r, err := limitar(new(big.Int).Add(big.NewInt(d.centavos), big.NewInt(o.centavos)))
	return Dinheiro{centavos: r}, err
}

// MultiplicarPor devolve d × f arredondado ao centavo, metade para longe do
// zero; fora de ±LimiteCentavos é ErrDinheiroForaDoLimite.
func (d Dinheiro) MultiplicarPor(f *big.Rat) (Dinheiro, error) {
	num := new(big.Int).Mul(big.NewInt(d.centavos), f.Num())
	den := f.Denom()
	q, r := new(big.Int).QuoRem(new(big.Int).Abs(num), den, new(big.Int))
	if r.Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if num.Sign() < 0 {
		q.Neg(q)
	}
	c, err := limitar(q)
	return Dinheiro{centavos: c}, err
}

func limitar(v *big.Int) (int64, error) {
	if v.CmpAbs(big.NewInt(LimiteCentavos)) > 0 {
		return 0, ErrDinheiroForaDoLimite
	}
	return v.Int64(), nil
}

// MarshalJSON emite o número sem zeros à direita (123.4, 100), byte a byte
// igual ao float64 de antes do ARQ-09.
func (d Dinheiro) MarshalJSON() ([]byte, error) {
	s := strings.TrimSuffix(strings.TrimRight(d.String(), "0"), ".")
	return []byte(s), nil
}

// UnmarshalJSON aceita só número JSON válido para a entrada (ParseDinheiro);
// null não altera o valor, como no float64.
func (d *Dinheiro) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	v, err := ParseDinheiro(string(b))
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// errScanDinheiro sinaliza tipo de coluna que não é DECIMAL nem inteiro.
var errScanDinheiro = errors.New("kernel: Dinheiro só lê DECIMAL (texto) ou inteiro")

// Scan (sql.Scanner) lê o DECIMAL como texto, exato. NULL (SUM sem
// linhas) vira zero, como o sql.NullFloat64 do legado; float64 é recusado.
func (d *Dinheiro) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = Dinheiro{}
		return nil
	case []byte:
		return d.lerTexto(string(v))
	case string:
		return d.lerTexto(v)
	case int64:
		return d.lerTexto(strconv.FormatInt(v, 10))
	}
	return errScanDinheiro
}

// lerTexto não embrulha o kernel.Erro: valor ruim no banco é 500, nunca 400.
func (d *Dinheiro) lerTexto(s string) error {
	v, err := LerDinheiro(s)
	if err != nil {
		return errors.New("kernel: DECIMAL fora de Dinheiro: " + err.Error())
	}
	*d = v
	return nil
}

// Decimal é um número JSON guardado como texto (entrada da API, ARQ-09): o
// domínio o converte com Dinheiro, sem passar por float64.
type Decimal string

// UnmarshalJSON aceita só número JSON (string, bool ou objeto falham como
// no float64: 400 "body JSON inválido"); null não altera o valor.
func (v *Decimal) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		return nil
	}
	if _, _, _, ok := decompor(s); !ok {
		return ErrDinheiroInvalido
	}
	*v = Decimal(s)
	return nil
}

// Dinheiro converte a entrada (vazio = 0, campo ausente): mais de 2 casas
// vira errCasas; negativo, inválido ou fora do limite, errInvalido.
func (v Decimal) Dinheiro(errInvalido, errCasas error) (Dinheiro, error) {
	if v == "" {
		return Dinheiro{}, nil
	}
	d, err := ParseDinheiro(string(v))
	switch {
	case errors.Is(err, ErrDinheiroCasasDecimais):
		return Dinheiro{}, errCasas
	case err != nil:
		return Dinheiro{}, errInvalido
	}
	return d, nil
}
