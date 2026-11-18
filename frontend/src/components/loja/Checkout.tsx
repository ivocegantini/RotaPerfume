"use client";

// ECOM-13/14: tela de pagamento. Mostra o carrinho (ainda editavel) e a forma
// de pagamento; visitante e convidado a entrar. "Finalizar pedido" chama
// POST /api/loja/checkout (preco recalculado no servidor; a chave evita pedido
// em dobro) e, no sucesso, limpa o carrinho e mostra o numero do pedido.

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { CarrinhoLateral } from "@/components/loja/CarrinhoLateral";
import { TopoLoja, useSessaoLocal } from "@/components/loja/TopoLoja";
import { apiFinalizarCompra } from "@/lib/api";
import { limparCarrinho, useCarrinho } from "@/lib/carrinho";
import type { CompraFinalizada, FormaCheckout } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "Checkout.tsx";
const moeda = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

/** UUID v4 da tentativa de compra (randomUUID exige HTTPS; o fallback nao). */
export function novaChave(): string {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

const dataBR = (iso: string) => iso.split("-").reverse().join("/");

function Confirmacao({ c }: { c: CompraFinalizada }) {
  return (
    <section aria-labelledby="titulo-confirmacao" className="mx-auto max-w-lg rounded-xl border border-green-200 bg-white p-6 text-center shadow-sm">
      <h2 id="titulo-confirmacao" className="text-xl font-bold text-green-700">
        Pedido #{c.pedido_id} recebido!
      </h2>
      <p className="mt-2 text-sm text-slate-600">
        Total de <strong>{moeda.format(c.valor_total)}</strong> em {c.forma_pagamento}, vencimento em {dataBR(c.data_vencimento)}.
      </p>
      <p className="mt-1 text-sm text-slate-600">Pagamento: {c.status_pagamento}. Status do pedido: {c.status}.</p>
      <div className="mt-5 flex justify-center gap-4 text-sm font-medium">
        <Link href="/minhas-compras" className="text-primary-600 hover:underline">
          Minhas compras
        </Link>
        <Link href="/" className="text-primary-600 hover:underline">
          Continuar comprando
        </Link>
      </div>
    </section>
  );
}

const FORMAS = [
  { valor: "pix", rotulo: "Pix" },
  { valor: "cartao", rotulo: "Cartao de credito" },
  { valor: "boleto", rotulo: "Boleto" },
] as const;

export function Checkout() {
  const itens = useCarrinho();
  const sessao = useSessaoLocal();
  const [forma, setForma] = useState<FormaCheckout>("pix");
  const [chave] = useState(novaChave);
  const [enviando, setEnviando] = useState(false);
  const [erro, setErro] = useState<string | null>(null);
  const [compra, setCompra] = useState<CompraFinalizada | null>(null);

  const finalizar = async () => {
    setEnviando(true);
    setErro(null);
    try {
      const c = await apiFinalizarCompra({
        chave,
        forma_pagamento: forma,
        itens: itens.map((i) => ({ produto_id: i.id, quantidade: i.quantidade })),
      });
      vlog(FILE, "Checkout.finalizar", "pedido criado, repetida:", c.repetida);
      limparCarrinho();
      setCompra(c);
    } catch (e) {
      vlog(FILE, "Checkout.finalizar", "falha ao finalizar a compra");
      setErro(e instanceof Error && e.message ? e.message : "Erro ao finalizar a compra");
    } finally {
      setEnviando(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-50">
      <TopoLoja />
      <main className="mx-auto max-w-5xl px-4 py-6">
        <Link href="/" className="mb-5 inline-block text-sm font-medium text-primary-600 hover:underline">
          ← Continuar comprando
        </Link>
        <h1 className="mb-5 text-2xl font-bold text-slate-900">{compra ? "Pedido confirmado" : "Pagamento"}</h1>
        {compra ? (
          <Confirmacao c={compra} />
        ) : itens.length === 0 ? (
          <p className="py-16 text-center text-sm text-slate-500">
            Seu carrinho esta vazio.{" "}
            <Link href="/" className="font-medium text-primary-600 hover:underline">
              Ver produtos
            </Link>
          </p>
        ) : (
          <div className="grid gap-6 md:grid-cols-[1fr_20rem]">
            <CarrinhoLateral semBotaoPagamento />
            <section aria-labelledby="titulo-forma" className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
              <h2 id="titulo-forma" className="mb-3 text-base font-semibold text-slate-900">
                Forma de pagamento
              </h2>
              {sessao ? (
                <>
                  <fieldset className="flex flex-col gap-2">
                    <legend className="sr-only">Forma de pagamento</legend>
                    {FORMAS.map((f) => (
                      <label key={f.valor} className="flex items-center gap-2 text-sm text-slate-700">
                        <input
                          type="radio"
                          name="forma"
                          value={f.valor}
                          checked={forma === f.valor}
                          onChange={() => setForma(f.valor)}
                        />
                        {f.rotulo}
                      </label>
                    ))}
                  </fieldset>
                  <div className="mt-4">
                    {erro && (
                      <p role="alert" className="mb-2 text-sm text-red-600">
                        {erro}
                      </p>
                    )}
                    <Button fullWidth loading={enviando} disabled={enviando} onClick={() => void finalizar()}>
                      Finalizar pedido
                    </Button>
                    <p className="mt-2 text-xs text-slate-500">
                      Os precos sao confirmados na finalizacao. O pagamento fica em aberto ate a confirmacao da loja.
                    </p>
                  </div>
                </>
              ) : (
                <p className="text-sm text-slate-600">
                  <Link href="/login" className="font-medium text-primary-600 hover:underline">
                    Entre na sua conta
                  </Link>{" "}
                  para finalizar a compra.
                </p>
              )}
            </section>
          </div>
        )}
      </main>
    </div>
  );
}
