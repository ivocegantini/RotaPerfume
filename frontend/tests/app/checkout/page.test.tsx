/** ECOM-13/14/17: botao Comprar (vitrine e produto), carrinho lateral e /checkout (endereco obrigatorio). */
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ProdutoLojaDetalhe } from "@/lib/types";

const params = vi.hoisted(() => ({ id: "7" as string | undefined }));
vi.mock("next/navigation", () => ({ useParams: () => params, useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));

const produto: ProdutoLojaDetalhe = {
  id: 7, sku: "SKU-7", nome: "Perfume Oud", categoria: "EDP", marca: "Sahra", nota_olfativa: "Oud",
  preco: 100.5, unidade: "UN", imagem_url: null, descricao: "",
};
const finalizarMock = vi.fn();
vi.mock("@/lib/api", () => ({
  apiFinalizarCompra: (...a: unknown[]) => finalizarMock(...a),
  apiGetProdutoLoja: () => Promise.resolve(produto),
  apiListProdutosLoja: () => Promise.resolve({ data: [produto], page: 1, limit: 24, total: 1, pages: 1 }),
  apiOpcoesFiltroLoja: () => Promise.resolve({ categorias: [], marcas: [], notas_olfativas: [] }),
}));

const cepMock = vi.fn();
vi.mock("@/lib/viaCep", () => ({ buscarCEP: (...a: unknown[]) => cepMock(...a) }));

import LojaPage from "@/app/loja/page";
import ProdutoPage from "@/app/produtos/[id]/page";
import CheckoutPage from "@/app/checkout/page";
import { obterItens } from "@/lib/carrinho";
import { novaChave } from "@/components/loja/Checkout";

const lateral = () => within(screen.getAllByRole("complementary", { name: "Carrinho de compras" })[0]);

beforeEach(() => {
  localStorage.clear();
  finalizarMock.mockReset();
  cepMock.mockReset();
  cepMock.mockResolvedValue({ logradouro: "Avenida Paulista", bairro: "Bela Vista", cidade: "São Paulo", uf: "SP" });
});

// ECOM-17: digita o CEP, espera a pesquisa liberar os campos e informa o numero.
const preencherEndereco = async () => {
  fireEvent.change(screen.getByLabelText("CEP"), { target: { value: "01310100" } });
  await waitFor(() => expect(screen.getByLabelText("Rua")).toBeEnabled());
  fireEvent.change(screen.getByLabelText("Numero"), { target: { value: "1000" } });
};

const umItem = () =>
  localStorage.setItem("carrinho_loja", JSON.stringify([{ id: 7, nome: "Perfume Oud", marca: "S", preco: 10, imagem_url: null, quantidade: 2 }]));
const logar = () => localStorage.setItem("auth_user", JSON.stringify({ nome: "Ana", role: "usuario" }));

describe("Comprar e carrinho lateral", () => {
  it("vitrine: Comprar adiciona; + soma mais do mesmo; - e Remover tiram", async () => {
    render(<LojaPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Comprar Perfume Oud" }));
    expect(obterItens()).toMatchObject([{ id: 7, quantidade: 1 }]);
    const c = lateral();
    expect(c.getByRole("link", { name: "Ir para o pagamento" })).toHaveAttribute("href", "/checkout");
    fireEvent.click(c.getByRole("button", { name: "Adicionar mais Perfume Oud" }));
    fireEvent.click(c.getByRole("button", { name: "Adicionar mais Perfume Oud" }));
    expect(obterItens()[0].quantidade).toBe(3);
    expect(c.getAllByText(/301,50/)).toHaveLength(2); // subtotal e total
    fireEvent.click(c.getByRole("button", { name: "Diminuir Perfume Oud" }));
    expect(obterItens()[0].quantidade).toBe(2);
    fireEvent.click(c.getByRole("button", { name: "Remover Perfume Oud" }));
    expect(c.getByText("Seu carrinho esta vazio.")).toBeInTheDocument();
    expect(c.getByRole("button", { name: "Ir para o pagamento" })).toBeDisabled();
  });

  it("pagina do produto: Comprar adiciona e abre a gaveta (celular)", async () => {
    render(<ProdutoPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Comprar Perfume Oud" }));
    expect(obterItens()).toMatchObject([{ id: 7, quantidade: 1 }]);
    const gaveta = screen.getByRole("dialog", { name: "Carrinho de compras" });
    fireEvent.click(within(gaveta).getByRole("button", { name: "Fechar carrinho" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("button", { name: "Abrir carrinho (1 item)" })).toBeInTheDocument();
  });
});

describe("CheckoutPage", () => {
  it("vazio mostra o aviso", () => {
    render(<CheckoutPage />);
    expect(screen.getByText(/Seu carrinho esta vazio/)).toBeInTheDocument();
  });

  it("visitante e convidado a entrar; logado escolhe a forma", () => {
    umItem();
    const { unmount } = render(<CheckoutPage />);
    expect(screen.getByRole("heading", { level: 1, name: "Pagamento" })).toBeInTheDocument();
    expect(screen.getAllByText(/20,00/)).toHaveLength(2);
    expect(screen.getByRole("link", { name: "Entre na sua conta" })).toHaveAttribute("href", "/login");
    expect(screen.queryByRole("link", { name: "Ir para o pagamento" })).toBeNull();
    unmount();

    logar();
    render(<CheckoutPage />);
    expect(screen.getByRole("radio", { name: "Pix" })).toBeChecked();
    fireEvent.click(screen.getByRole("radio", { name: "Boleto" }));
    expect(screen.getByRole("radio", { name: "Boleto" })).toBeChecked();
    expect(screen.getByRole("button", { name: "Finalizar pedido" })).toBeEnabled();
  });

  it("Finalizar envia so produto/quantidade, limpa o carrinho e mostra o pedido", async () => {
    umItem();
    logar();
    finalizarMock.mockResolvedValue({
      pedido_id: 1000, data_pedido: "2026-11-18", status: "Em separação", valor_total: 20, forma_pagamento: "Boleto 14 dias",
      status_pagamento: "Em aberto", data_vencimento: "2026-12-02", repetida: false, itens: [],
    });
    render(<CheckoutPage />);
    fireEvent.click(screen.getByRole("radio", { name: "Boleto" }));
    await preencherEndereco();
    fireEvent.click(screen.getByRole("button", { name: "Finalizar pedido" }));
    const titulo = await screen.findByRole("heading", { name: "Pedido #1000 recebido!" });
    const [corpo] = finalizarMock.mock.calls[0] as [{ chave: string; forma_pagamento: string; itens: unknown[]; entrega: unknown }];
    expect(corpo.entrega).toEqual({ cep: "01310100", endereco: "Avenida Paulista", numero: "1000", complemento: "",
      bairro: "Bela Vista", cidade: "São Paulo", uf: "SP" });
    expect(corpo.forma_pagamento).toBe("boleto");
    expect(corpo.itens).toEqual([{ produto_id: 7, quantidade: 2 }]); // sem preco
    expect(corpo.chave).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    expect(obterItens()).toEqual([]);
    expect(screen.getByText(/02\/12\/2026/)).toBeInTheDocument();
    expect(within(titulo.closest("section")!).getByRole("link", { name: "Minhas compras" })).toHaveAttribute("href", "/minhas-compras");
  });

  it("erro da API aparece e o carrinho fica; reenvio usa a mesma chave", async () => {
    umItem();
    logar();
    finalizarMock.mockRejectedValueOnce(new Error("um ou mais produtos não estão mais disponíveis na loja"));
    render(<CheckoutPage />);
    await preencherEndereco();
    fireEvent.click(screen.getByRole("button", { name: "Finalizar pedido" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("não estão mais disponíveis");
    expect(obterItens()).toHaveLength(1);
    finalizarMock.mockRejectedValueOnce(new Error("falhou"));
    fireEvent.click(screen.getByRole("button", { name: "Finalizar pedido" }));
    await waitFor(() => expect(finalizarMock).toHaveBeenCalledTimes(2));
    const chaves = finalizarMock.mock.calls.map((c) => (c[0] as { chave: string }).chave);
    expect(chaves[0]).toBe(chaves[1]);
  });

  it("ECOM-17: sem endereco nao finaliza; campos so liberam depois da pesquisa do CEP", async () => {
    umItem();
    logar();
    render(<CheckoutPage />);
    expect(screen.getByLabelText("Cidade")).toBeDisabled();
    expect(screen.getByLabelText("Estado")).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Finalizar pedido" }));
    expect(screen.getAllByRole("alert").map((a) => a.textContent).join(" ")).toMatch(/Preencha o endereco de entrega/);
    expect(finalizarMock).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("CEP"), { target: { value: "01310100" } });
    await waitFor(() => expect(screen.getByLabelText("Cidade")).toHaveValue("São Paulo"));
    expect(cepMock).toHaveBeenCalledWith("01310100");
    expect(screen.getByLabelText("CEP")).toHaveValue("01310-100");
    expect(screen.getByLabelText("Estado")).toHaveValue("SP");
    expect(screen.getByLabelText("Bairro")).toHaveValue("Bela Vista");
    // cidade e estado continuam abertos (editaveis)
    fireEvent.change(screen.getByLabelText("Cidade"), { target: { value: "Osasco" } });
    expect(screen.getByLabelText("Cidade")).toHaveValue("Osasco");
    // sem numero ainda: continua bloqueado
    fireEvent.click(screen.getByRole("button", { name: "Finalizar pedido" }));
    expect(screen.getByText(/Numero e obrigatorio/)).toBeInTheDocument();
    expect(finalizarMock).not.toHaveBeenCalled();
  });

  it("ECOM-17: CEP nao encontrado libera os campos vazios para digitar", async () => {
    umItem();
    logar();
    cepMock.mockResolvedValue(null);
    render(<CheckoutPage />);
    fireEvent.change(screen.getByLabelText("CEP"), { target: { value: "99999-999" } });
    await waitFor(() => expect(screen.getByLabelText("Cidade")).toBeEnabled());
    expect(screen.getByText(/CEP nao encontrado/)).toBeInTheDocument();
    expect(screen.getByLabelText("Cidade")).toHaveValue("");
  });

  it("novaChave gera UUID v4 tambem sem randomUUID (HTTP fora do localhost)", () => {
    const original = crypto.randomUUID;
    Object.defineProperty(crypto, "randomUUID", { value: undefined, configurable: true });
    try {
      const a = novaChave();
      expect(a).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
      expect(novaChave()).not.toBe(a);
    } finally {
      Object.defineProperty(crypto, "randomUUID", { value: original, configurable: true });
    }
  });
});
