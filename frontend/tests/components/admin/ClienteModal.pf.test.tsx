/**
 * ECOM-06: o ClienteModal edita cliente pessoa fisica (CPF + endereco). O
 * payload da PF vai com cnpj vazio e cpf/cep sem mascara.
 */
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Cliente } from "@/lib/types";
import { ClienteModal } from "@/components/admin/ClienteModal";

const clientePF: Cliente = {
  cliente_id_origem: 7, cnpj: "", cpf: "52998224725", razao_social: "Ana Souza", segmento: "Loja Online",
  cidade: "Curitiba", uf: "PR", bairro: "Centro", endereco: "Rua XV", numero: "10", complemento: "",
  cep: "80020310", data_cadastro: "2026-11-01T00:00:00Z", ativo: true, created_at: "", updated_at: "",
};

function montar(cliente: Cliente | null, mode: "create" | "edit") {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  render(<ClienteModal open mode={mode} cliente={cliente} onClose={vi.fn()} onSubmit={onSubmit} />);
  return { onSubmit };
}

describe("ClienteModal - pessoa fisica (ECOM-06)", () => {
  it("edicao de PF abre com CPF e endereco e salva sem CNPJ", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar(clientePF, "edit");
    expect(screen.getByLabelText("Pessoa fisica (CPF)")).toBeChecked();
    expect(screen.getByLabelText("CPF")).toHaveValue("529.982.247-25");
    expect(screen.getByLabelText("CEP")).toHaveValue("80020-310");
    expect(screen.queryByLabelText("CNPJ")).not.toBeInTheDocument();
    await u.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({
      cnpj: "", cpf: "52998224725", razao_social: "Ana Souza", endereco: "Rua XV", numero: "10", cep: "80020310",
    }));
  });

  it("PF recusa CPF invalido e exige rua e numero", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar({ ...clientePF, cpf: "52998224724" }, "edit");
    await u.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(await screen.findByText("CPF invalido.")).toBeInTheDocument();
    await u.clear(screen.getByLabelText("CPF"));
    await u.type(screen.getByLabelText("CPF"), "52998224725");
    // Rua e numero sao required na PF (o form nativo bloqueia o envio).
    expect(screen.getByLabelText(/Rua/)).toBeRequired();
    expect(screen.getByLabelText(/Numero/)).toBeRequired();
    await u.clear(screen.getByLabelText(/Rua/));
    await u.click(screen.getByRole("button", { name: "Salvar alteracoes" }));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("PJ continua com CNPJ e manda cpf vazio", async () => {
    const u = userEvent.setup();
    const { onSubmit } = montar(null, "create");
    expect(screen.getByLabelText("Pessoa juridica (CNPJ)")).toBeChecked();
    await u.type(screen.getByLabelText("Razao social"), "Loja");
    await u.type(screen.getByLabelText("CNPJ"), "11222333000181");
    await u.type(screen.getByLabelText("Segmento"), "Varejo");
    await u.type(screen.getByLabelText("Cidade"), "Santos");
    await u.type(screen.getByLabelText("UF"), "SP");
    await u.click(screen.getByRole("button", { name: "Criar cliente" }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ cnpj: "11222333000181", cpf: "" })));
  });
});
