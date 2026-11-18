"use client";

// ECOM-10/11/12: topo da loja. Mostra Entrar/Criar conta ao visitante e
// Ola/Minhas compras/Sair (e Painel para a equipe) a quem esta logado.
// ECOM-13: botao do carrinho (abre a gaveta no celular); Sair limpa o carrinho.

import Link from "next/link";
import { useSyncExternalStore } from "react";
import { Button } from "@/components/ui/Button";
import { logout, rotaInicial } from "@/lib/auth";
import { limparCarrinho, totalCarrinho, useCarrinho } from "@/lib/carrinho";
import type { User } from "@/lib/types";

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

/** Sessao do cache local (so para a UI; a API decide o acesso). */
export function useSessaoLocal(): Pick<User, "nome" | "role"> | null {
  const bruto = useSyncExternalStore(assinarSessao, lerSessao, sessaoNoServidor);
  if (!bruto) return null;
  try {
    const u = JSON.parse(bruto) as User;
    return u && typeof u.nome === "string" ? { nome: u.nome, role: u.role } : null;
  } catch {
    return null;
  }
}

/** marcaComoTitulo: na vitrine a marca e o h1; na pagina do produto o h1 e o nome. */
export function TopoLoja({
  marcaComoTitulo = false,
  onAbrirCarrinho,
}: {
  marcaComoTitulo?: boolean;
  onAbrirCarrinho?: () => void;
}) {
  const sessao = useSessaoLocal();
  const { unidades } = totalCarrinho(useCarrinho());
  const Marca = marcaComoTitulo ? "h1" : "p";
  return (
    <header className="border-b border-slate-200 bg-white">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-3 px-4 py-3">
        <Marca className="text-xl font-bold tracking-tight text-slate-900">
          <Link href="/" className="hover:opacity-80">
            Rota<span className="text-primary-600">Perfumes</span>
          </Link>
        </Marca>
        <nav className="flex items-center gap-2 sm:gap-3" aria-label="Conta">
          {onAbrirCarrinho && (
            <button
              type="button"
              className="rounded-md border border-slate-300 px-2.5 py-1 text-sm font-medium text-slate-700 hover:bg-slate-50 lg:hidden"
              aria-label={`Abrir carrinho (${unidades} ${unidades === 1 ? "item" : "itens"})`}
              onClick={onAbrirCarrinho}
            >
              🛒 {unidades}
            </button>
          )}
          {sessao ? (
            <>
              <span className="hidden text-sm text-slate-600 sm:inline">Ola, {sessao.nome}</span>
              <Link href="/minhas-compras" className="text-sm font-medium text-slate-700 hover:underline">
                Minhas compras
              </Link>
              {sessao.role !== "usuario" && (
                <Link href={rotaInicial(sessao.role)} className="text-sm font-medium text-primary-600 hover:underline">
                  Painel
                </Link>
              )}
              <Button size="sm" variant="secondary" onClick={() => {
                  limparCarrinho();
                  void logout();
                }}>
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
  );
}
