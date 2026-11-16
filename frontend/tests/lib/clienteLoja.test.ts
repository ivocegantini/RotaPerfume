/** ECOM-05: CPF, CEP e mascaras do cadastro da loja. */
import { describe, expect, it } from "vitest";
import { cepValido, cpfValido, mascararCEP, mascararCPF } from "@/lib/clienteLoja";

describe("cpfValido", () => {
  it.each(["52998224725", "529.982.247-25", "111.444.777-35"])("%s valido", (c) => expect(cpfValido(c)).toBe(true));
  it.each(["", "52998224724", "11111111111", "5299822472", "52998224725x", "529982247250"])("%j invalido", (c) =>
    expect(cpfValido(c)).toBe(false)
  );
});

describe("mascaras e CEP", () => {
  it.each([
    ["5", "5"],
    ["5299", "529.9"],
    ["5299822", "529.982.2"],
    ["5299822472", "529.982.247-2"],
    ["529982247259999", "529.982.247-25"],
  ])("CPF %s -> %s", (e, s) => expect(mascararCPF(e)).toBe(s));
  it("CEP", () => {
    expect(mascararCEP("01001000")).toBe("01001-000");
    expect(mascararCEP("0100")).toBe("0100");
    expect(cepValido("01001-000")).toBe(true);
    expect(cepValido("0100100")).toBe(false);
    expect(cepValido("01001-00a")).toBe(false);
  });
});
