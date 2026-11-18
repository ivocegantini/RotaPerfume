/**
 * Layout raiz (metadata + <html>/<body>) e pagina inicial, que e a vitrine
 * publica de produtos (ECOM-10).
 */
import { render, screen } from "@testing-library/react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock }) }));

const api = vi.hoisted(() => ({ apiListProdutosLoja: vi.fn(), apiOpcoesFiltroLoja: vi.fn() }));
vi.mock("@/lib/api", () => api);

import RootLayout, { metadata, viewport } from "@/app/layout";
import Home from "@/app/page";

beforeEach(() => {
  replaceMock.mockReset();
});

describe("RootLayout", () => {
  it("define titulo e descricao da aplicacao", () => {
    expect(metadata.title).toBe("RotaPerfumes - Sistema de Gestao");
    expect(metadata.description).toBe("Sistema de gestao de clientes, estoque e pedidos");
  });

  it("define viewport para celular (UX-01) sem bloquear o zoom", () => {
    expect(viewport).toEqual({ width: "device-width", initialScale: 1 });
  });

  it("renderiza <html lang=pt-BR> com o conteudo dentro do <body>", () => {
    const html = renderToStaticMarkup(
      <RootLayout>
        <p id="filho">conteudo</p>
      </RootLayout>
    );
    expect(html).toMatch(/^<html lang="pt-BR">/);
    expect(html).toContain('<body class="antialiased"><p id="filho">conteudo</p></body>');
  });
});

describe("Home (/) - vitrine publica (ECOM-10)", () => {
  beforeEach(() => {
    localStorage.clear();
    api.apiListProdutosLoja.mockResolvedValue({
      data: [{ id: 1, sku: "S1", nome: "Perfume Azul", categoria: "C", marca: "M", nota_olfativa: "", preco: 10, unidade: "UN", imagem_url: null }],
      page: 1, limit: 24, total: 1, pages: 1,
    });
    api.apiOpcoesFiltroLoja.mockResolvedValue({ categorias: [], marcas: [], notas_olfativas: [] });
  });

  it("sem sessao: mostra os produtos e Entrar/Criar conta, sem redirecionar", async () => {
    render(<Home />);
    expect(await screen.findByText("Perfume Azul")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Entrar" })).toHaveAttribute("href", "/login");
    expect(screen.getByRole("link", { name: "Criar conta" })).toHaveAttribute("href", "/cadastro");
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it.each<[string, string, boolean]>([
    ["cliente da loja", "usuario", false],
    ["equipe", "normal", true],
  ])("%s logado: Ola e Sair; Painel so para a equipe", async (_n, role, painel) => {
    localStorage.setItem("auth_user", JSON.stringify({ id: 1, nome: "Ana", email: "a@x", role, ativo: true }));
    render(<Home />);
    expect(await screen.findByText("Ola, Ana")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sair" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Entrar" })).not.toBeInTheDocument();
    expect(!!screen.queryByRole("link", { name: "Painel" })).toBe(painel);
  });

  it("sessao corrompida e tratada como visitante", async () => {
    localStorage.setItem("auth_user", "{quebrado");
    render(<Home />);
    expect(await screen.findByRole("link", { name: "Entrar" })).toBeInTheDocument();
  });

  it("HTML do servidor e sempre o de visitante (sem mismatch de hydration)", () => {
    localStorage.setItem("auth_user", JSON.stringify({ id: 1, nome: "Ana", email: "a@x", role: "usuario", ativo: true }));
    const html = renderToStaticMarkup(<Home />);
    expect(html).toContain("Entrar");
    expect(html).not.toContain("Ola, Ana");
  });
});
