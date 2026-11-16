/** ECOM-01: vitrine da loja (lista, busca, ordenacao e erro). */
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ProdutoLoja } from "@/lib/types";

const listarMock = vi.fn();
const opcoesMock = vi.fn();
vi.mock("@/lib/api", () => ({
  apiListProdutosLoja: (...a: unknown[]) => listarMock(...a),
  apiOpcoesFiltroLoja: () => opcoesMock(),
}));

import LojaPage from "@/app/loja/page";

const produto = (id: number, descricao: string): ProdutoLoja => ({
  id, sku: `SKU-${id}`, descricao, categoria: "Perfumaria", marca: "Marca X", nota_olfativa: "Citrico",
  preco: 99.9, unidade: "UN", imagem_url: null,
});

const SEM_FILTRO = { categoria: "", marca: "", nota_olfativa: "" };

beforeEach(() => {
  opcoesMock.mockReset().mockResolvedValue({ categorias: ["Bakhoor", "Incenso"], marcas: ["Marca X"], notas_olfativas: ["Oud", "Rosa"] });
  listarMock.mockReset().mockResolvedValue({ data: [produto(1, "Perfume Azul")], page: 1, limit: 24, total: 1, pages: 1 });
});

describe("LojaPage", () => {
  it("vitrine publica (ECOM-10): lista os produtos com preco, sem exigir login", async () => {
    render(<LojaPage />);
    expect(await screen.findByText("Perfume Azul")).toBeInTheDocument();
    expect(screen.getByText(/99,90/)).toBeInTheDocument();
    expect(listarMock).toHaveBeenCalledWith(1, 24, "", "descricao", "asc", SEM_FILTRO);
  });

  it("busca e ordenacao refazem a consulta", async () => {
    render(<LojaPage />);
    await screen.findByText("Perfume Azul");
    await userEvent.type(screen.getByLabelText("Buscar produtos"), "azul");
    await userEvent.click(screen.getByRole("button", { name: "Buscar" }));
    await waitFor(() => expect(listarMock).toHaveBeenLastCalledWith(1, 24, "azul", "descricao", "asc", SEM_FILTRO));
    await userEvent.selectOptions(screen.getByLabelText("Ordenar"), "preco_tabela:desc");
    await waitFor(() => expect(listarMock).toHaveBeenLastCalledWith(1, 24, "azul", "preco_tabela", "desc", SEM_FILTRO));
  });

  it("lista vazia e erro", async () => {
    listarMock.mockResolvedValueOnce({ data: [], page: 1, limit: 24, total: 0, pages: 0 });
    const { unmount } = render(<LojaPage />);
    expect(await screen.findByText("Nenhum produto encontrado.")).toBeInTheDocument();
    unmount();
    listarMock.mockRejectedValueOnce(new Error("falhou"));
    render(<LojaPage />);
    expect(await screen.findByText("falhou")).toBeInTheDocument();
  });
});

describe("LojaPage - filtros (ECOM-04)", () => {
  it("categoria, marca e nota olfativa vao para a consulta e 'Limpar filtros' zera", async () => {
    render(<LojaPage />);
    await screen.findByText("Perfume Azul");
    await screen.findByRole("option", { name: "Oud" });
    await userEvent.selectOptions(screen.getByLabelText("Categoria"), "Bakhoor");
    await userEvent.selectOptions(screen.getByLabelText("Marca"), "Marca X");
    await userEvent.selectOptions(screen.getByLabelText("Nota olfativa"), "Oud");
    await waitFor(() =>
      expect(listarMock).toHaveBeenLastCalledWith(1, 24, "", "descricao", "asc", {
        categoria: "Bakhoor",
        marca: "Marca X",
        nota_olfativa: "Oud",
      })
    );
    await userEvent.click(screen.getByRole("button", { name: "Limpar filtros" }));
    await waitFor(() => expect(listarMock).toHaveBeenLastCalledWith(1, 24, "", "descricao", "asc", SEM_FILTRO));
    expect(screen.queryByRole("button", { name: "Limpar filtros" })).not.toBeInTheDocument();
  });

  it("falha nas opcoes nao derruba a vitrine", async () => {
    opcoesMock.mockRejectedValueOnce(new Error("x"));
    render(<LojaPage />);
    expect(await screen.findByText("Perfume Azul")).toBeInTheDocument();
    expect(screen.getByLabelText("Categoria")).toHaveTextContent("Todas as categorias");
  });
});
