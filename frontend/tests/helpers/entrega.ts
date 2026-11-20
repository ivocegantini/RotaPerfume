// ECOM-17: endereco de entrega dos testes. Os arquivos que usam
// preencherEntrega mockam a ViaCEP: vi.mock("@/lib/viaCep", () => viaCepMock).
import { fireEvent, waitFor, within } from "@testing-library/react";
import { expect } from "vitest";
import type { EnderecoEntrega } from "@/lib/types";

export const ENTREGA: EnderecoEntrega = {
  cep: "01310100", endereco: "Avenida Paulista", numero: "1000", complemento: "",
  bairro: "Bela Vista", cidade: "São Paulo", uf: "SP",
};

export const viaCepMock = {
  buscarCEP: async () => ({ logradouro: "Avenida Paulista", bairro: "Bela Vista", cidade: "São Paulo", uf: "SP" }),
};

/** Digita o CEP, espera a pesquisa liberar os campos e informa o numero. */
export async function preencherEntrega(raiz: HTMLElement = document.body) {
  const w = within(raiz);
  fireEvent.change(w.getByLabelText("CEP"), { target: { value: "01310100" } });
  await waitFor(() => expect(w.getByLabelText("Rua")).toBeEnabled());
  fireEvent.change(w.getByLabelText("Numero"), { target: { value: "1000" } });
}
