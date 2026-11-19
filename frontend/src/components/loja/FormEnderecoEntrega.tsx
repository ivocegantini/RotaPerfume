"use client";

// ECOM-17: endereco de entrega. O CEP vem primeiro: ao completar 8 digitos a
// ViaCEP preenche rua, bairro, cidade e estado. Os demais campos so liberam
// depois da pesquisa (achou ou nao) e continuam abertos (editaveis).

import { useEffect, useRef, useState } from "react";
import { Input } from "@/components/ui/Input";
import { mascararCEP } from "@/lib/clienteLoja";
import type { ErrosEntrega } from "@/lib/entrega";
import type { EnderecoEntrega } from "@/lib/types";
import { buscarCEP } from "@/lib/viaCep";
import { vlog } from "@/lib/vlog";

const FILE = "FormEnderecoEntrega.tsx";

interface Props {
  valor: EnderecoEntrega;
  onChange: (v: EnderecoEntrega) => void;
  erros?: ErrosEntrega;
  /** true quando ja ha endereco (edicao): campos liberados sem nova pesquisa. */
  pesquisadoInicial?: boolean;
  buscar?: typeof buscarCEP;
}

export function FormEnderecoEntrega({ valor, onChange, erros = {}, pesquisadoInicial = false, buscar = buscarCEP }: Props) {
  const [buscando, setBuscando] = useState(false);
  const [aviso, setAviso] = useState<string | null>(null);
  const [pesquisado, setPesquisado] = useState(pesquisadoInicial);
  const ultimoCep = useRef("");
  // valor mais recente: a resposta da ViaCEP nao sobrescreve o que foi digitado depois.
  const atual = useRef(valor);
  useEffect(() => {
    atual.current = valor;
  }, [valor]);

  const set = (campo: keyof EnderecoEntrega, v: string) => onChange({ ...atual.current, [campo]: v });

  const onCep = async (bruto: string) => {
    const cep = mascararCEP(bruto);
    onChange({ ...atual.current, cep });
    setAviso(null);
    const digitos = cep.replace(/\D/g, "");
    ultimoCep.current = digitos;
    if (digitos.length !== 8) {
      setBuscando(false);
      return;
    }
    setBuscando(true);
    vlog(FILE, "onCep", "pesquisando CEP");
    const achado = await buscar(digitos);
    if (ultimoCep.current !== digitos) return;
    setBuscando(false);
    setPesquisado(true);
    if (!achado) {
      setAviso("CEP nao encontrado: preencha o endereco abaixo.");
      return;
    }
    onChange({
      ...atual.current,
      endereco: achado.logradouro || atual.current.endereco,
      bairro: achado.bairro || atual.current.bairro,
      cidade: achado.cidade,
      uf: achado.uf,
    });
    setAviso(achado.logradouro ? "Endereco preenchido pelo CEP. Confira e informe o numero." : "CEP da cidade: informe a rua e o bairro.");
  };

  const bloqueado = !pesquisado || buscando;
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-1 text-sm font-semibold text-slate-900">Endereco de entrega</legend>
      <Input
        label="CEP"
        inputMode="numeric"
        autoComplete="postal-code"
        placeholder="00000-000"
        value={valor.cep}
        onChange={(e) => void onCep(e.target.value)}
        error={erros.cep}
        helperText={buscando ? "Buscando endereco..." : aviso ?? (pesquisado ? undefined : "Digite o CEP para carregar o endereco.")}
      />
      <Input label="Rua" autoComplete="address-line1" maxLength={255} disabled={bloqueado}
        value={valor.endereco} onChange={(e) => set("endereco", e.target.value)} error={erros.endereco} />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_2fr]">
        <Input label="Numero" placeholder="123 ou s/n" maxLength={20} disabled={bloqueado}
          value={valor.numero} onChange={(e) => set("numero", e.target.value)} error={erros.numero} />
        <Input label="Complemento" autoComplete="address-line2" placeholder="Apto, bloco (opcional)" maxLength={120}
          disabled={bloqueado} value={valor.complemento} onChange={(e) => set("complemento", e.target.value)} error={erros.complemento} />
      </div>
      <Input label="Bairro" maxLength={120} disabled={bloqueado}
        value={valor.bairro} onChange={(e) => set("bairro", e.target.value)} error={erros.bairro} />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-[2fr_1fr]">
        <Input label="Cidade" autoComplete="address-level2" maxLength={120} disabled={bloqueado}
          value={valor.cidade} onChange={(e) => set("cidade", e.target.value)} error={erros.cidade} />
        <Input label="Estado" autoComplete="address-level1" placeholder="UF" maxLength={2} disabled={bloqueado}
          value={valor.uf} onChange={(e) => set("uf", e.target.value.toUpperCase())} error={erros.uf} />
      </div>
    </fieldset>
  );
}
