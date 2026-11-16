/** ECOM-09: o CEP preenche rua, bairro, cidade e UF; tudo segue editavel. */
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn(), push: vi.fn() }) }));
vi.mock("@/lib/api", () => ({ apiCadastro: vi.fn(), apiGoogle: vi.fn(), apiGoogleCadastro: vi.fn() }));
vi.mock("@/components/ui/Turnstile", async () => {
  const React = await import("react");
  const Turnstile = React.forwardRef<{ reset: () => void }, object>(function Fake(_p, ref) {
    React.useImperativeHandle(ref, () => ({ reset: () => {} }));
    return null;
  });
  return { Turnstile, TURNSTILE_ACTION_CADASTRO: "cadastro", isTurnstileEnabled: false };
});
const buscarMock = vi.hoisted(() => vi.fn());
vi.mock("@/lib/viaCep", () => ({ buscarCEP: (c: string) => buscarMock(c) }));

import CadastroPage from "@/app/cadastro/page";

beforeEach(() => buscarMock.mockReset());

describe("Cadastro - CEP (ECOM-09)", () => {
  it("preenche pelo CEP e os campos continuam editaveis", async () => {
    buscarMock.mockResolvedValue({ logradouro: "Praça da Sé", bairro: "Sé", cidade: "São Paulo", uf: "SP" });
    render(<CadastroPage />);
    await userEvent.type(screen.getByLabelText("CEP"), "01001000");
    await waitFor(() => expect(screen.getByLabelText("Rua")).toHaveValue("Praça da Sé"));
    expect(buscarMock).toHaveBeenCalledWith("01001000");
    expect(screen.getByLabelText("CEP")).toHaveValue("01001-000");
    expect(screen.getByLabelText("Bairro")).toHaveValue("Sé");
    expect(screen.getByLabelText("Cidade")).toHaveValue("São Paulo");
    expect(screen.getByLabelText("UF")).toHaveValue("SP");
    expect(screen.getByText(/Confira e informe o numero/)).toBeInTheDocument();
    await userEvent.clear(screen.getByLabelText("Rua"));
    await userEvent.type(screen.getByLabelText("Rua"), "Rua Nova");
    expect(screen.getByLabelText("Rua")).toHaveValue("Rua Nova");
  });

  it("CEP nao encontrado: avisa e deixa digitar", async () => {
    buscarMock.mockResolvedValue(null);
    render(<CadastroPage />);
    await userEvent.type(screen.getByLabelText("CEP"), "99999999");
    expect(await screen.findByText(/CEP nao encontrado/)).toBeInTheDocument();
    expect(screen.getByLabelText("Rua")).toHaveValue("");
  });

  it("CEP de cidade (sem rua): preenche cidade/UF e pede rua e bairro", async () => {
    buscarMock.mockResolvedValue({ logradouro: "", bairro: "", cidade: "Itaú de Minas", uf: "MG" });
    render(<CadastroPage />);
    await userEvent.type(screen.getByLabelText("CEP"), "37975000");
    expect(await screen.findByText(/informe a rua e o bairro/)).toBeInTheDocument();
    expect(screen.getByLabelText("Cidade")).toHaveValue("Itaú de Minas");
    expect(screen.getByLabelText("UF")).toHaveValue("MG");
  });

  it("so consulta com 8 digitos", async () => {
    render(<CadastroPage />);
    await userEvent.type(screen.getByLabelText("CEP"), "0100");
    expect(buscarMock).not.toHaveBeenCalled();
  });
});
