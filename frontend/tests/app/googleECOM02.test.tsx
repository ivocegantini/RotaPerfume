/** ECOM-02: entrar e criar conta com Google (login e cadastro). */
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const replaceMock = vi.fn();
const pushMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock, push: pushMock }) }));

const api = vi.hoisted(() => ({
  apiLogin: vi.fn(),
  apiCadastro: vi.fn(),
  apiGoogle: vi.fn(),
  apiGoogleCadastro: vi.fn(),
}));
vi.mock("@/lib/api", () => api);

// Botao do Google falso: um clique entrega uma credencial.
vi.mock("@/components/ui/GoogleButton", () => ({
  isGoogleEnabled: true,
  GoogleButton: ({ onCredential, texto }: { onCredential: (c: string) => void; texto?: string }) => (
    <button type="button" data-texto={texto} onClick={() => onCredential("cred-google")}>
      Google
    </button>
  ),
}));
vi.mock("@/components/ui/Turnstile", async () => {
  const React = await import("react");
  const Turnstile = React.forwardRef<{ reset: () => void }, { action: string }>(function Fake(_p, ref) {
    React.useImperativeHandle(ref, () => ({ reset: () => {} }));
    return <div data-testid="turnstile" />;
  });
  return { Turnstile, TURNSTILE_ACTION_LOGIN: "login", TURNSTILE_ACTION_CADASTRO: "cadastro", isTurnstileEnabled: false };
});

import LoginPage from "@/app/login/page";
import CadastroPage from "@/app/cadastro/page";
import { definirCadastroGoogle, lerCadastroGoogle } from "@/lib/googleConta";

const userLoja = { id: 9, nome: "Ana", email: "ana@gmail.com", role: "usuario" as const, ativo: true };

beforeEach(() => {
  replaceMock.mockReset();
  pushMock.mockReset();
  Object.values(api).forEach((f) => f.mockReset());
  localStorage.clear();
  definirCadastroGoogle(null);
});

describe("Login com Google", () => {
  it("conta da loja: entra e vai para /loja", async () => {
    api.apiGoogle.mockResolvedValue({ token: "", user: userLoja });
    render(<LoginPage />);
    await userEvent.click(screen.getByRole("button", { name: "Google" }));
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/loja"));
    expect(api.apiGoogle).toHaveBeenCalledWith("cred-google");
    expect(JSON.parse(localStorage.getItem("auth_user")!)).toMatchObject({ id: 9, role: "usuario" });
  });

  it("sem conta: guarda o perfil so em memoria e vai para /cadastro", async () => {
    api.apiGoogle.mockResolvedValue({ cadastro_necessario: true, email: "ana@gmail.com", nome: "Ana" });
    render(<LoginPage />);
    await userEvent.click(screen.getByRole("button", { name: "Google" }));
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/cadastro"));
    expect(lerCadastroGoogle()).toEqual({ credential: "cred-google", email: "ana@gmail.com", nome: "Ana" });
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it("erro do backend aparece (ex.: so para clientes da loja)", async () => {
    api.apiGoogle.mockRejectedValue(new Error("o login com Google é só para clientes da loja; use email e senha"));
    render(<LoginPage />);
    await userEvent.click(screen.getByRole("button", { name: "Google" }));
    expect(await screen.findByText(/só para clientes da loja/)).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });
});

describe("Cadastro com Google", () => {
  it("vindo do login: pede so CPF e endereco, cria a conta e entra", async () => {
    definirCadastroGoogle({ credential: "cred-google", email: "ana@gmail.com", nome: "Ana" });
    api.apiGoogleCadastro.mockResolvedValue({ token: "", user: userLoja });
    render(<CadastroPage />);
    expect(screen.getByText(/Criando conta com Google/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Senha")).not.toBeInTheDocument();
    expect(screen.queryByTestId("turnstile")).not.toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("CPF"), "52998224725");
    await userEvent.type(screen.getByLabelText("Rua"), "Rua A");
    await userEvent.type(screen.getByLabelText("Numero"), "10");
    await userEvent.type(screen.getByLabelText("Cidade"), "Sao Paulo");
    await userEvent.selectOptions(screen.getByLabelText("UF"), "SP");
    await userEvent.click(screen.getByRole("button", { name: "Criar conta com Google" }));
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/loja"));
    expect(api.apiGoogleCadastro).toHaveBeenCalledWith({
      credential: "cred-google", cpf: "529.982.247-25", endereco: "Rua A", numero: "10", complemento: "", bairro: "",
      cep: "", cidade: "Sao Paulo", uf: "SP",
    });
    expect(lerCadastroGoogle()).toBeNull();
  });

  it("botao do Google no cadastro: sem conta abre o modo Google; 'usar outro email' volta", async () => {
    api.apiGoogle.mockResolvedValue({ cadastro_necessario: true, email: "ana@gmail.com", nome: "Ana" });
    render(<CadastroPage />);
    expect(screen.getByLabelText("Senha")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Google" }));
    expect(await screen.findByText(/Criando conta com Google/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Usar outro email" }));
    expect(screen.getByLabelText("Senha")).toBeInTheDocument();
    expect(lerCadastroGoogle()).toBeNull();
  });

  it("token do Google expirado no envio: volta ao inicio e mostra o erro", async () => {
    definirCadastroGoogle({ credential: "velho", email: "ana@gmail.com", nome: "Ana" });
    api.apiGoogleCadastro.mockRejectedValue(new Error("login com Google inválido ou expirado; tente novamente"));
    render(<CadastroPage />);
    await userEvent.type(screen.getByLabelText("CPF"), "52998224725");
    await userEvent.type(screen.getByLabelText("Rua"), "Rua A");
    await userEvent.type(screen.getByLabelText("Numero"), "10");
    await userEvent.type(screen.getByLabelText("Cidade"), "Sao Paulo");
    await userEvent.selectOptions(screen.getByLabelText("UF"), "SP");
    await userEvent.click(screen.getByRole("button", { name: "Criar conta com Google" }));
    expect(await screen.findByText(/inválido ou expirado/)).toBeInTheDocument();
    expect(screen.getByLabelText("Senha")).toBeInTheDocument();
  });
});
