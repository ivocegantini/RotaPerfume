/** ECOM-03: pagina de confirmacao do e-mail (token no #fragmento) e reenvio. */
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock }) }));
const confirmarMock = vi.fn();
const reenviarMock = vi.fn();
vi.mock("@/lib/api", () => ({
  apiConfirmarEmail: (t: string) => confirmarMock(t),
  apiReenviarConfirmacao: (e: string, c?: string) => reenviarMock(e, c),
}));
vi.mock("@/components/ui/Turnstile", async () => {
  const React = await import("react");
  const Turnstile = React.forwardRef<{ reset: () => void }, { action: string }>(function Fake({ action }, ref) {
    React.useImperativeHandle(ref, () => ({ reset: () => {} }));
    return <div data-testid="turnstile" data-action={action} />;
  });
  return { Turnstile, TURNSTILE_ACTION_REENVIO: "reenviar-confirmacao", isTurnstileEnabled: false };
});

import ConfirmarEmailPage from "@/app/confirmar-email/page";
import { lerAvisoLogin, limparAvisoLogin } from "@/lib/avisoLogin";

const TOKEN = "ab".repeat(32);

beforeEach(() => {
  replaceMock.mockReset();
  confirmarMock.mockReset();
  reenviarMock.mockReset();
  limparAvisoLogin();
});
afterEach(() => window.history.replaceState(null, "", "/"));

describe("ConfirmarEmailPage", () => {
  it("com token valido: confirma uma vez, avisa o login, limpa a URL e redireciona", async () => {
    window.history.replaceState(null, "", `/confirmar-email#token=${TOKEN}`);
    confirmarMock.mockResolvedValue({ mensagem: "ok" });
    render(<ConfirmarEmailPage />);
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/login"));
    expect(confirmarMock).toHaveBeenCalledTimes(1);
    expect(confirmarMock).toHaveBeenCalledWith(TOKEN);
    expect(lerAvisoLogin()).toMatch(/Email confirmado/);
    expect(window.location.hash).toBe("");
  });

  it("link invalido: mostra o erro e o formulario de reenvio", async () => {
    window.history.replaceState(null, "", `/confirmar-email#token=${TOKEN}`);
    confirmarMock.mockRejectedValue(new Error("link de confirmação inválido ou expirado"));
    render(<ConfirmarEmailPage />);
    expect(await screen.findByText(/inválido ou expirado/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reenviar link" })).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("sem token: so o formulario; reenvio usa a action propria e mostra aviso neutro", async () => {
    reenviarMock.mockResolvedValue({ mensagem: "ok" });
    render(<ConfirmarEmailPage />);
    expect(confirmarMock).not.toHaveBeenCalled();
    expect(screen.getByTestId("turnstile")).toHaveAttribute("data-action", "reenviar-confirmacao");
    await userEvent.type(screen.getByLabelText("Email"), "ana@x.com");
    await userEvent.click(screen.getByRole("button", { name: "Reenviar link" }));
    expect(await screen.findByText(/Se houver um cadastro pendente/)).toBeInTheDocument();
    expect(reenviarMock).toHaveBeenCalledWith("ana@x.com", undefined);
  });

  it("email invalido nao chama a API", async () => {
    render(<ConfirmarEmailPage />);
    await userEvent.type(screen.getByLabelText("Email"), "sem-arroba");
    await userEvent.click(screen.getByRole("button", { name: "Reenviar link" }));
    expect(screen.getByText("Email invalido")).toBeInTheDocument();
    expect(reenviarMock).not.toHaveBeenCalled();
  });
});
