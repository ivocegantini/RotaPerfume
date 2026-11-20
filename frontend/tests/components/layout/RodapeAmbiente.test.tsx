/** ECOM-18: selo DEV/QA no rodapé; nada em produção ou com falha da API. */
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const ambienteMock = vi.fn();
vi.mock("@/lib/api", () => ({ apiAmbiente: () => ambienteMock() }));

import { RodapeAmbiente } from "@/components/layout/RodapeAmbiente";
import { limparAmbienteMemo } from "@/lib/ambiente";

beforeEach(() => {
  limparAmbienteMemo();
  ambienteMock.mockReset();
});

describe("RodapeAmbiente", () => {
  it.each([
    ["qa", "QA", /testes \(QA\)/],
    ["dev", "DEV", /desenvolvimento/],
  ])("APP_ENV=%s mostra o selo %s", async (ambiente, rotulo, texto) => {
    ambienteMock.mockResolvedValue(ambiente);
    render(<RodapeAmbiente />);
    const rodape = await screen.findByTestId("rodape-ambiente");
    expect(rodape).toHaveTextContent(rotulo);
    expect(rodape).toHaveTextContent(texto);
  });

  it("produção não mostra nada", async () => {
    ambienteMock.mockResolvedValue("producao");
    const { container } = render(<RodapeAmbiente />);
    await waitFor(() => expect(ambienteMock).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("falha da API vale produção (fail-closed)", async () => {
    ambienteMock.mockRejectedValue(new Error("rede"));
    const { container } = render(<RodapeAmbiente />);
    await waitFor(() => expect(ambienteMock).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });
});
