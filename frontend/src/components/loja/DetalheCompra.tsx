"use client";

// ECOM-16: detalhe de uma compra (/minhas-compras/{id}): todos os itens e o
// pagamento, via GET /api/loja/minhas-compras/{id}. Pedido de outra pessoa
// ou inexistente: a API da 404 e a tela mostra "Compra nao encontrada".

import Link from "next/link";
import { useEffect, useState } from "react";
import { TopoLoja, useSessaoLocal } from "@/components/loja/TopoLoja";
import { StatusPedido, dataCompra, moeda } from "@/components/loja/MinhasCompras";
import { SimuladorPagamento } from "@/components/loja/SimuladorPagamento";
import { apiMinhaCompra } from "@/lib/api";
import { ApiError } from "@/lib/apiError";
import { formatarEntrega } from "@/lib/entrega";
import { urlImagemProduto } from "@/lib/imagemProduto";
import type { CompraDetalhe, ItemComprado, PagamentoComprado } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "DetalheCompra.tsx";
const real = (v: number) => moeda.format(Number(v) || 0);

type Estado =
  | { tipo: "carregando" }
  | { tipo: "ok"; compra: CompraDetalhe }
  | { tipo: "nao-encontrado" }
  | { tipo: "erro"; mensagem: string };

/** id do path como inteiro positivo, ou null (nem chama a API). */
export function idDaCompra(bruto: string | undefined): number | null {
  if (!bruto || !/^[1-9][0-9]{0,15}$/.test(bruto)) return null;
  return Number(bruto);
}

function Item({ it }: { it: ItemComprado }) {
  const img = urlImagemProduto(it.imagem_url);
  return (
    <li className="flex items-center gap-3 py-3">
      <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-md bg-slate-100">
        {img ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={img} alt="" className="h-full w-full object-cover" />
        ) : (
          <span className="text-lg font-bold text-slate-300" aria-hidden="true">
            {it.nome.charAt(0).toUpperCase()}
          </span>
        )}
      </div>
      <div className="min-w-0 flex-1">
        {it.ativo ? (
          <Link href={`/produtos/${it.produto_id}`} className="font-medium text-primary-700 hover:underline">
            {it.nome}
          </Link>
        ) : (
          <span className="font-medium text-slate-900">{it.nome}</span>
        )}
        <p className="text-xs text-slate-500">
          SKU {it.sku} · {it.quantidade} x {real(it.preco_praticado)}
          {it.desconto_pct > 0 && ` · ${it.desconto_pct}% de desconto`}
          {!it.ativo && " · indisponivel na loja"}
        </p>
      </div>
      <span className="text-sm font-semibold text-slate-900">{real(it.valor_bruto)}</span>
    </li>
  );
}

function Pagamento({ p }: { p: PagamentoComprado }) {
  return (
    <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-3">
      <div>
        <dt className="text-slate-500">Forma</dt>
        <dd className="font-medium text-slate-900">
          {p.forma_pagamento}
          {p.parcelas > 1 && ` em ${p.parcelas}x`}
        </dd>
      </div>
      <div>
        <dt className="text-slate-500">Valor</dt>
        <dd className="font-medium text-slate-900">{real(p.valor)}</dd>
      </div>
      <div>
        <dt className="text-slate-500">Situacao</dt>
        <dd className="font-medium text-slate-900">{p.status_pagamento}</dd>
      </div>
      <div>
        <dt className="text-slate-500">Vencimento</dt>
        <dd className="text-slate-900">{dataCompra(p.data_vencimento)}</dd>
      </div>
      <div>
        <dt className="text-slate-500">Pago em</dt>
        <dd className="text-slate-900">{p.data_pagamento ? dataCompra(p.data_pagamento) : "—"}</dd>
      </div>
    </dl>
  );
}

