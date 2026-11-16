"use client";

// ECOM-01/10: vitrine da loja, a pagina inicial do site (publica). Lista os
// produtos ativos de GET /api/loja/produtos (sem custo), com busca, filtros,
// ordenacao e paginacao no servidor. O topo mostra Entrar/Criar conta ao
// visitante e Ola/Sair (e Painel para a equipe) a quem esta logado.

import Link from "next/link";
import { FormEvent, useEffect, useState, useSyncExternalStore } from "react";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Paginador } from "@/components/ui/Paginador";
import { apiListProdutosLoja, apiOpcoesFiltroLoja } from "@/lib/api";
import { logout, rotaInicial } from "@/lib/auth";
import { urlImagemProduto } from "@/lib/imagemProduto";
import { useUltimaResposta } from "@/lib/useListaSegura";
import type { OpcoesFiltroLoja, ProdutoLoja, User } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "Vitrine.tsx";

// Sessao do cache local (so para o topo; a API decide o acesso). Store
// externa: no servidor e na hydration e "sem sessao", entao o HTML bate.
const CHAVE_USUARIO = "auth_user";
const assinarSessao = (cb: () => void) => {
  window.addEventListener("storage", cb);
  return () => window.removeEventListener("storage", cb);
};
const lerSessao = () => {
  try {
    return localStorage.getItem(CHAVE_USUARIO);
  } catch {
    return null;
  }
};
const sessaoNoServidor = () => null;

function useSessaoLocal(): Pick<User, "nome" | "role"> | null {
  const bruto = useSyncExternalStore(assinarSessao, lerSessao, sessaoNoServidor);
  if (!bruto) return null;
  try {
    const u = JSON.parse(bruto) as User;
    return u && typeof u.nome === "string" ? { nome: u.nome, role: u.role } : null;
  } catch {
    return null;
  }
}

const POR_PAGINA = 24;

type Ordem = "descricao:asc" | "preco_tabela:asc" | "preco_tabela:desc" | "marca:asc";
const ORDENS: { value: Ordem; label: string }[] = [
  { value: "descricao:asc", label: "Nome (A-Z)" },
  { value: "preco_tabela:asc", label: "Menor preco" },
  { value: "preco_tabela:desc", label: "Maior preco" },
  { value: "marca:asc", label: "Marca (A-Z)" },
];

// Pedido da carga: pagina, busca, ordem e filtros exatos ("" = todos).
interface Pedido {
  pg: number;
  q: string;
  o: Ordem;
  categoria: string;
  marca: string;
  nota: string;
}
const PEDIDO_INICIAL: Pedido = { pg: 1, q: "", o: "descricao:asc", categoria: "", marca: "", nota: "" };
const SEM_OPCOES: OpcoesFiltroLoja = { categorias: [], marcas: [], notas_olfativas: [] };

const moeda = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

