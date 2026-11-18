/** ECOM-12: pagina "Minhas compras" (/minhas-compras). */
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ProdutoComprado } from "@/lib/types";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));

const comprasMock = vi.fn();
vi.mock("@/lib/api", () => ({ apiMinhasCompras: (...a: unknown[]) => comprasMock(...a) }));
vi.mock("@/lib/auth", () => ({ logout: vi.fn(), rotaInicial: () => "/dashboard" }));

import MinhasComprasPage from "@/app/minhas-compras/page";
import { dataCompra } from "@/components/loja/MinhasCompras";

const ativo: ProdutoComprado = {
  produto_id: 7, sku: "SKU-7", nome: "Perfume Oud", imagem_url: null, ativo: true,
  quantidade: 3, pedidos: 2, total_gasto: 299.7, ultima_compra: "2026-11-10",
};
const inativo: ProdutoComprado = { ...ativo, produto_id: 8, sku: "SKU-8", nome: "Perfume Antigo", ativo: false };

function logar() {
  localStorage.setItem("auth_user", JSON.stringify({ id: 900, nome: "Ana", email: "a@x.com", role: "usuario" }));
}

beforeEach(() => {
  localStorage.clear();
  comprasMock.mockReset().mockResolvedValue({ data: [ativo, inativo], page: 1, limit: 20, total: 2, pages: 1 });
});

describe("MinhasComprasPage", () => {
  it("sem sessao convida a entrar e nao chama a API", () => {
    render(<MinhasComprasPage />);
    expect(screen.getByRole("link", { name: "Entre na sua conta" })).toHaveAttribute("href", "/login");
    expect(comprasMock).not.toHaveBeenCalled();
  });

  it("lista os produtos comprados; so o ativo tem link para a pagina do produto", async () => {
    logar();
    render(<MinhasComprasPage />);
    const link = await screen.findByRole("link", { name: "7 - Perfume Oud" });
    expect(link).toHaveAttribute("href", "/produtos/7");
    expect(comprasMock).toHaveBeenCalledWith(1, 20, "ultima_compra", "desc");
    expect(screen.getByText("8 - Perfume Antigo").closest("a")).toBeNull();
    expect(screen.getByText(/indisponivel na loja/)).toBeInTheDocument();
    const linha = link.closest("tr")!;
    expect(within(linha).getByText(/299,70/)).toBeInTheDocument();
    expect(within(linha).getByText("10/11/2026")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Minhas compras" })).toHaveAttribute("href", "/minhas-compras");
  });

  it("clicar no cabecalho ordena pelo servidor (desc e depois asc)", async () => {
    logar();
    render(<MinhasComprasPage />);
    await screen.findByText("7 - Perfume Oud");
    fireEvent.click(screen.getByText("Total gasto"));
    await vi.waitFor(() => expect(comprasMock).toHaveBeenLastCalledWith(1, 20, "total_gasto", "desc"));
    fireEvent.click(screen.getByText("Total gasto"));
    await vi.waitFor(() => expect(comprasMock).toHaveBeenLastCalledWith(1, 20, "total_gasto", "asc"));
  });

  it("sem compras mostra o aviso; erro mostra a mensagem", async () => {
    logar();
    comprasMock.mockResolvedValueOnce({ data: [], page: 1, limit: 20, total: 0, pages: 0 });
    const { unmount } = render(<MinhasComprasPage />);
    expect(await screen.findByText("Voce ainda nao comprou nenhum produto.")).toBeInTheDocument();
    unmount();
    comprasMock.mockRejectedValueOnce(new Error("falha de rede"));
    render(<MinhasComprasPage />);
    expect(await screen.findByText("falha de rede")).toBeInTheDocument();
  });

  it("dataCompra formata AAAA-MM-DD", () => {
    expect(dataCompra("2026-01-05")).toBe("05/01/2026");
    expect(dataCompra("x")).toBe("x");
  });
});
