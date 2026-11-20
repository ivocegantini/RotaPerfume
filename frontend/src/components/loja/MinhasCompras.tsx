"use client";

// ECOM-12/16: "Minhas compras" — um pedido por linha (ID, data, produtos,
// status, total) do usuario logado (GET /api/loja/minhas-compras), ordenavel
// e paginado no servidor. A linha abre /minhas-compras/{id} (itens e
// pagamento). Visitante sem sessao ve o convite para entrar (nem chama a API).

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/Alert";
import { Paginador } from "@/components/ui/Paginador";
import { Badge, Column, Table } from "@/components/ui/Table";
import { TopoLoja, useSessaoLocal } from "@/components/loja/TopoLoja";
import { apiMinhasCompras, type Paginado } from "@/lib/api";
import { useUltimaResposta } from "@/lib/useListaSegura";
import type { Compra, OrdemCompra } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "MinhasCompras.tsx";
const POR_PAGINA = 20;
export const moeda = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

/** "AAAA-MM-DD" → "DD/MM/AAAA" (sem fuso: a data ja e do pedido). */
export function dataCompra(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  return m ? `${m[3]}/${m[2]}/${m[1]}` : iso;
}

/** Cor do status do pedido. */
export function StatusPedido({ status }: { status: string }) {
  const cor =
    status === "Cancelado"
      ? "red"
      : status === "Entregue"
        ? "green"
        : status === "Faturado"
          ? "blue"
          : status === "Aguardando pagamento" // ECOM-18
            ? "purple"
            : "yellow";
  return <Badge color={cor}>{status}</Badge>;
}

const urlCompra = (c: Compra) => `/minhas-compras/${c.pedido_id}`;

const colunas: Column<Compra>[] = [
  {
    key: "pedido_id",
    header: "Pedido",
    sortable: true,
    render: (c) => (
      <Link href={urlCompra(c)} className="font-medium text-primary-700 hover:underline">
        #{c.pedido_id}
      </Link>
    ),
  },
  { key: "data", header: "Data", sortable: true, render: (c) => dataCompra(c.data) },
  {
    key: "produtos",
    header: "Produtos",
    render: (c) => (
      <span className="block max-w-md truncate" title={c.produtos.join(", ")}>
        {c.produtos.join(", ") || "—"}
      </span>
    ),
  },
  { key: "status", header: "Status", render: (c) => <StatusPedido status={c.status} /> },
  { key: "valor_total", header: "Total", sortable: true, align: "right", render: (c) => moeda.format(Number(c.valor_total) || 0) },
];

export function MinhasCompras() {
  const router = useRouter();
  const sessao = useSessaoLocal();
  const executar = useUltimaResposta();
  const [pagina, setPagina] = useState(1);
  const [ordem, setOrdem] = useState<{ campo: OrdemCompra; dir: "asc" | "desc" }>({ campo: "data", dir: "desc" });
  const [res, setRes] = useState<Paginado<Compra[]> | null>(null);
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

  function ordenar(chave: keyof Compra | string) {
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
            para ver as suas compras.
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
              keyExtractor={(c) => c.pedido_id}
              loading={!res && !erro} // so a 1a carga; depois mantem as linhas
              emptyMessage="Voce ainda nao fez nenhuma compra."
              sortKey={ordem.campo}
              sortDir={ordem.dir}
              onSort={ordenar}
              erroCarga={erro !== null}
              onRowClick={(c) => router.push(urlCompra(c))}
            />
            <Paginador pagina={res?.page ?? 1} paginas={res?.pages ?? 1} onIrPara={setPagina} />
          </div>
        )}
      </main>
    </div>
  );
}
