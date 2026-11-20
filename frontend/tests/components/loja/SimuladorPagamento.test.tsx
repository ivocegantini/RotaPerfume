/** ECOM-18: simulador de pagamento só em dev/qa e só no pedido Aguardando pagamento. */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const ambienteMock = vi.fn();
const simularMock = vi.fn();
vi.mock("@/lib/api", () => ({
  apiAmbiente: () => ambienteMock(),
  apiSimularPagamento: (...a: unknown[]) => simularMock(...a),
}));

import { SimuladorPagamento } from "@/components/loja/SimuladorPagamento";
import { instrucaoPagamento, mensagemStatus } from "@/components/loja/Checkout";
import { limparAmbienteMemo } from "@/lib/ambiente";

beforeEach(() => {
  limparAmbienteMemo();
  ambienteMock.mockReset().mockResolvedValue("qa");
  simularMock.mockReset();
});

describe("SimuladorPagamento", () => {
  it("aprovar chama a API e devolve a compra atualizada", async () => {
    const concluido = vi.fn();
    simularMock.mockResolvedValue({ pedido_id: 9, status: "Em separação", status_pagamento: "Pago" });
    render(<SimuladorPagamento pedidoId={9} status="Aguardando pagamento" onConcluido={concluido} />);
    fireEvent.click(await screen.findByRole("button", { name: "Aprovar pagamento" }));
    await waitFor(() => expect(concluido).toHaveBeenCalledWith(expect.objectContaining({ status: "Em separação" })));
    expect(simularMock).toHaveBeenCalledWith(9, "aprovado");
  });

  it("recusar com erro da API mostra a mensagem", async () => {
    simularMock.mockRejectedValue(new Error("o pedido não está aguardando pagamento"));
    render(<SimuladorPagamento pedidoId={9} status="Aguardando pagamento" onConcluido={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Recusar pagamento" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("não está aguardando pagamento");
    expect(simularMock).toHaveBeenCalledWith(9, "recusado");
  });

  it("produção ou pedido já confirmado: sem botões", async () => {
    ambienteMock.mockResolvedValue("producao");
    const { container, rerender } = render(<SimuladorPagamento pedidoId={9} status="Aguardando pagamento" onConcluido={vi.fn()} />);
    await waitFor(() => expect(ambienteMock).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
    limparAmbienteMemo();
    ambienteMock.mockResolvedValue("dev");
    rerender(<SimuladorPagamento pedidoId={9} status="Em separação" onConcluido={vi.fn()} />);
    expect(screen.queryByTestId("simulador-pagamento")).toBeNull();
  });
});

describe("textos do checkout (ECOM-18)", () => {
  it("instrução por forma e mensagem por status", () => {
    expect(instrucaoPagamento("PIX", "2026-11-20")).toMatch(/Pix/);
    expect(instrucaoPagamento("Boleto 14 dias", "2026-12-04")).toMatch("04/12/2026");
    expect(instrucaoPagamento("Cartão de crédito", "2026-11-20")).toMatch(/cartão/);
    expect(mensagemStatus("Aguardando pagamento")).toMatch(/assim que o pagamento for confirmado/);
    expect(mensagemStatus("Em separação")).toMatch(/em separação/);
    expect(mensagemStatus("Cancelado")).toMatch(/recusado/);
  });
});
