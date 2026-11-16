// ECOM-09: busca do endereco pelo CEP na ViaCEP (servico publico, sem chave).
// So o CEP sai do navegador. A resposta e conferida campo a campo; qualquer
// falha (rede, tempo, CEP inexistente) devolve null e o cliente digita.

import { UFS_BRASIL } from "./clienteLoja";
import { vlog } from "./vlog";

const F = "viaCep.ts";
const TIMEOUT_MS = 5000;

export interface EnderecoCEP {
  logradouro: string;
  bairro: string;
  cidade: string;
  uf: string;
}

const texto = (v: unknown, max: number) => (typeof v === "string" ? v.trim().slice(0, max) : "");

export async function buscarCEP(bruto: string, fetcher: typeof fetch = fetch): Promise<EnderecoCEP | null> {
  const cep = bruto.replace(/\D/g, "");
  if (cep.length !== 8) return null;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), TIMEOUT_MS);
  try {
    vlog(F, "buscarCEP", "consultando ViaCEP");
    const r = await fetcher(`https://viacep.com.br/ws/${cep}/json/`, { signal: ctrl.signal, credentials: "omit" });
    if (!r.ok) return null;
    const d: unknown = await r.json();
    if (!d || typeof d !== "object" || (d as { erro?: unknown }).erro) return null;
    const o = d as Record<string, unknown>;
    const uf = texto(o.uf, 2).toUpperCase();
    const cidade = texto(o.localidade, 120);
    if (!cidade || !UFS_BRASIL.some((u) => u.sigla === uf)) return null;
    return { logradouro: texto(o.logradouro, 255), bairro: texto(o.bairro, 120), cidade, uf };
  } catch {
    vlog(F, "buscarCEP", "falha ou tempo esgotado na ViaCEP");
    return null;
  } finally {
    clearTimeout(timer);
  }
}
