/** ECOM-13: carrinho no localStorage (validacao, soma, limites, total). */
import { beforeEach, describe, expect, it } from "vitest";
import {
  adicionarAoCarrinho,
  alterarQuantidade,
  CHAVE_CARRINHO,
  ITENS_MAXIMOS,
  lerItens,
  limparCarrinho,
  obterItens,
  QTD_MAXIMA,
  removerDoCarrinho,
  totalCarrinho,
} from "@/lib/carrinho";

const p = { id: 7, nome: "Oud", marca: "Sahra", preco: 10.1, imagem_url: null };

beforeEach(() => localStorage.clear());

describe("carrinho", () => {
  it("adicionar soma no mesmo item; alterar/remover/limpar", () => {
    adicionarAoCarrinho(p);
    adicionarAoCarrinho(p);
    adicionarAoCarrinho({ ...p, id: 8 });
    expect(obterItens().map((i) => [i.id, i.quantidade])).toEqual([[7, 2], [8, 1]]);
    alterarQuantidade(7, 5);
    expect(obterItens()[0].quantidade).toBe(5);
    alterarQuantidade(7, 0);
    expect(obterItens().map((i) => i.id)).toEqual([8]);
    removerDoCarrinho(8);
    expect(localStorage.getItem(CHAVE_CARRINHO)).toBeNull();
    adicionarAoCarrinho(p);
    limparCarrinho();
    expect(obterItens()).toEqual([]);
  });

  it("limita quantidade e numero de itens", () => {
    alterarQuantidade(7, 1);
    adicionarAoCarrinho(p);
    alterarQuantidade(7, 1000);
    expect(obterItens()[0].quantidade).toBe(QTD_MAXIMA);
    adicionarAoCarrinho(p);
    expect(obterItens()[0].quantidade).toBe(QTD_MAXIMA);
    for (let i = 100; obterItens().length < ITENS_MAXIMOS; i++) adicionarAoCarrinho({ ...p, id: i });
    expect(adicionarAoCarrinho({ ...p, id: 9999 })).toBe(false);
    expect(obterItens()).toHaveLength(ITENS_MAXIMOS);
  });

  it("storage adulterado: descarta lixo, duplicados e valores invalidos", () => {
    expect(lerItens("nao json")).toEqual([]);
    expect(lerItens('{"id":1}')).toEqual([]);
    const itens = lerItens(
      JSON.stringify([
        { ...p, quantidade: 2 },
        { ...p, quantidade: 3 }, // duplicado
        { ...p, id: 2, quantidade: 1.5 },
        { ...p, id: 3, quantidade: 1, preco: -1 },
        { ...p, id: 4, quantidade: 1, preco: "1" },
        { ...p, id: 5, quantidade: 1, imagem_url: 5 },
        { ...p, id: 6, quantidade: 500 },
        null,
      ])
    );
    expect(itens.map((i) => [i.id, i.quantidade])).toEqual([[7, 2], [6, QTD_MAXIMA]]);
  });

  it("total em unidades e valor arredondado", () => {
    expect(totalCarrinho(lerItens(JSON.stringify([{ ...p, quantidade: 3 }, { ...p, id: 2, preco: 0.2, quantidade: 1 }])))).toEqual({
      unidades: 4,
      valor: 30.5,
    });
  });
});