function Detalhe({ c, onRecarregar }: { c: CompraDetalhe; onRecarregar: () => void }) {
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-bold text-slate-900">Compra #{c.pedido_id}</h1>
        <StatusPedido status={c.status} />
        <span className="text-sm text-slate-500">{dataCompra(c.data)}</span>
      </div>
      <section className="rounded-xl border border-slate-200 bg-white p-4">
        <h2 className="mb-1 text-lg font-semibold text-slate-900">Produtos</h2>
        <ul className="divide-y divide-slate-100">
          {c.itens.map((it, i) => (
            <Item key={`${it.produto_id}-${i}`} it={it} />
          ))}
        </ul>
        <p className="mt-2 border-t border-slate-200 pt-3 text-right text-base font-bold text-slate-900">
          Total {real(c.valor_total)}
        </p>
      </section>
      {c.entrega && (
        <section className="rounded-xl border border-slate-200 bg-white p-4">
          <h2 className="mb-1 text-lg font-semibold text-slate-900">Endereco de entrega</h2>
          <p className="text-sm text-slate-700">{formatarEntrega(c.entrega)}</p>
        </section>
      )}
      <section className="rounded-xl border border-slate-200 bg-white p-4">
        <h2 className="mb-3 text-lg font-semibold text-slate-900">Pagamento</h2>
        {c.status === "Aguardando pagamento" && (
          <p className="mb-3 text-sm text-slate-600">O pedido vai para separação assim que o pagamento for confirmado.</p>
        )}
        {c.pagamentos.length === 0 ? (
          <p className="text-sm text-slate-500">Nenhum pagamento registrado para esta compra.</p>
        ) : (
          <div className="space-y-4 divide-y divide-slate-100">
            {c.pagamentos.map((p, i) => (
              <div key={i} className={i > 0 ? "pt-4" : undefined}>
                <Pagamento p={p} />
              </div>
            ))}
          </div>
        )}
        <SimuladorPagamento pedidoId={c.pedido_id} status={c.status} onConcluido={onRecarregar} />
      </section>
    </div>
  );
}

export function DetalheCompra({ idBruto }: { idBruto: string | undefined }) {
  const sessao = useSessaoLocal();
  const logado = sessao !== null;
  const id = idDaCompra(idBruto);
  const [carregado, setCarregado] = useState<{ id: number; estado: Estado } | null>(null);
  const [versao, setVersao] = useState(0); // ECOM-18: relê depois do simulador

  useEffect(() => {
    if (!logado || id === null) return;
    let vivo = true;
    vlog(FILE, "DetalheCompra.useEffect", "buscando compra, id:", id);
    apiMinhaCompra(id).then(
      (compra) => {
        if (vivo) setCarregado({ id, estado: { tipo: "ok", compra } });
      },
      (e: unknown) => {
        if (!vivo) return;
        vlog(FILE, "DetalheCompra.useEffect", "falha ao carregar a compra");
        const estado: Estado =
          e instanceof ApiError && e.status === 404
            ? { tipo: "nao-encontrado" }
            : { tipo: "erro", mensagem: e instanceof Error && e.message ? e.message : "Erro ao carregar a compra" };
        setCarregado({ id, estado });
      }
    );
    return () => {
      vivo = false;
    };
  }, [logado, id, versao]);

  const estado: Estado =
    id === null ? { tipo: "nao-encontrado" } : carregado && carregado.id === id ? carregado.estado : { tipo: "carregando" };

  return (
    <div className="min-h-screen bg-slate-50">
      <TopoLoja />
      <main className="mx-auto max-w-4xl px-4 py-6">
        <Link href="/minhas-compras" className="mb-5 inline-block text-sm font-medium text-primary-600 hover:underline">
          ← Voltar para minhas compras
        </Link>
        {!logado ? (
          <p className="py-16 text-center text-sm text-slate-500">
            <Link href="/login" className="font-medium text-primary-600 hover:underline">
              Entre na sua conta
            </Link>{" "}
            para ver esta compra.
          </p>
        ) : (
          <>
            {estado.tipo === "carregando" && <p className="py-16 text-center text-sm text-slate-500">Carregando compra...</p>}
            {estado.tipo === "nao-encontrado" && (
              <p className="py-16 text-center text-sm text-slate-500">Compra nao encontrada.</p>
            )}
            {estado.tipo === "erro" && (
              <p role="alert" className="py-16 text-center text-sm text-red-600">
                {estado.mensagem}
              </p>
            )}
            {estado.tipo === "ok" && <Detalhe c={estado.compra} onRecarregar={() => setVersao((v) => v + 1)} />}
          </>
        )}
      </main>
    </div>
  );
}