function CartaoProduto({ p }: { p: ProdutoLoja }) {
  const img = urlImagemProduto(p.imagem_url);
  return (
    <li className="flex flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm transition hover:shadow-md">
      <div className="flex aspect-square items-center justify-center bg-slate-100">
        {img ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={img} alt={p.descricao} loading="lazy" className="h-full w-full object-cover" />
        ) : (
          <span className="text-4xl font-bold text-slate-300" aria-hidden="true">
            {p.descricao.charAt(0).toUpperCase()}
          </span>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <p className="text-xs uppercase tracking-wide text-slate-400">{p.marca}</p>
        <h3 className="line-clamp-2 text-sm font-semibold text-slate-900">{p.descricao}</h3>
        {p.nota_olfativa && <p className="text-xs text-slate-500">{p.nota_olfativa}</p>}
        <p className="mt-auto pt-2 text-lg font-bold text-primary-700">{moeda.format(Number(p.preco) || 0)}</p>
      </div>
    </li>
  );
}

export function Vitrine() {
  const [produtos, setProdutos] = useState<ProdutoLoja[]>([]);
  const [pagina, setPagina] = useState(1);
  const [paginas, setPaginas] = useState(1);
  const [total, setTotal] = useState(0);
  const [busca, setBusca] = useState("");
  const [loading, setLoading] = useState(true);
  const [erro, setErro] = useState<string | null>(null);
  const sessao = useSessaoLocal();

  const executar = useUltimaResposta();
  // Pedido da carga atual; trocar dispara a busca.
  const [pedido, setPedido] = useState<Pedido>(PEDIDO_INICIAL);
  const [opcoes, setOpcoes] = useState<OpcoesFiltroLoja>(SEM_OPCOES);

  // ECOM-04: opcoes dos filtros, uma vez. Falha so deixa os selects vazios.
  useEffect(() => {
    let vivo = true;
    apiOpcoesFiltroLoja().then(
      (o) => {
        if (vivo) setOpcoes(o);
      },
      () => vlog(FILE, "Vitrine.useEffect", "falha ao carregar as opcoes dos filtros")
    );
    return () => {
      vivo = false;
    };
  }, []);

  useEffect(() => {
    vlog(FILE, "Vitrine.useEffect", "buscando vitrine, pagina/ordem:", pedido.pg, pedido.o);
    const [campo, dir] = pedido.o.split(":") as ["descricao" | "marca" | "preco_tabela", "asc" | "desc"];
    void executar(
      apiListProdutosLoja(pedido.pg, POR_PAGINA, pedido.q, campo, dir, {
        categoria: pedido.categoria,
        marca: pedido.marca,
        nota_olfativa: pedido.nota,
      }),
      (res) => {
        setProdutos(res.data);
        setPagina(res.page);
        setPaginas(res.pages);
        setTotal(res.total);
        setErro(null);
        setLoading(false);
      },
      (e) => {
        vlog(FILE, "Vitrine.useEffect", "falha ao carregar a vitrine");
        setErro(e instanceof Error && e.message ? e.message : "Erro ao carregar os produtos");
        setLoading(false);
      }
    );
  }, [executar, pedido]);

  const pedir = (novo: Partial<Pedido>) => {
    setLoading(true);
    setPedido((atual) => ({ ...atual, ...novo }));
  };

  const onBuscar = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    pedir({ pg: 1, q: busca.trim() });
  };

  const temFiltro = !!(pedido.categoria || pedido.marca || pedido.nota || pedido.q);
  const limparFiltros = () => {
    setBusca("");
    pedir({ pg: 1, q: "", categoria: "", marca: "", nota: "" });
  };

  const filtros: { rotulo: string; todos: string; campo: "categoria" | "marca" | "nota"; valores: string[] }[] = [
    { rotulo: "Categoria", todos: "Todas as categorias", campo: "categoria", valores: opcoes.categorias },
    { rotulo: "Marca", todos: "Todas as marcas", campo: "marca", valores: opcoes.marcas },
    { rotulo: "Nota olfativa", todos: "Todas as notas", campo: "nota", valores: opcoes.notas_olfativas },
  ];

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-3">
          <h1 className="text-xl font-bold tracking-tight text-slate-900">
            Rota<span className="text-primary-600">Perfumes</span>
          </h1>
          <nav className="flex items-center gap-2 sm:gap-3" aria-label="Conta">
            {sessao ? (
              <>
                <span className="hidden text-sm text-slate-600 sm:inline">Ola, {sessao.nome}</span>
                {sessao.role !== "usuario" && (
                  <Link href={rotaInicial(sessao.role)} className="text-sm font-medium text-primary-600 hover:underline">
                    Painel
                  </Link>
                )}
                <Button size="sm" variant="secondary" onClick={() => void logout()}>
                  Sair
                </Button>
              </>
            ) : (
              <>
                <Link href="/login" className="text-sm font-medium text-slate-700 hover:underline">
                  Entrar
                </Link>
                <Link
                  href="/cadastro"
                  className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
                >
                  Criar conta
                </Link>
              </>
            )}
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-6">
        <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <form onSubmit={onBuscar} className="flex w-full gap-2 sm:max-w-md" role="search">
            <div className="flex-1">
              <Input
                aria-label="Buscar produtos"
                placeholder="Buscar por nome ou SKU"
                value={busca}
                onChange={(e) => setBusca(e.target.value)}
              />
            </div>
            <Button type="submit" variant="primary">
              Buscar
            </Button>
          </form>
          <label className="flex items-center gap-2 text-sm text-slate-600">
            Ordenar
            <select
              aria-label="Ordenar"
              className="rounded-lg border border-slate-300 bg-white px-2 py-2 text-sm"
              value={pedido.o}
              onChange={(e) => pedir({ pg: 1, o: e.target.value as Ordem })}
            >
              {ORDENS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="mb-5 grid grid-cols-1 gap-3 sm:grid-cols-3 lg:grid-cols-4 lg:items-end">
          {filtros.map((f) => (
            <label key={f.campo} className="flex flex-col gap-1 text-sm text-slate-600">
              {f.rotulo}
              <select
                aria-label={f.rotulo}
                className="rounded-lg border border-slate-300 bg-white px-2 py-2 text-sm text-slate-900"
                value={pedido[f.campo]}
                onChange={(e) => pedir({ pg: 1, [f.campo]: e.target.value })}
              >
                <option value="">{f.todos}</option>
                {f.valores.map((v) => (
                  <option key={v} value={v}>
                    {v}
                  </option>
                ))}
              </select>
            </label>
          ))}
          {temFiltro && (
            <Button variant="secondary" onClick={limparFiltros}>
              Limpar filtros
            </Button>
          )}
        </div>

        {erro && (
          <div className="mb-4">
            <Alert variant="error" onClose={() => setErro(null)}>
              {erro}
            </Alert>
          </div>
        )}

        {loading ? (
          <p className="py-16 text-center text-sm text-slate-500">Carregando produtos...</p>
        ) : produtos.length === 0 ? (
          <p className="py-16 text-center text-sm text-slate-500">Nenhum produto encontrado.</p>
        ) : (
          <>
            <p className="mb-3 text-sm text-slate-500">{total} produto(s)</p>
            <ul className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
              {produtos.map((p) => (
                <CartaoProduto key={p.id} p={p} />
              ))}
            </ul>
          </>
        )}

        <div className="mt-6">
          <Paginador pagina={pagina} paginas={paginas} onIrPara={(n) => pedir({ pg: n })} />
        </div>
      </main>
    </div>
  );
}
