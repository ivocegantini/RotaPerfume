/**
 * FE-19: os formularios recusam dinheiro com mais de 2 casas ou acima de
 * DECIMAL(15,2) com mensagem amigavel e sem chamar onSubmit (ARQ-09).
 */
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ClienteResumo,
  MeResponse,
  Oportunidade,
  Pagamento,
  PedidoDetalhe,
  Produto,
  Vendedor,
  VendedorCompleto,
} from "@/lib/types";

const api = vi.hoisted(() => ({
  apiMe: vi.fn(),
  apiListVendedores: vi.fn(),
  apiListProdutos: vi.fn(),
  apiListClientesDoVendedor: vi.fn(),
  apiListPedidos: vi.fn(),
  apiGetVendedor: vi.fn(),
  apiListClientes: vi.fn(),
  apiVincularCliente: vi.fn(),
  apiDesvincularCliente: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import { PagamentoModal } from "@/components/PagamentoModal";
import { OportunidadeModal } from "@/components/admin/OportunidadeModal";
import { PedidoModal } from "@/components/admin/PedidoModal";
import { ProdutoModal } from "@/components/admin/ProdutoModal";
import { VendedorModal } from "@/components/admin/VendedorModal";
import { clearSession, refreshSessionUser } from "@/lib/session";

const me: MeResponse = {
  id: 1, nome: "Ana", email: "a@x", role: "admin", ativo: true, id_vendedor: 7, vendedor_nome: "Vend 7",
};
const vendedores: Vendedor[] = [
  { id: 3, nome: "Vend 3", regiao: "S", uf: "SP", data_desligamento: null },
  { id: 7, nome: "Vend 7", regiao: "S", uf: "SP", data_desligamento: null },
];
function cliente(id: number): ClienteResumo {
  return {
    id, cnpj: "00", razao_social: `Cliente ${id}`, segmento: "", cidade: "", uf: "SP",
    carteira_id: id, data_inicio: "2026-01-01", data_fim: null,
  };
}
const produto = {
  id: 50, sku: "P50", nome: "Perfume 50", categoria: "Perfumaria", marca: "Marca X",
  nota_olfativa: "Floral", preco_tabela: 99.9, custo_unitario: 40, unidade: "UN",
  data_lancamento: "2024-03-01", ativo: true, imagem_url: null, created_at: "", updated_at: "",
} as Produto;
const vendedor: VendedorCompleto = {
  id: 3, nome: "Vend 3", regiao: "Sudeste", uf: "SP", data_admissao: "2020-02-02",
  data_desligamento: null, meta_mensal: 5000, created_at: "", updated_at: "",
};
const pagamento: Pagamento = {
  pagamento_id: 77, pedido_id: 12, forma_pagamento: "PIX", parcelas: 3, valor: 300, taxa_pct: 1.5,
  valor_liquido: 295.5, data_vencimento: "2026-10-05T00:00:00Z", data_pagamento: null,
  status_pagamento: "Em aberto", created_at: "", updated_at: "",
};
const oportunidade: Oportunidade = {
  oportunidade_id: 1, cliente_id: 301, vendedor_id: 3, origem: "Instagram", data_abertura: "2026-08-01",
  etapa: "Negociação", probabilidade_pct: 60, valor_estimado: 1500, data_fechamento: "2026-09-10",
  ciclo_dias: 40, motivo_perda: null, created_at: "", updated_at: "",
};
const pedido = {
  pedido_id_origem: 5, cliente_id: 701, cliente_nome: "Cliente 701", vendedor_id: 7,
  data_pedido: "2026-09-01T00:00:00Z", canal: "App", status: "Em separação", valor_total: 20,
  itens: [{ item_id_origem: 1, produto_id: 50, quantidade: 2, preco_praticado: 10, desconto_pct: 0 }],
} as unknown as PedidoDetalhe;

interface Caso {
  nome: string;
  campo: string;
  rotulo: string;
  sufixo?: string;
  render: (onSubmit: () => Promise<void>) => ReactElement;
  pronto: () => Promise<unknown>;
}

const fechar = () => {};
const CASOS: Caso[] = [
  {
    nome: "ProdutoModal preco de tabela", campo: "Preco de tabela", rotulo: "Preco de tabela",
    render: (s) => <ProdutoModal open mode="edit" produto={produto} onClose={fechar} onSubmit={s} />,
    pronto: async () => undefined,
  },
  {
    nome: "ProdutoModal custo unitario", campo: "Custo unitario", rotulo: "Custo unitario",
    render: (s) => <ProdutoModal open mode="edit" produto={produto} onClose={fechar} onSubmit={s} />,
    pronto: async () => undefined,
  },
  {
    nome: "VendedorModal meta mensal", campo: "Meta mensal", rotulo: "Meta mensal",
    render: (s) => <VendedorModal open mode="edit" vendedor={vendedor} onClose={fechar} onSubmit={s} />,
    pronto: () =>
      waitFor(() =>
        expect(screen.queryByText("Carregando dados do vendedor...")).not.toBeInTheDocument()
      ),
  },
  {
    nome: "PagamentoModal valor", campo: "Valor", rotulo: "Valor",
    render: (s) => <PagamentoModal open mode="edit" pagamento={pagamento} onClose={fechar} onSubmit={s} />,
    pronto: async () => undefined,
  },
  {
    nome: "PagamentoModal valor liquido", campo: "Valor liquido", rotulo: "Valor liquido",
    render: (s) => <PagamentoModal open mode="edit" pagamento={pagamento} onClose={fechar} onSubmit={s} />,
    pronto: async () => undefined,
  },
  {
    nome: "OportunidadeModal valor estimado", campo: "Valor estimado", rotulo: "Valor estimado",
    render: (s) => (
      <OportunidadeModal open mode="edit" oportunidade={oportunidade} onClose={fechar} onSubmit={s} />
    ),
    pronto: () => screen.findByRole("option", { name: "#301 - Cliente 301" }),
  },
  {
    nome: "PedidoModal preco praticado", campo: "Preco", rotulo: "Preco praticado",
    sufixo: " em todos os itens",
    render: (s) => <PedidoModal open mode="edit" pedido={pedido} onClose={fechar} onSubmit={s} />,
    pronto: async () => {
      await screen.findByRole("option", { name: "#50 - Perfume 50" });
      await screen.findByRole("option", { name: /^#701 - Cliente 701/ });
    },
  },
];

beforeEach(async () => {
  vi.clearAllMocks();
  clearSession();
  api.apiMe.mockResolvedValue(me);
  await act(async () => {
    await refreshSessionUser();
  });
  api.apiListVendedores.mockResolvedValue(vendedores);
  api.apiListProdutos.mockResolvedValue({ data: [produto], page: 1, limit: 100, total: 1, pages: 1 });
  api.apiListClientesDoVendedor.mockImplementation(async (id: number) => [cliente(id * 100 + 1)]);
  api.apiListPedidos.mockResolvedValue({ data: [], page: 1, limit: 20, total: 0, pages: 0 });
  api.apiGetVendedor.mockResolvedValue({ ...vendedor, clientes: null });
  api.apiListClientes.mockResolvedValue({ data: [], page: 1, limit: 100, total: 0, pages: 1 });
});

async function enviarCom(c: Caso, texto: string) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  render(c.render(onSubmit));
  await c.pronto();
  fireEvent.change(screen.getByLabelText(c.campo), { target: { value: texto } });
  const form = screen.getByRole("button", { name: "Salvar alteracoes" }).closest("form")!;
  await act(async () => {
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
  return onSubmit;
}

describe.each(CASOS)("FE-19: $nome", (c) => {
  const sufixo = c.sufixo ?? "";

  it("mais de 2 casas decimais: mensagem amigavel e nao envia", async () => {
    const onSubmit = await enviarCom(c, "10.555");
    expect(
      await screen.findByText(`${c.rotulo} deve ter no maximo 2 casas decimais${sufixo}.`)
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("acima de DECIMAL(15,2): mensagem amigavel e nao envia", async () => {
    const onSubmit = await enviarCom(c, "10000000000000");
    expect(
      await screen.findByText(`${c.rotulo} deve ser no maximo 9.999.999.999.999,99${sufixo}.`)
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("negativo: mensagem amigavel e nao envia", async () => {
    const onSubmit = await enviarCom(c, "-1");
    expect(
      await screen.findByText(`${c.rotulo} deve ser um numero maior ou igual a zero${sufixo}.`)
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("2 casas (zeros a direita ignorados) passa e envia", async () => {
    const onSubmit = await enviarCom(c, "12.340");
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
  });
});
