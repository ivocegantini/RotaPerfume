/** ECOM-07: status "Pendente de e-mail" na lista de usuarios do admin. */
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { User } from "@/lib/types";

vi.mock("@/components/layout/ProtectedRoute", () => ({
  ProtectedRoute: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
const api = vi.hoisted(() => ({
  apiListUsers: vi.fn(),
  apiCreateUser: vi.fn(),
  apiUpdateUser: vi.fn(),
  apiToggleUserStatus: vi.fn(),
  apiAdminResetPassword: vi.fn(),
  apiListVendedores: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

import UsuariosPage from "@/app/admin/usuarios/page";

const u = (id: number, extra: Partial<User>): User => ({
  id, nome: `Usuario ${id}`, email: `u${id}@gmail.com`, role: "usuario", ativo: true, id_vendedor: null, ...extra,
});

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.apiListVendedores.mockResolvedValue([]);
  api.apiListUsers.mockResolvedValue({
    data: [u(1, { status: "ativo" }), u(2, { status: "pendente_email" }), u(3, { ativo: false, status: "inativo" }), u(4, {})],
    page: 1, limit: 20, total: 4, pages: 1,
  });
});

describe("Usuarios - status (ECOM-07)", () => {
  it("mostra 'Pendente de e-mail' e deriva de ativo quando a API nao manda status", async () => {
    render(<UsuariosPage />);
    expect(await screen.findByText("Pendente de e-mail", { selector: "button" })).toBeInTheDocument();
    const linha = (nome: string) => screen.getByText(nome).closest("tr") as HTMLElement;
    expect(within(linha("Usuario 2")).getByRole("button", { name: /Pendente de e-mail/ })).toBeInTheDocument();
    expect(within(linha("Usuario 3")).getByRole("button", { name: /Inativo/ })).toBeInTheDocument();
    expect(within(linha("Usuario 4")).getByRole("button", { name: /Ativo/ })).toBeInTheDocument();
  });

  it("filtro por status pendente", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 2");
    const filtro = screen.getAllByRole("combobox").find((el) => within(el).queryByRole("option", { name: "Pendente de e-mail" }));
    expect(filtro).toBeDefined();
    await userEvent.selectOptions(filtro!, "pendente_email");
    await waitFor(() => expect(screen.queryByText("Usuario 1")).not.toBeInTheDocument());
    expect(screen.getByText("Usuario 2")).toBeInTheDocument();
    expect(screen.queryByText("Usuario 3")).not.toBeInTheDocument();
  });

  it("Resetar desabilitado para conta pendente (ECOM-08)", async () => {
    render(<UsuariosPage />);
    await screen.findByText("Usuario 2");
    const linha = (nome: string) => screen.getByText(nome).closest("tr") as HTMLElement;
    const pend = within(linha("Usuario 2")).getByRole("button", { name: "Resetar" });
    expect(pend).toBeDisabled();
    expect(pend).toHaveAttribute("title", expect.stringContaining("nao confirmou o email"));
    expect(within(linha("Usuario 1")).getByRole("button", { name: "Resetar" })).toBeEnabled();
  });
});
