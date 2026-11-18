"use client";

// ECOM-11: pagina publica do produto, aberta pelo cartao da vitrine. Le
// GET /api/loja/produtos/{id} (so ativos, sem custo) e mostra a descricao
// completa como texto (sem HTML; as quebras de paragrafo sao mantidas).
// ECOM-13: botao Comprar e o carrinho na barra lateral (LayoutLoja).

import Link from "next/link";
import { useEffect, useState } from "react";
import { BotaoComprar, LayoutLoja } from "@/components/loja/CarrinhoLateral";
import { apiGetProdutoLoja } from "@/lib/api";
import { ApiError } from "@/lib/apiError";
import { urlImagemProduto } from "@/lib/imagemProduto";
import type { ProdutoLojaDetalhe } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "PaginaProduto.tsx";
const moeda = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

type Estado =
  | { tipo: "carregando" }
  | { tipo: "ok"; produto: ProdutoLojaDetalhe }
  | { tipo: "nao-encontrado" }
  | { tipo: "erro"; mensagem: string };

/** id do path como inteiro positivo, ou null (nem chama a API). */
export function idDoProduto(bruto: string | undefined): number | null {
  if (!bruto || !/^[1-9][0-9]{0,15}$/.test(bruto)) return null;
  return Number(bruto);
}

export function PaginaProduto({ idBruto }: { idBruto: string | undefined }) {
  const id = idDoProduto(idBruto);
  const [carregado, setCarregado] = useState<{ id: number; estado: Estado } | null>(null);

  useEffect(() => {
    if (id === null) return;
    let vivo = true;
    vlog(FILE, "PaginaProduto.useEffect", "buscando produto da loja, id:", id);
    apiGetProdutoLoja(id).then(
      (produto) => {
        if (vivo) setCarregado({ id, estado: { tipo: "ok", produto } });
      },
      (e: unknown) => {
        if (!vivo) return;
        vlog(FILE, "PaginaProduto.useEffect", "falha ao carregar o produto");
        const estado: Estado =
          e instanceof ApiError && e.status === 404
            ? { tipo: "nao-encontrado" }
            : { tipo: "erro", mensagem: e instanceof Error && e.message ? e.message : "Erro ao carregar o produto" };
        setCarregado({ id, estado });
      }
    );
    return () => {
      vivo = false;
    };
  }, [id]);

  // Estado de outro id (navegacao entre produtos) conta como carregando.
  const estado: Estado =
    id === null ? { tipo: "nao-encontrado" } : carregado && carregado.id === id ? carregado.estado : { tipo: "carregando" };

  return (
    <LayoutLoja>
        <Link href="/" className="mb-5 inline-block text-sm font-medium text-primary-600 hover:underline">
          ← Voltar para a loja
        </Link>
        {estado.tipo === "carregando" && <p className="py-16 text-center text-sm text-slate-500">Carregando produto...</p>}
        {estado.tipo === "nao-encontrado" && (
          <p className="py-16 text-center text-sm text-slate-500">Produto nao encontrado.</p>
        )}
        {estado.tipo === "erro" && (
          <p role="alert" className="py-16 text-center text-sm text-red-600">
            {estado.mensagem}
          </p>
        )}
        {estado.tipo === "ok" && <DetalheProduto p={estado.produto} />}
    </LayoutLoja>
  );
}

function DetalheProduto({ p }: { p: ProdutoLojaDetalhe }) {
  const img = urlImagemProduto(p.imagem_url);
  const caracteristicas: [string, string][] = [
    ["Marca", p.marca],
    ["Categoria", p.categoria],
    ["Nota olfativa", p.nota_olfativa],
    ["Embalagem", p.unidade],
    ["SKU", p.sku],
  ];
  return (
    <article className="grid gap-8 md:grid-cols-2">
      <div className="flex aspect-square items-center justify-center overflow-hidden rounded-xl border border-slate-200 bg-slate-100">
        {img ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={img} alt={p.nome} className="h-full w-full object-cover" />
        ) : (
          <span className="text-7xl font-bold text-slate-300" aria-hidden="true">
            {p.nome.charAt(0).toUpperCase()}
          </span>
        )}
      </div>
      <div className="flex flex-col gap-4">
        <div>
          <p className="text-xs uppercase tracking-wide text-slate-400">{p.marca}</p>
          <h1 className="mt-1 text-2xl font-bold text-slate-900">{p.nome}</h1>
        </div>
        <p className="text-3xl font-bold text-primary-700">{moeda.format(Number(p.preco) || 0)}</p>
        <div className="sm:max-w-xs">
          <BotaoComprar produto={p} size="lg" fullWidth />
        </div>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          {caracteristicas
            .filter(([, v]) => v)
            .map(([rotulo, valor]) => (
              <div key={rotulo} className="contents">
                <dt className="text-slate-500">{rotulo}</dt>
                <dd className="text-slate-900">{valor}</dd>
              </div>
            ))}
        </dl>
        <section aria-labelledby="titulo-descricao" className="border-t border-slate-200 pt-4">
          <h2 id="titulo-descricao" className="mb-2 text-base font-semibold text-slate-900">
            Descricao
          </h2>
          {p.descricao ? (
            <p className="whitespace-pre-line text-sm leading-relaxed text-slate-700">{p.descricao}</p>
          ) : (
            <p className="text-sm text-slate-500">Este produto ainda nao tem descricao.</p>
          )}
        </section>
      </div>
    </article>
  );
}
