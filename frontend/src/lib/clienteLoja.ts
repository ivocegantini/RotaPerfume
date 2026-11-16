// ECOM-05: validacao (so UX; o backend decide) do cliente pessoa fisica do
// cadastro da loja: CPF com digitos verificadores, CEP e UF.

// ECOM-07: o select mostra o nome do estado; o valor enviado e a sigla.
export const UFS_BRASIL: { sigla: string; nome: string }[] = [
  { sigla: "AC", nome: "Acre" },
  { sigla: "AL", nome: "Alagoas" },
  { sigla: "AP", nome: "Amapá" },
  { sigla: "AM", nome: "Amazonas" },
  { sigla: "BA", nome: "Bahia" },
  { sigla: "CE", nome: "Ceará" },
  { sigla: "DF", nome: "Distrito Federal" },
  { sigla: "ES", nome: "Espírito Santo" },
  { sigla: "GO", nome: "Goiás" },
  { sigla: "MA", nome: "Maranhão" },
  { sigla: "MT", nome: "Mato Grosso" },
  { sigla: "MS", nome: "Mato Grosso do Sul" },
  { sigla: "MG", nome: "Minas Gerais" },
  { sigla: "PA", nome: "Pará" },
  { sigla: "PB", nome: "Paraíba" },
  { sigla: "PR", nome: "Paraná" },
  { sigla: "PE", nome: "Pernambuco" },
  { sigla: "PI", nome: "Piauí" },
  { sigla: "RJ", nome: "Rio de Janeiro" },
  { sigla: "RN", nome: "Rio Grande do Norte" },
  { sigla: "RS", nome: "Rio Grande do Sul" },
  { sigla: "RO", nome: "Rondônia" },
  { sigla: "RR", nome: "Roraima" },
  { sigla: "SC", nome: "Santa Catarina" },
  { sigla: "SP", nome: "São Paulo" },
  { sigla: "SE", nome: "Sergipe" },
  { sigla: "TO", nome: "Tocantins" },
];

const soDigitos = (s: string) => s.replace(/\D/g, "");
const soMascaraNumerica = (s: string) => /^[0-9.\- ]*$/.test(s);

/** CPF: 11 digitos, nao todos iguais, DVs corretos (mascara aceita). */
export function cpfValido(bruto: string): boolean {
  if (!soMascaraNumerica(bruto)) return false;
  const cpf = soDigitos(bruto);
  if (cpf.length !== 11 || /^(\d)\1{10}$/.test(cpf)) return false;
  const dv = (n: number) => {
    let soma = 0;
    for (let i = 0; i < n; i++) soma += Number(cpf[i]) * (n + 1 - i);
    const r = (soma * 10) % 11;
    return r === 10 ? 0 : r;
  };
  return dv(9) === Number(cpf[9]) && dv(10) === Number(cpf[10]);
}

/** Mascara de exibicao 000.000.000-00 enquanto digita. */
export function mascararCPF(bruto: string): string {
  const d = soDigitos(bruto).slice(0, 11);
  return d
    .replace(/^(\d{3})(\d)/, "$1.$2")
    .replace(/^(\d{3})\.(\d{3})(\d)/, "$1.$2.$3")
    .replace(/\.(\d{3})(\d{1,2})$/, ".$1-$2");
}

/** Mascara de exibicao 00000-000. */
export function mascararCEP(bruto: string): string {
  const d = soDigitos(bruto).slice(0, 8);
  return d.length > 5 ? `${d.slice(0, 5)}-${d.slice(5)}` : d;
}

/** ECOM-09: CEP e opcional; vazio vale, preenchido precisa de 8 digitos. */
export function cepOpcionalValido(bruto: string): boolean {
  return bruto.trim() === "" || cepValido(bruto);
}

export function cepValido(bruto: string): boolean {
  return soMascaraNumerica(bruto) && soDigitos(bruto).length === 8;
}
