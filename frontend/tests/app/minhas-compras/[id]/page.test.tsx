/** ECOM-16: detalhe da compra (/minhas-compras/{id}): itens e pagamento. */
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/apiError";
import type { CompraDetalhe } from "@/lib/types";

const params = vi.hoisted(() => ({ id: "31" as string | undefined }));
vi.mock("next/navigation", () => ({ useParams: () => params, useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));

const getMock = vi.fn();
vi.mock("@/lib/api", () => ({ apiMinhaCompra: (...a: unknown[]) => getMock(...a) }));
vi.mock("@/lib/auth", () => ({ logout: vi.fn(), rotaInicial: () => "/dashboard" }));

import MinhaCompraPage from "@/app/minhas-compras/[id]/page";
import { idDaCompra } from "@/components/loja/DetalheCompra";

const compra: CompraDetalhe = {
  pedido_id: 31, data: "2026-11-10", status: "Faturado", valor_total: 349.7,
  entrega: { cep: "01310100", endereco: "Avenida Paulista", numero: "1000", complemento: "ap 5", bairro: "Bela Vista", cidade: "São Paulo", uf: "SP" },
  itens: [
    { produto_id: 7, sku: "SKU-7", nome: "Perfume Oud", imagem_url: null, ativo: true, quantidade: 3, preco_praticado: 99.9, desconto_pct: 0, valor_bruto: 299.7 },
    { produto_id: 8, sku: "SKU-8", nome: "Perfume Antigo", imagem_url: null, ativo: false, quantidade: 1, preco_praticado: 50, desconto_pct: 0, valor_bruto: 50 },
  ],
  pagamentos: [{ forma_pagamento: "Cartão de crédito", parcelas: 3, valor: 349.7, data_vencimento: "2026-12-10", data_pagamento: null, status_pagamento: "Em aberto" }],
};

function logar() {
  localStorage.setItem("auth_user", JSON.stringify({ id: 900, nome: "Ana", email: "a@x.com", role: "usuario" }));
}

beforeEach(() => {
  localStorage.clear();
  params.id = "31";
  getMock.mockReset().mockResolvedValue(compra);
});

describe("MinhaCompraPage", () => {
  it("mostra todos os produtos e o pagamento", async () => {
    logar();
    render(<MinhaCompraPage />);
    expect(await screen.findByRole("heading", { name: "Compra #31" })).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith(31);
    expect(screen.getByRole("link", { name: "Perfume Oud" })).toHaveAttribute("href", "/produtos/7");
    expect(screen.getByText("Perfume Antigo").closest("a")).toBeNull();
    expect(screen.getByText(/indisponivel na loja/)).toBeInTheDocument();
    expect(screen.getByText("Cartão de crédito em 3x")).toBeInTheDocument();
    expect(screen.getByText("Em aberto")).toBeInTheDocument();
    expect(screen.getByText("10/12/2026")).toBeInTheDocument();
    expect(screen.getByText("Avenida Paulista, 1000 - ap 5, Bela Vista, São Paulo/SP, CEP 01310-100")).toBeInTheDocument(); // ECOM-17
    expect(screen.getByRole("link", { name: /Voltar para minhas compras/ })).toHaveAttribute("href", "/minhas-compras");
  });

  it("sem pagamento avisa", async () => {
    logar();
    getMock.mockResolvedValueOnce({ ...compra, pagamentos: [] });
    render(<MinhaCompraPage />);
    expect(await screen.findByText("Nenhum pagamento registrado para esta compra.")).toBeInTheDocument();
  });

  it("404 (pedido alheio) mostra compra nao encontrada; id invalido nem chama a API", async () => {
    logar();
    getMock.mockRejectedValueOnce(new ApiError(404, "pedido não encontrado"));
    const { unmount } = render(<MinhaCompraPage />);
    expect(await screen.findByText("Compra nao encontrada.")).toBeInTheDocument();
    unmount();
    getMock.mockClear();
    params.id = "abc";
    render(<MinhaCompraPage />);
    expect(screen.getByText("Compra nao encontrada.")).toBeInTheDocument();
    expect(getMock).not.toHaveBeenCalled();
  });

  it("sem sessao convida a entrar e nao chama a API", () => {
    render(<MinhaCompraPage />);
    expect(screen.getByRole("link", { name: "Entre na sua conta" })).toHaveAttribute("href", "/login");
    expect(getMock).not.toHaveBeenCalled();
  });

  it("idDaCompra aceita so inteiro positivo", () => {
    expect(idDaCompra("31")).toBe(31);
    expect(idDaCompra("0")).toBeNull();
    expect(idDaCompra("-1")).toBeNull();
    expect(idDaCompra(undefined)).toBeNull();
  });
});
