/** ECOM-01: tela inicial por papel e politica de senha do cadastro. */
import { describe, expect, it } from "vitest";
import { rotaInicial } from "@/lib/auth";
import { erroSenhaCadastro } from "@/lib/senhaCadastro";

describe("rotaInicial", () => {
  it.each([
    ["admin", "/dashboard"],
    ["normal", "/pagamentos"],
    ["usuario", "/loja"],
  ] as const)("%s -> %s", (role, destino) => expect(rotaInicial(role)).toBe(destino));
});

describe("erroSenhaCadastro", () => {
  it.each([
    ["", "Senha e obrigatoria"],
    ["Ab1!", "Senha deve ter pelo menos 8 caracteres"],
    ["minusculas", "Use ao menos 3 de: minuscula, maiuscula, numero e simbolo"],
    ["x".repeat(73), "Senha muito longa (maximo 72 bytes)"],
  ])("%j recusada", (senha, msg) => expect(erroSenhaCadastro(senha)).toBe(msg));

  it.each(["Loja#2026ok", "abcdEFGH1", "abcd1234!"])("%s aceita", (s) => expect(erroSenhaCadastro(s)).toBeUndefined());
});
