"use client";

// ECOM-18: rodapé com o selo do ambiente (DEV ou QA). Em produção (ou se o
// GET /api/ambiente falhar) não aparece nada.

import { ehAmbienteDeTeste, useAmbiente } from "@/lib/ambiente";

const ROTULO = { dev: "DEV", qa: "QA" } as const;

export function RodapeAmbiente() {
  const ambiente = useAmbiente();
  if (!ehAmbienteDeTeste(ambiente)) return null;
  const rotulo = ROTULO[ambiente as "dev" | "qa"];
  const cor = ambiente === "dev" ? "bg-purple-700" : "bg-amber-600";
  return (
    <>
      {/* espaço reservado: o rodapé fixo não cobre o fim da página */}
      <div aria-hidden="true" className="h-7" />
      <footer
        data-testid="rodape-ambiente"
        className={`fixed inset-x-0 bottom-0 z-50 flex h-7 items-center justify-center gap-2 px-4 text-xs font-semibold text-white ${cor}`}
      >
        <span className="rounded bg-white/20 px-1.5 py-0.5 tracking-wider">{rotulo}</span>
        <span className="truncate">Ambiente de {rotulo === "DEV" ? "desenvolvimento" : "testes (QA)"} — pagamentos simulados</span>
      </footer>
    </>
  );
}
