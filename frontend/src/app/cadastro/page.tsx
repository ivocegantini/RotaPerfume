"use client";

// ECOM-01/03: autocadastro do cliente da loja (role "usuario"). O backend cria
// a conta com o e-mail pendente e envia o link de confirmacao; o login so
// funciona depois de confirmar (a tela mostra o aviso, sem sessao).

import { FormEvent, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import {
  TURNSTILE_ACTION_CADASTRO,
  Turnstile,
  TurnstileHandle,
  isTurnstileEnabled,
} from "@/components/ui/Turnstile";
import { apiCadastro, apiGoogle, apiGoogleCadastro } from "@/lib/api";
import { GoogleButton, isGoogleEnabled } from "@/components/ui/GoogleButton";
import { CadastroGooglePendente, definirCadastroGoogle, lerCadastroGoogle } from "@/lib/googleConta";
import { getUser, rotaInicial, saveUser } from "@/lib/auth";
import type { CadastroResponse } from "@/lib/types";
import { erroSenhaCadastro } from "@/lib/senhaCadastro";
import { UFS_BRASIL, cepOpcionalValido, cpfValido, mascararCEP, mascararCPF } from "@/lib/clienteLoja";
import { buscarCEP } from "@/lib/viaCep";
import { vlog } from "@/lib/vlog";

const FILE = "cadastro/page.tsx";

type Campo =
  | "nome" | "email" | "senha" | "confirmacao" | "cpf" | "endereco" | "numero" | "complemento" | "bairro" | "cep"
  | "cidade" | "uf";
type ErrosCampo = Partial<Record<Campo, string>>;

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
export default function CadastroPage() {
  const router = useRouter();
  const [nome, setNome] = useState("");
  const [email, setEmail] = useState("");
  const [senha, setSenha] = useState("");
  const [confirmacao, setConfirmacao] = useState("");
  // ECOM-05: cliente pessoa fisica criado junto com o login.
  const [cpf, setCpf] = useState("");
  const [endereco, setEndereco] = useState("");
  // ECOM-09: endereco separado; o CEP (opcional) preenche rua/bairro/cidade/UF.
  const [numero, setNumero] = useState("");
  const [complemento, setComplemento] = useState("");
  const [bairro, setBairro] = useState("");
  const [buscandoCep, setBuscandoCep] = useState(false);
  const [avisoCep, setAvisoCep] = useState<string | null>(null);
  const ultimoCep = useRef("");

  // Ao completar 8 digitos, busca na ViaCEP e preenche (tudo continua
  // editavel). So a resposta do ultimo CEP digitado e aplicada.
  const onCep = async (valor: string) => {
    const mascarado = mascararCEP(valor);
    setCep(mascarado);
    setAvisoCep(null);
    const digitos = mascarado.replace(/\D/g, "");
    ultimoCep.current = digitos;
    if (digitos.length !== 8) {
      setBuscandoCep(false);
      return;
    }
    setBuscandoCep(true);
    const achado = await buscarCEP(digitos);
    if (ultimoCep.current !== digitos) return;
    setBuscandoCep(false);
    if (!achado) {
      setAvisoCep("CEP nao encontrado: preencha o endereco abaixo.");
      return;
    }
    if (achado.logradouro) setEndereco(achado.logradouro);
    if (achado.bairro) setBairro(achado.bairro);
    setCidade(achado.cidade);
    setUf(achado.uf);
    setAvisoCep(
      achado.logradouro ? "Endereco preenchido pelo CEP. Confira e informe o numero." : "CEP da cidade: informe a rua e o bairro."
    );
  };
  const [cep, setCep] = useState("");
  const [cidade, setCidade] = useState("");
  const [uf, setUf] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<ErrosCampo>({});
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const turnstileRef = useRef<TurnstileHandle>(null);
  const [criado, setCriado] = useState<CadastroResponse | null>(null);
  // ECOM-02: cadastro com Google em andamento (nome/e-mail vem do Google).
  const [google, setGoogle] = useState<CadastroGooglePendente | null>(() => lerCadastroGoogle());

  const usarGoogle = async (credential: string) => {
    vlog(FILE, "CadastroPage.usarGoogle", "enviando credencial do Google ao backend");
    setError(null);
    setLoading(true);
    try {
      const res = await apiGoogle(credential);
      if ("cadastro_necessario" in res) {
        const p = { credential, email: res.email, nome: res.nome };
        definirCadastroGoogle(p);
        setGoogle(p);
        return;
      }
      vlog(FILE, "CadastroPage.usarGoogle", "conta ja existe: entrando");
      saveUser(res.user);
      router.replace(rotaInicial(res.user.role));
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : "Nao foi possivel usar o Google");
    } finally {
      setLoading(false);
    }
  };

  const sairDoGoogle = () => {
    definirCadastroGoogle(null);
    setGoogle(null);
  };

  vlog(FILE, "CadastroPage", "registrando efeito de redirecionamento se ja autenticado");
  useEffect(() => {
    const u = getUser();
    if (u) {
      vlog(FILE, "CadastroPage.useEffect", "usuario ja autenticado; redirecionando, papel:", u.role);
      router.replace(rotaInicial(u.role));
    }
  }, [router]);

  const validate = (): boolean => {
    vlog(FILE, "CadastroPage.validate", "validando campos do cadastro");
    const errs: ErrosCampo = {};
    if (!google) {
      if (!nome.trim()) errs.nome = "Nome e obrigatorio";
      else if ([...nome.trim()].length > 120) errs.nome = "Nome deve ter no maximo 120 caracteres";
      if (!email.trim()) errs.email = "Email e obrigatorio";
      else if (!EMAIL_REGEX.test(email.trim())) errs.email = "Email invalido";
      const erroSenha = erroSenhaCadastro(senha);
      if (erroSenha) errs.senha = erroSenha;
      if (confirmacao !== senha) errs.confirmacao = "As senhas nao conferem";
    }
    if (!cpfValido(cpf)) errs.cpf = "CPF invalido";
    if (!cepOpcionalValido(cep)) errs.cep = "CEP deve ter 8 digitos (ou deixe em branco)";
    if (!endereco.trim()) errs.endereco = "Rua e obrigatoria";
    if (!numero.trim()) errs.numero = "Numero e obrigatorio (use s/n se nao houver)";
    if (!cidade.trim()) errs.cidade = "Cidade e obrigatoria";
    if (!uf) errs.uf = "Selecione a UF";
    setFieldErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    if (!validate()) return;
    setLoading(true);
    if (google) {
      try {
        vlog(FILE, "CadastroPage.handleSubmit", "criando conta com Google");
        const res = await apiGoogleCadastro({
          credential: google.credential, cpf, endereco: endereco.trim(), numero: numero.trim(),
          complemento: complemento.trim(), bairro: bairro.trim(), cep, cidade: cidade.trim(), uf,
        });
        definirCadastroGoogle(null);
        saveUser(res.user);
        router.replace(rotaInicial(res.user.role));
      } catch (err) {
        const raw = err instanceof Error ? err.message : "";
        if (/google/i.test(raw)) sairDoGoogle(); // token expirado: clicar de novo no Google
        setError(raw || "Erro ao criar a conta com Google");
      } finally {
        setLoading(false);
      }
      return;
    }
    try {
      vlog(FILE, "CadastroPage.handleSubmit", "chamando apiCadastro (captcha presente=%s)", captchaToken !== null);
      const res = await apiCadastro({
        nome: nome.trim(),
        email: email.trim(),
        password: senha,
        captchaToken: captchaToken ?? undefined,
        cpf,
        endereco: endereco.trim(),
        numero: numero.trim(),
        complemento: complemento.trim(),
        bairro: bairro.trim(),
        cep,
        cidade: cidade.trim(),
        uf,
      });
      vlog(FILE, "CadastroPage.handleSubmit", "conta criada pendente; email enviado=%s", res.email_enviado);
      setCriado(res);
    } catch (err) {
      const raw = err instanceof Error ? err.message : "";
      vlog(FILE, "CadastroPage.handleSubmit", "falha no cadastro");
      setError(
        /captcha|turnstile/i.test(raw)
          ? "Nao foi possivel validar o captcha. Tente novamente."
          : raw || "Erro ao criar a conta"
      );
      setCaptchaToken(null);
      turnstileRef.current?.reset();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-linear-to-br from-slate-900 via-blue-900 to-slate-900 px-3 py-8 sm:px-4">
      <div className="pointer-events-none absolute -top-32 -left-32 h-96 w-96 rounded-full bg-blue-500/30 blur-3xl" />
      <div className="pointer-events-none absolute -bottom-32 -right-32 h-96 w-96 rounded-full bg-sky-500/20 blur-3xl" />

      <div className="relative z-10 w-full max-w-md">
        <div className="mb-6 text-center">
          <h1 className="text-3xl font-bold tracking-tight text-white">
            Rota<span className="text-primary-400">Perfumes</span>
          </h1>
          <p className="mt-1 text-sm text-slate-300">Loja online</p>
        </div>

        <Card padded={false} className="border-slate-200/20 bg-white/95 p-4 shadow-2xl backdrop-blur-sm sm:p-6">
          {criado ? (
            <div className="space-y-4 text-sm text-slate-600">
              <h2 className="text-xl font-semibold text-slate-900">Confirme seu email</h2>
              {criado.email_enviado ? (
                <p>
                  Enviamos um link de confirmacao para <strong className="text-slate-900">{criado.email}</strong>.
                  Abra o email e clique no link para ativar a conta (vale por 24 horas).
                </p>
              ) : (
                <Alert variant="warning">
                  A conta foi criada, mas nao conseguimos enviar o email agora. Peca um novo link abaixo.
                </Alert>
              )}
              <p>Nao recebeu? Confira a caixa de spam ou peca um novo link.</p>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Link href="/confirmar-email" className="font-medium text-primary-600 hover:underline">
                  Reenviar email de confirmacao
                </Link>
                <span className="hidden text-slate-300 sm:inline">|</span>
                <Link href="/login" className="font-medium text-primary-600 hover:underline">
                  Ir para o login
                </Link>
              </div>
            </div>
          ) : (
          <>
          <h2 className="mb-1 text-xl font-semibold text-slate-900">Criar conta</h2>
          <p className="mb-6 text-sm text-slate-500">Cadastre-se para ver os produtos da loja.</p>

          {error && (
            <div className="mb-4">
              <Alert variant="error" onClose={() => setError(null)}>
                {error}
              </Alert>
            </div>
          )}

          {google ? (
            <div className="mb-5 rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm text-slate-600">
              <p>
                Criando conta com Google: <strong className="text-slate-900">{google.nome || google.email}</strong>
              </p>
              <p className="text-slate-500">{google.email}</p>
              <p className="mt-2">Complete os dados abaixo para finalizar.</p>
              <button type="button" onClick={sairDoGoogle} className="mt-2 text-xs font-medium text-primary-600 hover:underline">
                Usar outro email
              </button>
            </div>
          ) : (
            isGoogleEnabled && (
              <div className="mb-5">
                <GoogleButton texto="signup_with" onCredential={(c) => void usarGoogle(c)} />
                <div className="mt-4 flex items-center gap-3 text-xs text-slate-400">
                  <span className="h-px flex-1 bg-slate-200" />
                  ou preencha
                  <span className="h-px flex-1 bg-slate-200" />
                </div>
              </div>
            )
          )}

          <form onSubmit={handleSubmit} noValidate className="space-y-4">
            {!google && (
            <>
            <Input
              label="Nome"
              autoComplete="name"
              maxLength={120}
              value={nome}
              onChange={(e) => setNome(e.target.value)}
              error={fieldErrors.nome}
            />
            <Input
              label="Email"
              type="email"
              placeholder="seu@email.com"
              autoComplete="email"
              maxLength={120}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              error={fieldErrors.email}
              helperText="Use um email de provedor conhecido (Gmail, Outlook, Hotmail, Yahoo, iCloud...)."
            />
            <Input
              label="Senha"
              type="password"
              autoComplete="new-password"
              value={senha}
              onChange={(e) => setSenha(e.target.value)}
              error={fieldErrors.senha}
              helperText="Minimo 8 caracteres, com 3 de: minuscula, maiuscula, numero e simbolo."
            />
            <Input
              label="Confirmar senha"
              type="password"
              autoComplete="new-password"
              value={confirmacao}
              onChange={(e) => setConfirmacao(e.target.value)}
              error={fieldErrors.confirmacao}
            />
            </>
            )}

            <Input
              label="CPF"
              inputMode="numeric"
              placeholder="000.000.000-00"
              value={cpf}
              onChange={(e) => setCpf(mascararCPF(e.target.value))}
              error={fieldErrors.cpf}
            />
            <Input
              label="CEP"
              inputMode="numeric"
              autoComplete="postal-code"
              placeholder="00000-000"
              value={cep}
              onChange={(e) => onCep(e.target.value)}
              error={fieldErrors.cep}
              helperText={
                buscandoCep
                  ? "Buscando endereco..."
                  : avisoCep ?? "Preenchemos o endereco pelo CEP. Sem CEP? Deixe em branco e digite."
              }
            />
            <Input
              label="Rua"
              autoComplete="address-line1"
              placeholder="Rua, avenida, estrada..."
              maxLength={255}
              value={endereco}
              onChange={(e) => setEndereco(e.target.value)}
              error={fieldErrors.endereco}
            />
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_2fr]">
              <Input
                label="Numero"
                placeholder="123 ou s/n"
                maxLength={20}
                value={numero}
                onChange={(e) => setNumero(e.target.value)}
                error={fieldErrors.numero}
              />
              <Input
                label="Complemento"
                autoComplete="address-line2"
                placeholder="Apto, bloco (opcional)"
                maxLength={120}
                value={complemento}
                onChange={(e) => setComplemento(e.target.value)}
                error={fieldErrors.complemento}
              />
            </div>
            <Input
              label="Bairro"
              maxLength={120}
              value={bairro}
              onChange={(e) => setBairro(e.target.value)}
              error={fieldErrors.bairro}
            />
            <Input
              label="Cidade"
              autoComplete="address-level2"
              maxLength={120}
              value={cidade}
              onChange={(e) => setCidade(e.target.value)}
              error={fieldErrors.cidade}
            />
            <label className="block text-sm font-medium text-slate-700">
              UF
              <select
                aria-label="UF"
                className="mt-1.5 block w-full rounded-md border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900"
                value={uf}
                onChange={(e) => setUf(e.target.value)}
              >
                <option value="">Selecione</option>
                {UFS_BRASIL.map((u) => (
                  <option key={u.sigla} value={u.sigla}>
                    {u.nome}
                  </option>
                ))}
              </select>
              {fieldErrors.uf && (
                <span className="mt-1 block text-xs font-normal text-red-600" role="alert">
                  {fieldErrors.uf}
                </span>
              )}
            </label>

            {!google && (
              <Turnstile
                ref={turnstileRef}
                action={TURNSTILE_ACTION_CADASTRO}
                onVerify={(token) => setCaptchaToken(token)}
                onExpire={() => setCaptchaToken(null)}
                onError={() => setCaptchaToken(null)}
              />
            )}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              loading={loading}
              disabled={!google && isTurnstileEnabled && !captchaToken}
              fullWidth
            >
              {loading ? "Criando conta..." : google ? "Criar conta com Google" : "Criar conta"}
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-slate-500">
            Ja tem conta?{" "}
            <Link href="/login" className="font-medium text-primary-600 hover:underline">
              Entrar
            </Link>
          </p>
          </>
          )}
        </Card>
      </div>
    </div>
  );
}
