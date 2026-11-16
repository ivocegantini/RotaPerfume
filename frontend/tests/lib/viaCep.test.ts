/** ECOM-09: busca de endereco pelo CEP (ViaCEP). */
import { describe, expect, it, vi } from "vitest";
import { buscarCEP } from "@/lib/viaCep";

const resposta = (corpo: unknown, ok = true) =>
  vi.fn(async () => ({ ok, json: async () => corpo }) as unknown as Response) as unknown as typeof fetch;

describe("buscarCEP", () => {
  it("preenche a partir da resposta e so envia o CEP", async () => {
    const f = resposta({ cep: "01001-000", logradouro: "Praça da Sé", bairro: "Sé", localidade: "São Paulo", uf: "SP" });
    expect(await buscarCEP("01001-000", f)).toEqual({ logradouro: "Praça da Sé", bairro: "Sé", cidade: "São Paulo", uf: "SP" });
    expect(f).toHaveBeenCalledWith("https://viacep.com.br/ws/01001000/json/", expect.objectContaining({ credentials: "omit" }));
  });

  it.each([
    ["CEP incompleto", "0100", resposta({})],
    ["CEP inexistente", "99999999", resposta({ erro: true })],
    ["HTTP 400", "01001000", resposta({}, false)],
    ["UF desconhecida", "01001000", resposta({ localidade: "X", uf: "ZZ" })],
    ["sem cidade", "01001000", resposta({ uf: "SP" })],
    ["resposta nao-objeto", "01001000", resposta("lixo")],
    ["falha de rede", "01001000", vi.fn(async () => { throw new Error("rede"); }) as unknown as typeof fetch],
  ])("%s -> null", async (_n, cep, f) => {
    expect(await buscarCEP(cep, f)).toBeNull();
  });
});
