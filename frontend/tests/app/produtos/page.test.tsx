/** ECOM-11: pagina do produto (/produtos/{id}) e link do cartao da vitrine. */
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/apiError";
import type { ProdutoLojaDetalhe } from "@/lib/types";

const params = vi.hoisted(() => ({ id: "7" as string | undefined }));
vi.mock("next/navigation", () => ({ useParams: () => params, useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));

const getMock = vi.fn();
const listarMock = vi.fn();
vi.mock("@/lib/api", () => ({
  apiGetProdutoLoja: (...a: unknown[]) => getMock(...a),
  apiListProdutosLoja: (...a: unknown[]) => listarMock(...a),
  apiOpcoesFiltroLoja: () => Promise.resolve({ categorias: [], marcas: [], notas_olfativas: [] }),
}));

import ProdutoPage from "@/app/produtos/[id]/page";
import LojaPage from "@/app/loja/page";
import { idDoProduto } from "@/components/loja/PaginaProduto";

const produto: ProdutoLojaDetalhe = {
  id: 7, sku: "SKU-7", nome: "Perfume Oud", categoria: "Eau de Parfum", marca: "Sahra", nota_olfativa: "Oud",
  preco: 337.35, unidade: "KIT 3", imagem_url: null,
  descricao: "Primeiro paragrafo.\n\n<b>Segundo</b> paragrafo.",
};

beforeEach(() => {
  params.id = "7";
  getMock.mockReset().mockResolvedValue(produto);
  listarMock.mockReset().mockResolvedValue({ data: [produto], page: 1, limit: 24, total: 1, pages: 1 });
});

describe("ProdutoPage", () => {
  it("mostra nome, preco, caracteristicas e a descricao completa como texto", async () => {
    render(<ProdutoPage />);
    expect(await screen.findByRole("heading", { level: 1, name: "Perfume Oud" })).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith(7);
    expect(screen.getByText(/337,35/)).toBeInTheDocument();
    expect(screen.getByText("KIT 3")).toBeInTheDocument();
    expect(screen.getByText("Oud")).toBeInTheDocument();
    const desc = screen.getByText(/Primeiro paragrafo/);
    expect(desc).toHaveClass("whitespace-pre-line");
    expect(desc.textContent).toContain("<b>Segundo</b>"); // nunca vira HTML
    expect(desc.querySelector("b")).toBeNull();
    expect(screen.getByRole("link", { name: /Voltar para a loja/ })).toHaveAttribute("href", "/");
  });

  it("sem descricao mostra o aviso", async () => {
    getMock.mockResolvedValueOnce({ ...produto, descricao: "" });
    render(<ProdutoPage />);
    expect(await screen.findByText("Este produto ainda nao tem descricao.")).toBeInTheDocument();
  });

  it("404 da API e id invalido mostram 'nao encontrado'; outro erro mostra a mensagem", async () => {
    getMock.mockRejectedValueOnce(new ApiError(404, "produto não encontrado"));
    const { unmount } = render(<ProdutoPage />);
    expect(await screen.findByText("Produto nao encontrado.")).toBeInTheDocument();
    unmount();

    params.id = "abc";
    const r2 = render(<ProdutoPage />);
    expect(screen.getByText("Produto nao encontrado.")).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledTimes(1); // id invalido nem chama a API
    r2.unmount();

    params.id = "7";
    getMock.mockRejectedValueOnce(new Error("falhou"));
    render(<ProdutoPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("falhou");
  });

  it("idDoProduto so aceita inteiro positivo", () => {
    expect(idDoProduto("12")).toBe(12);
    for (const v of [undefined, "", "0", "-1", "1.5", "01", "1e3", "abc", "12345678901234567"]) {
      expect(idDoProduto(v)).toBeNull();
    }
  });
});

describe("Vitrine - cartao (ECOM-11)", () => {
  it("o cartao do produto leva para /produtos/{id}", async () => {
    render(<LojaPage />);
    const nome = await screen.findByText("Perfume Oud");
    expect(nome.closest("a")).toHaveAttribute("href", "/produtos/7");
  });
});
