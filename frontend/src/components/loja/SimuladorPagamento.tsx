"use client";

// ECOM-18: botões "Aprovar" / "Recusar pagamento" de um pedido Aguardando
// pagamento. Só aparecem em dev/qa (APP_ENV); a API recusa (404) fora deles.

import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { apiSimularPagamento } from "@/lib/api";
import { ehAmbienteDeTeste, useAmbiente } from "@/lib/ambiente";
import type { CompraFinalizada, ResultadoPagamento } from "@/lib/types";
import { vlog } from "@/lib/vlog";

const FILE = "SimuladorPagamento.tsx";

export function SimuladorPagamento({
  pedidoId,
  status,
  onConcluido,
}: {
  pedidoId: number;
  status: string;
  onConcluido: (c: CompraFinalizada) => void;
}) {
  const ambiente = useAmbiente();
  const [enviando, setEnviando] = useState<ResultadoPagamento | null>(null);
  const [erro, setErro] = useState<string | null>(null);
  if (!ehAmbienteDeTeste(ambiente) || status !== "Aguardando pagamento") return null;

  const simular = async (r: ResultadoPagamento) => {
    vlog(FILE, "simular", "simulando pagamento, pedido/resultado:", pedidoId, r);
    setEnviando(r);
    setErro(null);
    try {
      onConcluido(await apiSimularPagamento(pedidoId, r));
    } catch (e) {
      setErro(e instanceof Error && e.message ? e.message : "Falha ao simular o pagamento");
    } finally {
      setEnviando(null);
    }
  };

  return (
    <div data-testid="simulador-pagamento" className="mt-4 rounded-lg border border-dashed border-amber-400 bg-amber-50 p-3 text-left">
      <p className="text-xs font-semibold uppercase tracking-wide text-amber-800">
        Simulador de pagamento ({ambiente === "dev" ? "DEV" : "QA"})
      </p>
      <div className="mt-2 flex flex-wrap gap-2">
        <Button type="button" onClick={() => simular("aprovado")} disabled={enviando !== null}>
          {enviando === "aprovado" ? "Aprovando..." : "Aprovar pagamento"}
        </Button>
        <Button type="button" variant="danger" onClick={() => simular("recusado")} disabled={enviando !== null}>
          {enviando === "recusado" ? "Recusando..." : "Recusar pagamento"}
        </Button>
      </div>
      {erro && (
        <p role="alert" className="mt-2 text-sm text-red-600">
          {erro}
        </p>
      )}
    </div>
  );
}
