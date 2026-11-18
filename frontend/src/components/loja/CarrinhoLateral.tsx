"use client";

// ECOM-13: carrinho da loja. CarrinhoLateral e a lista (com + / - e remover);
// LayoutLoja poe o topo e a barra lateral direita (fixa no desktop, gaveta no
// celular); BotaoComprar soma 1 unidade e abre a gaveta.

import Link from "next/link";
import { createContext, ReactNode, useCallback, useContext, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { TopoLoja } from "@/components/loja/TopoLoja";
import {
  adicionarAoCarrinho,
  alterarQuantidade,
  ItemCarrinho,
  QTD_MAXIMA,
  removerDoCarrinho,
  totalCarrinho,
  useCarrinho,
} from "@/lib/carrinho";
import { urlImagemProduto } from "@/lib/imagemProduto";
import type { ProdutoLoja } from "@/lib/types";

const moeda = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

const AbrirCarrinho = createContext<() => void>(() => {});

function LinhaItem({ item }: { item: ItemCarrinho }) {
  const img = urlImagemProduto(item.imagem_url);
  return (
    <li className="flex gap-3 py-3">
      <div className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-md bg-slate-100">
        {img ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={img} alt="" className="h-full w-full object-cover" />
        ) : (
          <span className="text-lg font-bold text-slate-300" aria-hidden="true">
            {item.nome.charAt(0).toUpperCase()}
          </span>
        )}
      </div>
      <div className="min-w-0 flex-1">
        <Link href={`/produtos/${item.id}`} className="line-clamp-2 text-sm font-medium text-slate-900 hover:underline">
          {item.nome}
        </Link>
        <p className="text-xs text-slate-500">{moeda.format(item.preco)} cada</p>
        <div className="mt-2 flex items-center justify-between gap-2">
          <div className="flex items-center rounded-md border border-slate-300" role="group" aria-label={`Quantidade de ${item.nome}`}>
            <button
              type="button"
              className="px-2 py-0.5 text-slate-700 hover:bg-slate-100"
              aria-label={`Diminuir ${item.nome}`}
              onClick={() => alterarQuantidade(item.id, item.quantidade - 1)}
            >
              −
            </button>
            <span className="min-w-8 text-center text-sm" aria-live="polite">
              {item.quantidade}
            </span>
            <button
              type="button"
              className="px-2 py-0.5 text-slate-700 hover:bg-slate-100 disabled:text-slate-300"
              aria-label={`Adicionar mais ${item.nome}`}
              disabled={item.quantidade >= QTD_MAXIMA}
              onClick={() => alterarQuantidade(item.id, item.quantidade + 1)}
            >
              +
            </button>
          </div>
          <span className="text-sm font-semibold text-slate-900">{moeda.format(item.preco * item.quantidade)}</span>
        </div>
        <button
          type="button"
          className="mt-1 text-xs text-red-600 hover:underline"
          aria-label={`Remover ${item.nome}`}
          onClick={() => removerDoCarrinho(item.id)}
        >
          Remover
        </button>
      </div>
    </li>
  );
}

/** Lista do carrinho com total e o botao para a tela de pagamento. */
export function CarrinhoLateral({ onFechar, semBotaoPagamento = false }: { onFechar?: () => void; semBotaoPagamento?: boolean }) {
  const itens = useCarrinho();
  const { unidades, valor } = totalCarrinho(itens);
  return (
    <section aria-labelledby="titulo-carrinho" className="flex h-full flex-col rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex items-center justify-between">
        <h2 id="titulo-carrinho" className="text-base font-semibold text-slate-900">
          Carrinho {unidades > 0 && <span className="text-sm font-normal text-slate-500">({unidades})</span>}
        </h2>
        {onFechar && (
          <button type="button" className="text-slate-500 hover:text-slate-800" aria-label="Fechar carrinho" onClick={onFechar}>
            ✕
          </button>
        )}
      </div>
      {itens.length === 0 ? (
        <p className="py-8 text-center text-sm text-slate-500">Seu carrinho esta vazio.</p>
      ) : (
        <ul className="min-h-0 flex-1 divide-y divide-slate-100 overflow-y-auto">
          {itens.map((i) => (
            <LinhaItem key={i.id} item={i} />
          ))}
        </ul>
      )}
      <div className="mt-3 border-t border-slate-200 pt-3">
        <p className="mb-3 flex justify-between text-sm">
          <span className="text-slate-600">Total</span>
          <span className="text-lg font-bold text-primary-700">{moeda.format(valor)}</span>
        </p>
        {semBotaoPagamento ? null : itens.length > 0 ? (
          <Link
            href="/checkout"
            className="block rounded-lg bg-primary-600 px-4 py-2 text-center text-sm font-medium text-white hover:bg-primary-700"
          >
            Ir para o pagamento
          </Link>
        ) : (
          <Button fullWidth disabled>
            Ir para o pagamento
          </Button>
        )}
      </div>
    </section>
  );
}

/** Topo + conteudo + barra lateral direita com o carrinho. */
export function LayoutLoja({ children, marcaComoTitulo = false }: { children: ReactNode; marcaComoTitulo?: boolean }) {
  const [aberto, setAberto] = useState(false);
  const abrir = useCallback(() => setAberto(true), []);
  const fechar = useCallback(() => setAberto(false), []);

  useEffect(() => {
    if (!aberto) return;
    const onTecla = (e: KeyboardEvent) => e.key === "Escape" && setAberto(false);
    window.addEventListener("keydown", onTecla);
    return () => window.removeEventListener("keydown", onTecla);
  }, [aberto]);

  return (
    <AbrirCarrinho.Provider value={abrir}>
      <div className="min-h-screen bg-slate-50">
        <TopoLoja marcaComoTitulo={marcaComoTitulo} onAbrirCarrinho={abrir} />
        <div className="mx-auto flex max-w-7xl gap-6 px-4 py-6">
          <main className="min-w-0 flex-1">{children}</main>
          <aside className="hidden w-80 shrink-0 lg:block" aria-label="Carrinho de compras">
            <div className="sticky top-4 max-h-[calc(100vh-2rem)]">
              <CarrinhoLateral />
            </div>
          </aside>
        </div>
        {aberto && (
          <div className="fixed inset-0 z-40 lg:hidden" role="dialog" aria-modal="true" aria-label="Carrinho de compras">
            <div className="absolute inset-0 bg-slate-900/40" onClick={fechar} aria-hidden="true" />
            <div className="absolute inset-y-0 right-0 w-full max-w-sm p-3">
              <CarrinhoLateral onFechar={fechar} />
            </div>
          </div>
        )}
      </div>
    </AbrirCarrinho.Provider>
  );
}

/** Botao "Comprar": soma 1 unidade ao carrinho e abre a gaveta (celular). */
export function BotaoComprar({
  produto,
  size = "md",
  fullWidth = false,
}: {
  produto: Pick<ProdutoLoja, "id" | "nome" | "marca" | "preco" | "imagem_url">;
  size?: "sm" | "md" | "lg";
  fullWidth?: boolean;
}) {
  const abrir = useContext(AbrirCarrinho);
  const [aviso, setAviso] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const comprar = () => {
    const ok = adicionarAoCarrinho(produto);
    setAviso(ok ? "Adicionado!" : "Carrinho cheio");
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setAviso(null), 1500);
    if (ok) abrir();
  };

  return (
    <Button size={size} fullWidth={fullWidth} onClick={comprar} aria-label={`Comprar ${produto.nome}`}>
      {aviso ?? "Comprar"}
    </Button>
  );
}
