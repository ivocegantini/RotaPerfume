// Log verbose de rastreamento (LOG-02).
// NEXT_PUBLIC_VERBOSE é embutido no bundle em tempo de build: em produção
// deve ficar ausente ou "false" para o bundler eliminar as chamadas.
// Nunca passe valores sensíveis (senhas, tokens, captcha, User, e-mail, CNPJ,
// corpo de request/response) — ver LOG-02 spec de segurança §3.
const VERBOSE = process.env.NEXT_PUBLIC_VERBOSE === "true";

const doisDig = (n: number) => String(n).padStart(2, "0");

// LOG-04: data e hora locais com segundos, no mesmo formato do log do Go
// ("2026/11/13 14:05:09"), no início de toda linha de log do frontend.
export function carimbo(d: Date = new Date()): string {
  return (
    `${d.getFullYear()}/${doisDig(d.getMonth() + 1)}/${doisDig(d.getDate())} ` +
    `${doisDig(d.getHours())}:${doisDig(d.getMinutes())}:${doisDig(d.getSeconds())}`
  );
}

export function vlog(arquivo: string, funcao: string, msg: string, ...args: unknown[]): void {
  if (!VERBOSE) return;
  console.log(`${carimbo()} [${arquivo}] [${funcao}] ${msg}`, ...args);
}
