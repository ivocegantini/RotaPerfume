// ECOM-13: carrinho da loja no localStorage (por navegador, sem API).
// O conteudo e dado nao confiavel: cada item e validado na leitura. Preco e
// nome sao so para exibir; o pedido (ECOM-14) deve recalcular tudo no servidor.

import { useMemo, useSyncExternalStore } from "react";
import type { ProdutoLoja } from "@/lib/types";

export const CHAVE_CARRINHO = "carrinho_loja";
export const EVENTO_CARRINHO = "carrinho-alterado";
export const QTD_MAXIMA = 99;
export const ITENS_MAXIMOS = 50;

export interface ItemCarrinho {
  id: number;
  nome: string;
  marca: string;
  preco: number;
  imagem_url: string | null;
  quantidade: number;
}

const inteiroPositivo = (v: unknown): v is number => Number.isSafeInteger(v) && (v as number) > 0;

/** Item valido ou null (descarta lixo e limita a quantidade). */
function itemValido(bruto: unknown): ItemCarrinho | null {
  if (!bruto || typeof bruto !== "object") return null;
  const o = bruto as Record<string, unknown>;
  if (!inteiroPositivo(o.id) || !inteiroPositivo(o.quantidade)) return null;
  if (typeof o.nome !== "string" || typeof o.marca !== "string") return null;
  if (typeof o.preco !== "number" || !Number.isFinite(o.preco) || o.preco < 0) return null;
  if (o.imagem_url !== null && typeof o.imagem_url !== "string") return null;
  return {
    id: o.id,
    nome: o.nome.slice(0, 200),
    marca: o.marca.slice(0, 100),
    preco: o.preco,
    imagem_url: o.imagem_url,
    quantidade: Math.min(o.quantidade, QTD_MAXIMA),
  };
}

/** Le o carrinho do texto bruto do storage; qualquer erro vira carrinho vazio. */
export function lerItens(bruto: string | null): ItemCarrinho[] {
  if (!bruto) return [];
  try {
    const lista: unknown = JSON.parse(bruto);
    if (!Array.isArray(lista)) return [];
    const vistos = new Set<number>();
    const itens: ItemCarrinho[] = [];
    for (const b of lista) {
      const item = itemValido(b);
      if (item && !vistos.has(item.id) && itens.length < ITENS_MAXIMOS) {
        vistos.add(item.id);
        itens.push(item);
      }
    }
    return itens;
  } catch {
    return [];
  }
}

function lerBruto(): string | null {
  try {
    return localStorage.getItem(CHAVE_CARRINHO);
  } catch {
    return null;
  }
}

function gravar(itens: ItemCarrinho[]): void {
  try {
    if (itens.length === 0) localStorage.removeItem(CHAVE_CARRINHO);
    else localStorage.setItem(CHAVE_CARRINHO, JSON.stringify(itens));
  } catch {
    // storage bloqueado: o carrinho so nao persiste
  }
  window.dispatchEvent(new Event(EVENTO_CARRINHO));
}

export function obterItens(): ItemCarrinho[] {
  return lerItens(lerBruto());
}

/** Soma 1 unidade (ou cria o item). Devolve false se o carrinho esta cheio. */
export function adicionarAoCarrinho(p: Pick<ProdutoLoja, "id" | "nome" | "marca" | "preco" | "imagem_url">): boolean {
  const itens = obterItens();
  const atual = itens.find((i) => i.id === p.id);
  if (atual) {
    atual.quantidade = Math.min(atual.quantidade + 1, QTD_MAXIMA);
  } else {
    if (itens.length >= ITENS_MAXIMOS) return false;
    const novo = itemValido({ ...p, preco: Number(p.preco), quantidade: 1 });
    if (!novo) return false;
    itens.push(novo);
  }
  gravar(itens);
  return true;
}

/** Define a quantidade (1..99); 0 ou menos remove o item. */
export function alterarQuantidade(id: number, quantidade: number): void {
  const itens = obterItens();
  const q = Math.min(Math.floor(quantidade), QTD_MAXIMA);
  gravar(q > 0 ? itens.map((i) => (i.id === id ? { ...i, quantidade: q } : i)) : itens.filter((i) => i.id !== id));
}

export function removerDoCarrinho(id: number): void {
  gravar(obterItens().filter((i) => i.id !== id));
}

export function limparCarrinho(): void {
  gravar([]);
}

export function totalCarrinho(itens: ItemCarrinho[]): { unidades: number; valor: number } {
  return itens.reduce(
    (t, i) => ({ unidades: t.unidades + i.quantidade, valor: Math.round((t.valor + i.preco * i.quantidade) * 100) / 100 }),
    { unidades: 0, valor: 0 }
  );
}

const assinar = (cb: () => void) => {
  window.addEventListener(EVENTO_CARRINHO, cb);
  window.addEventListener("storage", cb); // outras abas
  return () => {
    window.removeEventListener(EVENTO_CARRINHO, cb);
    window.removeEventListener("storage", cb);
  };
};

/** Itens do carrinho, atualizados a cada mudanca (no servidor/hydration: vazio). */
export function useCarrinho(): ItemCarrinho[] {
  const bruto = useSyncExternalStore(assinar, lerBruto, () => null);
  return useMemo(() => lerItens(bruto), [bruto]);
}
