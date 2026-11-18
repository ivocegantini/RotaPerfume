"use client";

// ECOM-12: "Minhas compras" — produtos dos pedidos nao cancelados do usuario
// logado (GET /api/loja/minhas-compras), em tabela ordenavel e paginada no
// servidor. Visitante sem sessao ve o convite para entrar (nem chama a API).

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/Alert";
import { Paginador } from "@/components/ui/Paginador";
import { Column, Table } from "@/components/ui/Table";
import { TopoLoja, useSessaoLocal } from "@/components/loja/TopoLoja";
import { apiMinhasCompras, type Paginado } from "@/lib/api";
import { urlImagemProduto } from "@/lib/imagemProduto";
import { useUltimaResposta } from "@/lib/useListaSegura";
import type { OrdemCompra, ProdutoComprado } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "MinhasCompras.tsx";
const POR_PAGINA = 20;
const moeda = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

/** "AAAA-MM-DD" → "DD/MM/AAAA" (sem fuso: a data ja e do pedido). */
export function dataCompra(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  return m ? `${m[3]}/${m[2]}/${m[1]}` : iso;
}

function Produto({ c }: { c: ProdutoComprado }) {
  const img = urlImagemProduto(c.imagem_url);
  const rotulo = `${c.produto_id} - ${c.nome}`;
  return (
    <div className="flex items-center gap-3">
      <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-md bg-slate-100">
        {img ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={img} alt="" className="h-full w-full object-cover" />
        ) : (
          <span className="text-lg font-bold text-slate-300" aria-hidden="true">
            {c.nome.charAt(0).toUpperCase()}
          </span>
        )}
      </div>
      <div>
        {c.ativo ? (
          <Link href={`/produtos/${c.produto_id}`} className="font-medium text-primary-700 hover:underline">
            {rotulo}
          </Link>
        ) : (
          <span className="font-medium text-slate-900">{rotulo}</span>
        )}
        <p className="text-xs text-slate-500">
          SKU {c.sku}
          {!c.ativo && " · indisponivel na loja"}
        </p>
      </div>
    </div>
  );
}

const colunas: Column<ProdutoComprado>[] = [
  { key: "nome", header: "Produto", sortable: true, render: (c) => <Produto c={c} /> },
  { key: "quantidade", header: "Unidades", sortable: true, align: "right" },
  { key: "pedidos", header: "Pedidos", sortable: true, align: "right" },
  { key: "total_gasto", header: "Total gasto", sortable: true, align: "right", render: (c) => moeda.format(Number(c.total_gasto) || 0) },
  { key: "ultima_compra", header: "Ultima compra", sortable: true, align: "right", render: (c) => dataCompra(c.ultima_compra) },
];

export function MinhasCompras() {
  const sessao = useSessaoLocal();
  const executar = useUltimaResposta();
  const [pagina, setPagina] = useState(1);
  const [ordem, setOrdem] = useState<{ campo: OrdemCompra; dir: "asc" | "desc" }>({ campo: "ultima_compra", dir: "desc" });
  const [res, setRes] = useState<Paginado<ProdutoComprado[]> | null>(null);
  const [erro, setErro] = useState<string | null>(null);
  const logado = sessao !== null;

  useEffect(() => {
    if (!logado) return;
    vlog(FILE, "MinhasCompras.useEffect", "carregando compras, pagina/ordem:", pagina, ordem.campo, ordem.dir);
    void executar(
      apiMinhasCompras(pagina, POR_PAGINA, ordem.campo, ordem.dir),
      (r) => {
        setRes(r);
        setErro(null);
      },
      (e) => {
        setErro(e instanceof Error && e.message ? e.message : "Erro ao carregar suas compras");
      }
    );
  }, [logado, pagina, ordem, executar]);

  function ordenar(chave: keyof ProdutoComprado | string) {
    const campo = chave as OrdemCompra;
    vlog(FILE, "MinhasCompras.ordenar", "trocando ordenacao:", campo);
    setOrdem((o) => ({ campo, dir: o.campo === campo && o.dir === "desc" ? "asc" : "desc" }));
    setPagina(1);
  }

  return (
    <div className="min-h-screen bg-slate-50">
      <TopoLoja />
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Link href="/" className="mb-5 inline-block text-sm font-medium text-primary-600 hover:underline">
          ← Voltar para a loja
        </Link>
        <h1 className="mb-4 text-2xl font-bold text-slate-900">Minhas compras</h1>
        {!logado ? (
          <p className="py-16 text-center text-sm text-slate-500">
            <Link href="/login" className="font-medium text-primary-600 hover:underline">
              Entre na sua conta
            </Link>{" "}
            para ver os produtos que voce comprou.
          </p>
        ) : (
          <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
            {erro && (
              <div className="p-4">
                <Alert variant="error">{erro}</Alert>
              </div>
            )}
            <Table
              columns={colunas}
              data={res?.data ?? []}
              keyExtractor={(c) => c.produto_id}
              loading={!res && !erro} // so a 1a carga; depois mantem as linhas
              emptyMessage="Voce ainda nao comprou nenhum produto."
              sortKey={ordem.campo}
              sortDir={ordem.dir}
              onSort={ordenar}
              erroCarga={erro !== null}
            />
            <Paginador pagina={res?.page ?? 1} paginas={res?.pages ?? 1} onIrPara={setPagina} />
          </div>
        )}
      </main>
    </div>
  );
}
