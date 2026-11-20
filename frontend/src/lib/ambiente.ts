"use client";

// ECOM-18: ambiente da API (APP_ENV) para o selo do rodapé e o simulador de
// pagamento. Uma única chamada por carga da página; falha = "producao"
// (fail-closed: sem selo e sem botões de simulação).

import { useEffect, useState } from "react";
import { apiAmbiente } from "./api";
import type { Ambiente } from "./types";
import { vlog } from "./vlog";

let pendente: Promise<Ambiente> | null = null;

/** Busca o ambiente uma vez (memoizado); erro vira "producao". */
export function carregarAmbiente(): Promise<Ambiente> {
  if (!pendente) {
    pendente = Promise.resolve()
      .then(() => apiAmbiente())
      .catch(() => {
        vlog("ambiente.ts", "carregarAmbiente", "falha ao ler o ambiente; assumindo producao");
        return "producao" as Ambiente;
      });
  }
  return pendente;
}

/** Só para testes: esquece o ambiente memoizado. */
export function limparAmbienteMemo(): void {
  pendente = null;
}

/** dev e qa permitem o simulador de pagamento e mostram o selo. */
export const ehAmbienteDeTeste = (a: Ambiente | null) => a === "dev" || a === "qa";

/** Ambiente da API; null enquanto carrega. */
export function useAmbiente(): Ambiente | null {
  const [ambiente, setAmbiente] = useState<Ambiente | null>(null);
  useEffect(() => {
    let vivo = true;
    carregarAmbiente().then((a) => {
      if (vivo) setAmbiente(a);
    });
    return () => {
      vivo = false;
    };
  }, []);
  return ambiente;
}
