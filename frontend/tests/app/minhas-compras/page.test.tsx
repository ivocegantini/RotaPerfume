/** ECOM-12/16: pagina "Minhas compras" (/minhas-compras): um pedido por linha, clicavel. */
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Compra } from "@/lib/types";

const push = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ push, replace: vi.fn() }) }));

const comprasMock = vi.fn();
vi.mock("@/lib/api", () => ({ apiMinhasCompras: (...a: unknown[]) => comprasMock(...a) }));
vi.mock("@/lib/auth", () => ({ logout: vi.fn(), rotaInicial: () => "/dashboard" }));

import MinhasComprasPage from "@/app/minhas-compras/page";
import { dataCompra } from "@/components/loja/MinhasCompras";

const c31: Compra = { pedido_id: 31, data: "2026-11-10", status: "Faturado", valor_total: 299.7, produtos: ["Perfume Oud", "Perfume Ambar"] };
const c30: Compra = { pedido_id: 30, data: "2026-10-01", status: "Cancelado", valor_total: 50, produtos: ["Perfume Antigo"] };

function logar() {
  localStorage.setItem("auth_user", JSON.stringify({ id: 900, nome: "Ana", email: "a@x.com", role: "usuario" }));
}

beforeEach(() => {
  localStorage.clear();
  push.mockReset();
  comprasMock.mockReset().mockResolvedValue({ data: [c31, c30], page: 1, limit: 20, total: 2, pages: 1 });
});

describe("MinhasComprasPage", () => {
  it("sem sessao convida a entrar e nao chama a API", () => {
    render(<MinhasComprasPage />);
    expect(screen.getByRole("link", { name: "Entre na sua conta" })).toHaveAttribute("href", "/login");
    expect(comprasMock).not.toHaveBeenCalled();
  });

  it("lista ID, data, nomes dos produtos, status e total de cada pedido", async () => {
    logar();
    render(<MinhasComprasPage />);
    const link = await screen.findByRole("link", { name: "#31" });
    expect(link).toHaveAttribute("href", "/minhas-compras/31");
    expect(comprasMock).toHaveBeenCalledWith(1, 20, "data", "desc");
    const linha = link.closest("tr")!;
    expect(within(linha).getByText("10/11/2026")).toBeInTheDocument();
    expect(within(linha).getByText("Perfume Oud, Perfume Ambar")).toBeInTheDocument();
    expect(within(linha).getByText("Faturado")).toBeInTheDocument();
    expect(within(linha).getByText(/299,70/)).toBeInTheDocument();
    expect(screen.getByText("Cancelado")).toBeInTheDocument();
  });

  it("clicar na linha abre o detalhe da compra", async () => {
    logar();
    render(<MinhasComprasPage />);
    fireEvent.click(await screen.findByText("Perfume Antigo"));
    expect(push).toHaveBeenCalledWith("/minhas-compras/30");
    fireEvent.click(screen.getByRole("link", { name: "#31" })); // o link navega sozinho; a linha nao repete
    expect(push).toHaveBeenCalledTimes(1);
  });

  it("clicar no cabecalho ordena pelo servidor (desc e depois asc)", async () => {
    logar();
    render(<MinhasComprasPage />);
    await screen.findByText("#31");
    fireEvent.click(screen.getByText("Total"));
    await vi.waitFor(() => expect(comprasMock).toHaveBeenLastCalledWith(1, 20, "valor_total", "desc"));
    fireEvent.click(screen.getByText("Total"));
    await vi.waitFor(() => expect(comprasMock).toHaveBeenLastCalledWith(1, 20, "valor_total", "asc"));
  });

  it("sem compras mostra o aviso; erro mostra a mensagem", async () => {
    logar();
    comprasMock.mockResolvedValueOnce({ data: [], page: 1, limit: 20, total: 0, pages: 0 });
    const { unmount } = render(<MinhasComprasPage />);
    expect(await screen.findByText("Voce ainda nao fez nenhuma compra.")).toBeInTheDocument();
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
