package produtos

// ECOM-11: descrição completa gerada no seed (importer) a partir das
// características do produto: categoria, nota olfativa, marca, embalagem e
// lançamento. Determinística (mesmo CSV, mesmo texto) e sempre com pelo
// menos MinDescricaoGerada caracteres.

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MinDescricaoGerada é o tamanho mínimo (em caracteres) da descrição gerada.
const MinDescricaoGerada = 500

var textoCategoria = map[string]string{
	"Eau de Parfum":       "Com alta concentração de essências, a Eau de Parfum oferece fixação prolongada e uma evolução marcante ao longo do dia, das notas de saída ao fundo que permanece na pele por horas.",
	"Óleo Concentrado":    "Sem álcool na composição, o óleo concentrado é aplicado em pequenas quantidades nos pontos de pulsação e se funde ao calor da pele, criando uma assinatura olfativa íntima e duradoura.",
	"Kit Presente":        "Montado para presentear, o kit reúne itens que se complementam no ritual de cuidado e perfumação, em uma apresentação elegante, pronta para datas especiais.",
	"Body Splash":         "Leve e refrescante, o body splash pode ser borrifado várias vezes ao dia sobre o corpo e os cabelos, deixando um rastro suave e confortável, ideal para dias quentes.",
	"Bakhoor":             "Tradicional da cultura árabe, o bakhoor é queimado sobre carvão ou em queimador elétrico e libera uma fumaça aromática que perfuma ambientes, roupas e tecidos.",
	"Água Perfumada":      "Mais delicada que um perfume, a água perfumada traz uma fragrância limpa e versátil, que pode ser usada no corpo, nas roupas e na roupa de cama.",
	"Incenso":             "Produzido para queima lenta e uniforme, o incenso aromatiza o ambiente de forma gradual e ajuda a compor momentos de meditação, leitura ou descanso.",
	"Hidratante Corporal": "Com textura de rápida absorção, o hidratante corporal nutre a pele e deixa um toque sedoso, ao mesmo tempo em que prolonga a fragrância aplicada depois do banho.",
	"Difusor de Ambiente": "Com varetas que absorvem e liberam o aroma de forma contínua, o difusor perfuma salas, quartos e escritórios por semanas, sem uso de chama ou energia.",
	"Sabonete Artesanal":  "Feito de forma artesanal, o sabonete limpa a pele com suavidade e deixa um perfume delicado após o banho, transformando a higiene diária em um momento de cuidado.",
}

var textoNota = map[string]string{
	"Cedro":     "O cedro traz um acorde amadeirado seco e elegante, que transmite firmeza, sofisticação e uma sensação de natureza preservada.",
	"Sândalo":   "O sândalo confere um fundo cremoso, aveludado e levemente adocicado, conhecido pela sensação de calma e acolhimento.",
	"Cardamomo": "O cardamomo abre a fragrância com um frescor especiado e aromático, ao mesmo tempo vibrante e caloroso, típico da perfumaria oriental.",
	"Rosa":      "A rosa revela um floral clássico e romântico, com nuances aveludadas que equilibram delicadeza e presença.",
	"Patchouli": "O patchouli entrega um caráter terroso e intenso, com profundidade marcante que dá personalidade e longa duração à composição.",
	"Oud":       "O oud, uma das matérias-primas mais nobres do Oriente, traz um acorde amadeirado escuro, resinoso e envolvente, de grande sofisticação.",
	"Jasmim":    "O jasmim oferece um floral branco luminoso e sensual, de perfil exuberante, que perfuma com elegância do dia à noite.",
	"Açafrão":   "O açafrão adiciona um toque especiado, levemente couro e dourado, que remete às especiarias preciosas dos mercados árabes.",
	"Almíscar":  "O almíscar cria um efeito de pele limpa, macia e sensual, que amplia a fixação e deixa um rastro discreto e confortável.",
	"Baunilha":  "A baunilha traz doçura cremosa e gourmand, aquecendo a composição com um toque aconchegante e irresistível.",
	"Âmbar":     "O âmbar envolve a fragrância em um calor resinoso e balsâmico, com nuances adocicadas que transmitem luxo e intensidade.",
	"Incenso":   "O incenso confere um acorde fumê e místico, com nuances resinosas que lembram templos antigos e rituais de contemplação.",
}

const (
	textoCategoriaPadrao = "É um produto da perfumaria árabe pensado para o uso diário, com fragrância equilibrada e acabamento cuidadoso, que valoriza cada momento de uso."
	textoNotaPadrao      = "Sua composição olfativa equilibra notas de saída, de coração e de fundo, criando uma fragrância harmoniosa e marcante do início ao fim."
)

// textoUnidade descreve a embalagem de venda (UN, KIT n, CX n, DISPLAY n).
func textoUnidade(unidade string) string {
	partes := strings.Fields(strings.ToUpper(unidade))
	if len(partes) == 2 {
		switch partes[0] {
		case "KIT":
			return fmt.Sprintf("Vendido em kit com %s unidades, ótima opção para presentear ou para manter sempre uma reserva da fragrância favorita.", partes[1])
		case "CX":
			return fmt.Sprintf("Vendido em caixa com %s unidades, formato indicado para revenda, uso compartilhado ou para quem não quer ficar sem o produto.", partes[1])
		case "DISPLAY":
			return fmt.Sprintf("Vendido em display com %s unidades, pensado para exposição em lojas e para revendedores que desejam destacar o produto no ponto de venda.", partes[1])
		}
	}
	if partes := strings.Fields(unidade); len(partes) == 1 && strings.EqualFold(partes[0], "UN") {
		return "Vendido por unidade, para que você experimente a fragrância ou incorpore o produto à sua rotina de cuidado."
	}
	return fmt.Sprintf("Embalagem de venda: %s.", unidade)
}

// GerarDescricao monta a descrição completa da página do produto.
func GerarDescricao(r Row) string {
	cat, ok := textoCategoria[r.Categoria]
	if !ok {
		cat = textoCategoriaPadrao
	}
	nota := textoNotaPadrao
	destaque := ""
	if r.NotaOlfativa != "" {
		destaque = fmt.Sprintf(", com destaque para a nota de %s", strings.ToLower(r.NotaOlfativa))
		if t, ok := textoNota[r.NotaOlfativa]; ok {
			nota = t
		}
	}
	paragrafos := []string{
		fmt.Sprintf("%s é um produto da linha %s da marca %s%s. %s", r.Nome, r.Categoria, r.Marca, destaque, cat),
		nota + " O resultado é uma fragrância de identidade própria, inspirada na tradição da perfumaria árabe.",
		textoUnidade(r.Unidade) + fmt.Sprintf(" A %s seleciona matérias-primas de qualidade e cuida de cada etapa da produção, do envase ao acabamento da embalagem.", r.Marca),
	}
	fecho := "Guarde em local fresco, seco e ao abrigo da luz solar direta para preservar as características da fragrância. Produto para uso externo; em caso de irritação, suspenda o uso."
	if r.DataLancamento != nil {
		fecho = fmt.Sprintf("Lançado em %s. %s", r.DataLancamento.Format("01/2006"), fecho)
	}
	paragrafos = append(paragrafos, fecho)
	texto := strings.Join(paragrafos, "\n\n")
	if utf8.RuneCountInString(texto) < MinDescricaoGerada { // garantia (textos padrão já passam)
		texto += "\n\n" + textoCategoriaPadrao + " " + textoNotaPadrao
	}
	return texto
}
