// ECOM-17: endereco de entrega obrigatorio (UX; o backend decide).
// CEP, rua, numero, bairro, cidade e estado (UF) obrigatorios; complemento opcional.

import { UFS_BRASIL, cepValido, mascararCEP } from "./clienteLoja";
import type { EnderecoEntrega } from "./types";

export type ErrosEntrega = Partial<Record<keyof EnderecoEntrega, string>>;

export const entregaVazia = (): EnderecoEntrega => ({
  cep: "", endereco: "", numero: "", complemento: "", bairro: "", cidade: "", uf: "",
});

/** Erros por campo; objeto vazio = endereco completo. */
export function validarEntrega(e: EnderecoEntrega): ErrosEntrega {
  const erros: ErrosEntrega = {};
  if (!cepValido(e.cep)) erros.cep = "CEP deve ter 8 digitos";
  if (!e.endereco.trim()) erros.endereco = "Rua e obrigatoria";
  if (!e.numero.trim()) erros.numero = "Numero e obrigatorio (use s/n se nao houver)";
  if (!e.bairro.trim()) erros.bairro = "Bairro e obrigatorio";
  if (!e.cidade.trim()) erros.cidade = "Cidade e obrigatoria";
  const uf = e.uf.trim().toUpperCase();
  if (!UFS_BRASIL.some((u) => u.sigla === uf)) erros.uf = "Estado invalido (use a sigla, ex.: SP)";
  return erros;
}

/** Payload enviado a API (aparado, UF maiuscula). */
export function normalizarEntrega(e: EnderecoEntrega): EnderecoEntrega {
  return {
    cep: e.cep.replace(/\D/g, ""),
    endereco: e.endereco.trim(),
    numero: e.numero.trim(),
    complemento: e.complemento.trim(),
    bairro: e.bairro.trim(),
    cidade: e.cidade.trim(),
    uf: e.uf.trim().toUpperCase(),
  };
}

/** Uma linha: "Rua X, 10 - apto 2, Bairro, Cidade/UF, CEP 00000-000". */
export function formatarEntrega(e: EnderecoEntrega): string {
  const comp = e.complemento ? ` - ${e.complemento}` : "";
  return `${e.endereco}, ${e.numero}${comp}, ${e.bairro}, ${e.cidade}/${e.uf}, CEP ${mascararCEP(e.cep)}`;
}
