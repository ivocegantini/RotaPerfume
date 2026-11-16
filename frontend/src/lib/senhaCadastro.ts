// ECOM-01: validacao de senha do cadastro da loja (so UX; o backend decide).

const SIMBOLOS = "!@#$%&*+-=?";

/** Mesma politica do backend (identidade.ValidarForca): 8+ e 3 de 4 classes. */
export function erroSenhaCadastro(senha: string): string | undefined {
  if (!senha) return "Senha e obrigatoria";
  if (new TextEncoder().encode(senha).length > 72) return "Senha muito longa (maximo 72 bytes)";
  if ([...senha].length < 8) return "Senha deve ter pelo menos 8 caracteres";
  const classes = [/\p{Ll}/u, /\p{Lu}/u, /\p{Nd}/u].filter((re) => re.test(senha)).length +
    ([...senha].some((c) => SIMBOLOS.includes(c)) ? 1 : 0);
  if (classes < 3) return "Use ao menos 3 de: minuscula, maiuscula, numero e simbolo";
  return undefined;
}
