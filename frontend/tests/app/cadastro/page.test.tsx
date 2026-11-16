/** ECOM-01: tela de autocadastro da loja. */
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CadastroRequest, CadastroResponse } from "@/lib/types";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: replaceMock }) }));
const apiCadastroMock = vi.fn<(b: CadastroRequest) => Promise<CadastroResponse>>();
vi.mock("@/lib/api", () => ({
  apiCadastro: (b: CadastroRequest) => apiCadastroMock(b),
}));
vi.mock("@/components/ui/Turnstile", async () => {
  const React = await import("react");
  const Turnstile = React.forwardRef<{ reset: () => void }, { action: string }>(function Fake({ action }, ref) {
    React.useImperativeHandle(ref, () => ({ reset: () => {} }));
    return <div data-testid="turnstile" data-action={action} />;
  });
  return { Turnstile, TURNSTILE_ACTION_CADASTRO: "cadastro", isTurnstileEnabled: false };
});

import CadastroPage from "@/app/cadastro/page";

// ECOM-05: dados validos do cliente (CPF de teste).
const CLIENTE = { cpf: "52998224725", endereco: "Rua A", numero: "10", cep: "", cidade: "Sao Paulo", uf: "SP" };

async function preencher(nome: string, email: string, senha: string, confirmacao = senha, cliente: Partial<typeof CLIENTE> = CLIENTE) {
  if (nome) await userEvent.type(screen.getByLabelText("Nome"), nome);
  if (email) await userEvent.type(screen.getByLabelText("Email"), email);
  if (senha) await userEvent.type(screen.getByLabelText("Senha"), senha);
  if (confirmacao) await userEvent.type(screen.getByLabelText("Confirmar senha"), confirmacao);
  if (cliente.cpf) await userEvent.type(screen.getByLabelText("CPF"), cliente.cpf);
  if (cliente.endereco) await userEvent.type(screen.getByLabelText("Rua"), cliente.endereco);
  if (cliente.numero) await userEvent.type(screen.getByLabelText("Numero"), cliente.numero);
  if (cliente.cep) await userEvent.type(screen.getByLabelText("CEP"), cliente.cep);
  if (cliente.cidade) await userEvent.type(screen.getByLabelText("Cidade"), cliente.cidade);
  if (cliente.uf) await userEvent.selectOptions(screen.getByLabelText("UF"), cliente.uf);
  await userEvent.click(screen.getByRole("button", { name: "Criar conta" }));
}

beforeEach(() => {
  replaceMock.mockReset();
  apiCadastroMock.mockReset();
  localStorage.clear();
});

describe("CadastroPage", () => {
  it("UF mostra o nome do estado e envia a sigla (ECOM-07)", () => {
    render(<CadastroPage />);
    const opcao = screen.getByRole("option", { name: "São Paulo" }) as HTMLOptionElement;
    expect(opcao.value).toBe("SP");
    expect(screen.queryByRole("option", { name: "SP" })).not.toBeInTheDocument();
  });

  it("usa a action de captcha do cadastro", () => {
    render(<CadastroPage />);
    expect(screen.getByTestId("turnstile")).toHaveAttribute("data-action", "cadastro");
  });

  it("valida localmente e nao chama a API", async () => {
    render(<CadastroPage />);
    await preencher("", "sem-arroba", "fraca", "outra", { cpf: "11111111111", cep: "123" });
    expect(screen.getByText("Nome e obrigatorio")).toBeInTheDocument();
    expect(screen.getByText("Email invalido")).toBeInTheDocument();
    expect(screen.getByText("Senha deve ter pelo menos 8 caracteres")).toBeInTheDocument();
    expect(screen.getByText("As senhas nao conferem")).toBeInTheDocument();
    expect(screen.getByText("CPF invalido")).toBeInTheDocument();
    expect(screen.getByText("Rua e obrigatoria")).toBeInTheDocument();
    expect(screen.getByText("Numero e obrigatorio (use s/n se nao houver)")).toBeInTheDocument();
    expect(screen.getByText("CEP deve ter 8 digitos (ou deixe em branco)")).toBeInTheDocument();
    expect(screen.getByText("Cidade e obrigatoria")).toBeInTheDocument();
    expect(screen.getByText("Selecione a UF")).toBeInTheDocument();
    expect(apiCadastroMock).not.toHaveBeenCalled();
  });

  it("cadastro ok mostra o aviso de confirmacao, sem sessao nem redirect (ECOM-03)", async () => {
    apiCadastroMock.mockResolvedValue({ email: "ana@x.com", email_enviado: true, mensagem: "ok" });
    render(<CadastroPage />);
    await preencher("Ana", "ana@x.com", "Loja#2026ok");
    expect(await screen.findByText("Confirme seu email")).toBeInTheDocument();
    expect(screen.getByText("ana@x.com")).toBeInTheDocument();
    expect(apiCadastroMock).toHaveBeenCalledWith({
      nome: "Ana",
      email: "ana@x.com",
      password: "Loja#2026ok",
      captchaToken: undefined,
      cpf: "529.982.247-25",
      endereco: "Rua A",
      numero: "10",
      complemento: "",
      bairro: "",
      cep: "",
      cidade: "Sao Paulo",
      uf: "SP",
    });
    expect(replaceMock).not.toHaveBeenCalled();
    expect(localStorage.getItem("auth_user")).toBeNull();
    expect(screen.getByRole("link", { name: "Reenviar email de confirmacao" })).toHaveAttribute("href", "/confirmar-email");
  });

  it("email nao enviado: avisa para pedir novo link", async () => {
    apiCadastroMock.mockResolvedValue({ email: "ana@x.com", email_enviado: false, mensagem: "x" });
    render(<CadastroPage />);
    await preencher("Ana", "ana@x.com", "Loja#2026ok");
    expect(await screen.findByText(/nao conseguimos enviar o email agora/)).toBeInTheDocument();
  });

  it("erro da API aparece na tela", async () => {
    apiCadastroMock.mockRejectedValue(new Error("não foi possível concluir o cadastro com este email"));
    render(<CadastroPage />);
    await preencher("Ana", "ana@x.com", "Loja#2026ok");
    expect(await screen.findByText("não foi possível concluir o cadastro com este email")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });
});
