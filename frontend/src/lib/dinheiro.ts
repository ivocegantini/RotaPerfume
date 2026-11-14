/**
 * FE-19: validador unico dos campos de dinheiro, espelho do ParseDinheiro do
 * backend (ARQ-09): decimal >= 0, no maximo 2 casas (zeros a direita nao
 * contam) e ate DECIMAL(15,2). Le o texto do input, nunca o float.
 */
import { vlog } from "./vlog";

const F = "dinheiro.ts";

/** Maior valor de DECIMAL(15,2), em centavos (LimiteCentavos do backend). */
export const DINHEIRO_LIMITE_CENTAVOS = BigInt("999999999999999");
/** Texto do teto para as mensagens. */
export const DINHEIRO_LIMITE_TEXTO = "9.999.999.999.999,99";

const DECIMAL_REGEX = /^(-?)(\d*)(?:\.(\d*))?(?:[eE]([+-]?\d+))?$/;

export type ResultadoDinheiro =
  | { ok: true; valor: number }
  | { ok: false; erro: string };

/** Mensagens amigaveis; `rotulo` e o nome do campo (ex.: "Valor"). */
export const msgDinheiroInvalido = (rotulo: string, sufixo = "") =>
  `${rotulo} deve ser um numero maior ou igual a zero${sufixo}.`;
export const msgDinheiroCasas = (rotulo: string, sufixo = "") =>
  `${rotulo} deve ter no maximo 2 casas decimais${sufixo}.`;
export const msgDinheiroLimite = (rotulo: string, sufixo = "") =>
  `${rotulo} deve ser no maximo ${DINHEIRO_LIMITE_TEXTO}${sufixo}.`;

/**
 * Valida o texto de um campo de dinheiro. `sufixo` completa a mensagem
 * (ex.: " em todos os itens"). Em caso de sucesso devolve o numero a enviar.
 */
export function validarDinheiro(
  texto: string,
  rotulo: string,
  sufixo = ""
): ResultadoDinheiro {
  vlog(F, "validarDinheiro", "validando campo, tamanho:", texto.length);
  const s = texto.trim();
  const m = DECIMAL_REGEX.exec(s);
  const inteiro = m?.[2] ?? "";
  const fracao = m?.[3] ?? "";
  if (!m || (inteiro === "" && fracao === "")) {
    vlog(F, "validarDinheiro", "formato invalido");
    return { ok: false, erro: msgDinheiroInvalido(rotulo, sufixo) };
  }
  const exp = m[4] ? Number(m[4]) : 0;
  // valor = digitos x 10^(exp - casas da fracao); zeros a direita nao contam
  const digitosBrutos = (inteiro + fracao).replace(/^0+/, "");
  const digitos = digitosBrutos.replace(/0+$/, "");
  const zerosRetirados = digitosBrutos.length - digitos.length;
  const expEfetivo = exp - fracao.length + zerosRetirados;
  if (digitos === "") {
    vlog(F, "validarDinheiro", "valor zero");
    return { ok: true, valor: 0 };
  }
  if (m[1] === "-") {
    vlog(F, "validarDinheiro", "valor negativo");
    return { ok: false, erro: msgDinheiroInvalido(rotulo, sufixo) };
  }
  if (expEfetivo < -2) {
    vlog(F, "validarDinheiro", "mais de 2 casas decimais");
    return { ok: false, erro: msgDinheiroCasas(rotulo, sufixo) };
  }
  // centavos = digitos x 10^(expEfetivo + 2); corta cedo expoentes enormes
  const potencia = expEfetivo + 2;
  if (digitos.length + potencia > 15) {
    vlog(F, "validarDinheiro", "acima do limite");
    return { ok: false, erro: msgDinheiroLimite(rotulo, sufixo) };
  }
  const centavos = BigInt(digitos) * BigInt(10) ** BigInt(potencia);
  if (centavos > DINHEIRO_LIMITE_CENTAVOS) {
    vlog(F, "validarDinheiro", "acima do limite");
    return { ok: false, erro: msgDinheiroLimite(rotulo, sufixo) };
  }
  vlog(F, "validarDinheiro", "valor valido");
  return { ok: true, valor: Number(centavos) / 100 };
}
