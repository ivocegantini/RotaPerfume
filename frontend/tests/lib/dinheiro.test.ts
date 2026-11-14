/** FE-19: validador de dinheiro, espelho do ParseDinheiro do backend (ARQ-09). */
import { describe, expect, it } from "vitest";
import {
  msgDinheiroCasas,
  msgDinheiroInvalido,
  msgDinheiroLimite,
  validarDinheiro,
} from "@/lib/dinheiro";

describe("validarDinheiro", () => {
  it.each<[string, number]>([
    ["0", 0],
    ["10", 10],
    ["10.5", 10.5],
    ["10.55", 10.55],
    ["  123.45 ", 123.45],
    ["10.500", 10.5], // zeros a direita nao contam
    ["1.2300000", 1.23],
    [".5", 0.5],
    ["5.", 5],
    ["0.01", 0.01],
    ["0.10", 0.1],
    ["1e2", 100],
    ["1.5E1", 15],
    ["1234e-2", 12.34],
    ["-0", 0],
    ["0.000", 0],
    ["9999999999999.99", 9999999999999.99],
  ])("aceita %j -> %d", (txt, esperado) => {
    expect(validarDinheiro(txt, "Valor")).toEqual({ ok: true, valor: esperado });
  });

  it("o numero devolvido serializa sem lixo de ponto flutuante", () => {
    const r = validarDinheiro("0.30", "Valor");
    expect(r.ok && JSON.stringify(r.valor)).toBe("0.3");
    const t = validarDinheiro("9999999999999.99", "Valor");
    expect(t.ok && JSON.stringify(t.valor)).toBe("9999999999999.99");
  });

  it.each(["10.555", "0.001", "1.005", "99.999", "1e-3", "12345e-3"])(
    "recusa mais de 2 casas: %j",
    (txt) => {
      expect(validarDinheiro(txt, "Valor")).toEqual({
        ok: false,
        erro: "Valor deve ter no maximo 2 casas decimais.",
      });
    }
  );

  it.each(["10000000000000", "9999999999999.991", "1e13", "1e400", "99999999999999999999"])(
    "recusa acima de DECIMAL(15,2): %j",
    (txt) => {
      const r = validarDinheiro(txt, "Valor");
      // 9999999999999.991 tem 3 casas: cai na regra das casas antes do teto
      const esperado = txt.includes(".") ? msgDinheiroCasas("Valor") : msgDinheiroLimite("Valor");
      expect(r).toEqual({ ok: false, erro: esperado });
    }
  );

  it.each(["", "   ", "-1", "-0.01", "abc", "NaN", "Infinity", "0x10", "1,5", ".", "1e", "--1", "+1"])(
    "recusa vazio, negativo ou nao numerico: %j",
    (txt) => {
      expect(validarDinheiro(txt, "Valor")).toEqual({
        ok: false,
        erro: "Valor deve ser um numero maior ou igual a zero.",
      });
    }
  );

  it("usa o rotulo e o sufixo nas mensagens", () => {
    expect(validarDinheiro("1.234", "Preco praticado", " em todos os itens")).toEqual({
      ok: false,
      erro: "Preco praticado deve ter no maximo 2 casas decimais em todos os itens.",
    });
    expect(msgDinheiroInvalido("Meta mensal")).toBe(
      "Meta mensal deve ser um numero maior ou igual a zero."
    );
    expect(msgDinheiroLimite("Valor")).toBe("Valor deve ser no maximo 9.999.999.999.999,99.");
  });
});
