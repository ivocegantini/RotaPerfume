/** ECOM-03: token do fragmento do link e deteccao do 403 de e-mail pendente. */
import { describe, expect, it } from "vitest";
import { ehEmailNaoConfirmado, tokenDoHash } from "@/lib/confirmacaoEmail";

describe("tokenDoHash", () => {
  const tok = "a".repeat(64);
  it.each([
    [`#token=${tok}`, tok],
    ["", ""],
    ["#token=", ""],
    ["#token=xyz<script>", ""],
    [`#outra=${tok}`, ""],
  ])("%j -> %j", (hash, esperado) => expect(tokenDoHash(hash)).toBe(esperado));
});

describe("ehEmailNaoConfirmado", () => {
  it("reconhece a mensagem do backend", () => {
    expect(ehEmailNaoConfirmado("email não confirmado: abra o link que enviamos para o seu email")).toBe(true);
    expect(ehEmailNaoConfirmado("credenciais inválidas")).toBe(false);
  });
});
