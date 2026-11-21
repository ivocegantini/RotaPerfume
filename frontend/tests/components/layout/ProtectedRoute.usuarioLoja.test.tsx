/** ECOM-01: o cliente da loja ("usuario") nunca ve a area interna. */
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MeResponse } from "@/lib/types";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock }) }));
const refreshSessionUserMock = vi.fn<() => Promise<MeResponse>>();
const getSessionUserMock = vi.fn<() => MeResponse | null>();
vi.mock("@/lib/session", () => ({
  refreshSessionUser: () => refreshSessionUserMock(),
  getSessionUser: () => getSessionUserMock(),
  clearSession: vi.fn(),
}));

import { ProtectedRoute } from "@/components/layout/ProtectedRoute";

const me = (role: MeResponse["role"]): MeResponse => ({ id: 9, nome: "Cli", email: "c@x", role, ativo: true });

beforeEach(() => {
  replaceMock.mockReset();
  refreshSessionUserMock.mockReset();
  getSessionUserMock.mockReset().mockReturnValue(null);
  localStorage.setItem("auth_user", JSON.stringify(me("usuario")));
});

describe("ProtectedRoute - ECOM-01", () => {
  it("area interna com usuario da loja: redireciona para a vitrine sem renderizar", async () => {
    refreshSessionUserMock.mockResolvedValue(me("usuario"));
    render(<ProtectedRoute>interno</ProtectedRoute>);
    await vi.waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/"));
    expect(screen.queryByText("interno")).not.toBeInTheDocument();
  });

  it("sessao em memoria do usuario da loja nao renderiza a area interna de imediato", () => {
    getSessionUserMock.mockReturnValue(me("usuario"));
    refreshSessionUserMock.mockReturnValue(new Promise(() => {}));
    render(<ProtectedRoute>interno</ProtectedRoute>);
    expect(screen.queryByText("interno")).not.toBeInTheDocument();
  });

  it.each<MeResponse["role"]>(["usuario", "admin", "normal"])("area loja libera o papel %s", async (role) => {
    refreshSessionUserMock.mockResolvedValue(me(role));
    render(<ProtectedRoute area="loja">vitrine</ProtectedRoute>);
    expect(await screen.findByText("vitrine")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });
});
