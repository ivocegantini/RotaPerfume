"use client";

// ECOM-03: confirmacao do e-mail do autocadastro. O link do e-mail traz o
// token no fragmento (#token=...), que nao vai a servidores nem ao Referer.
// Com token: confirma (POST) e manda para o login. Sem token, ou com link
// invalido: formulario para pedir um novo link.

import { FormEvent, useEffect, useRef, useState, useSyncExternalStore } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Alert } from "@/components/ui/Alert";
import {
  TURNSTILE_ACTION_REENVIO,
  Turnstile,
  TurnstileHandle,
  isTurnstileEnabled,
} from "@/components/ui/Turnstile";
import { apiConfirmarEmail, apiReenviarConfirmacao } from "@/lib/api";
import { definirAvisoLogin } from "@/lib/avisoLogin";
import { tokenDoHash } from "@/lib/confirmacaoEmail";
import { vlog } from "@/lib/vlog";

const FILE = "confirmar-email/page.tsx";
const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

// Fragmento da URL como store externa: no servidor (e na hydration) e "",
// entao o HTML do servidor e o do cliente batem.
const assinarHash = (cb: () => void) => {
  window.addEventListener("hashchange", cb);
  return () => window.removeEventListener("hashchange", cb);
};
const lerHash = () => window.location.hash;
const hashNoServidor = () => "";

type Resultado = { ok: true } | { ok: false; msg: string };

export default function ConfirmarEmailPage() {
  const router = useRouter();
  const token = tokenDoHash(useSyncExternalStore(assinarHash, lerHash, hashNoServidor));
  const [resultado, setResultado] = useState<Resultado | null>(null);
  const enviado = useRef(false);

  const [email, setEmail] = useState("");
  const [erroEmail, setErroEmail] = useState<string | undefined>();
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const [reenviando, setReenviando] = useState(false);
  const [avisoReenvio, setAvisoReenvio] = useState<string | null>(null);
  const [erroReenvio, setErroReenvio] = useState<string | null>(null);
  const turnstileRef = useRef<TurnstileHandle>(null);

  vlog(FILE, "ConfirmarEmailPage", "registrando efeito de confirmacao (token presente=%s)", token !== "");
  useEffect(() => {
    if (!token || enviado.current) return;
    enviado.current = true; // uso unico: nao repete no Strict Mode
    apiConfirmarEmail(token).then(
      () => {
        vlog(FILE, "ConfirmarEmailPage.useEffect", "email confirmado");
        setResultado({ ok: true });
        definirAvisoLogin("Email confirmado! Agora voce ja pode entrar.");
        window.history.replaceState(null, "", window.location.pathname); // tira o token da barra
        router.replace("/login");
      },
      (e) => {
        vlog(FILE, "ConfirmarEmailPage.useEffect", "falha na confirmacao");
        setResultado({ ok: false, msg: e instanceof Error && e.message ? e.message : "Nao foi possivel confirmar o email" });
        window.history.replaceState(null, "", window.location.pathname);
      }
    );
  }, [token, router]);

  const reenviar = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setErroReenvio(null);
    setAvisoReenvio(null);
    if (!EMAIL_REGEX.test(email.trim())) {
      setErroEmail("Email invalido");
      return;
    }
    setErroEmail(undefined);
    setReenviando(true);
    try {
      const r = await apiReenviarConfirmacao(email.trim(), captchaToken ?? undefined);
      setAvisoReenvio(
        r.mensagem
          ? "Se houver um cadastro pendente para este email, enviamos um novo link. Confira sua caixa de entrada."
          : "Pedido enviado."
      );
    } catch (err) {
      const raw = err instanceof Error ? err.message : "";
      setErroReenvio(/captcha|turnstile/i.test(raw) ? "Nao foi possivel validar o captcha. Tente novamente." : raw || "Erro ao pedir o link");
    } finally {
      setCaptchaToken(null);
      turnstileRef.current?.reset();
      setReenviando(false);
    }
  };

  const confirmando = token !== "" && resultado === null;

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-linear-to-br from-slate-900 via-blue-900 to-slate-900 px-3 py-8 sm:px-4">
      <div className="relative z-10 w-full max-w-md">
        <div className="mb-6 text-center">
          <h1 className="text-3xl font-bold tracking-tight text-white">
            Rota<span className="text-primary-400">Perfumes</span>
          </h1>
          <p className="mt-1 text-sm text-slate-300">Confirmacao de email</p>
        </div>

        <Card padded={false} className="border-slate-200/20 bg-white/95 p-4 shadow-2xl backdrop-blur-sm sm:p-6">
          {confirmando ? (
            <p className="py-6 text-center text-sm text-slate-500">Confirmando seu email...</p>
          ) : resultado?.ok ? (
            <p className="py-6 text-center text-sm text-slate-600">Email confirmado! Redirecionando para o login...</p>
          ) : (
            <>
              {resultado && !resultado.ok && (
                <div className="mb-4">
                  <Alert variant="error">{resultado.msg}. Peca um novo link abaixo.</Alert>
                </div>
              )}
              <h2 className="mb-1 text-xl font-semibold text-slate-900">Reenviar link de confirmacao</h2>
              <p className="mb-5 text-sm text-slate-500">Informe o email usado no cadastro.</p>
              {avisoReenvio && (
                <div className="mb-4">
                  <Alert variant="success">{avisoReenvio}</Alert>
                </div>
              )}
              {erroReenvio && (
                <div className="mb-4">
                  <Alert variant="error" onClose={() => setErroReenvio(null)}>
                    {erroReenvio}
                  </Alert>
                </div>
              )}
              <form onSubmit={reenviar} noValidate className="space-y-4">
                <Input
                  label="Email"
                  type="email"
                  autoComplete="email"
                  maxLength={120}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  error={erroEmail}
                />
                <Turnstile
                  ref={turnstileRef}
                  action={TURNSTILE_ACTION_REENVIO}
                  onVerify={(t) => setCaptchaToken(t)}
                  onExpire={() => setCaptchaToken(null)}
                  onError={() => setCaptchaToken(null)}
                />
                <Button
                  type="submit"
                  variant="primary"
                  size="lg"
                  loading={reenviando}
                  disabled={isTurnstileEnabled && !captchaToken}
                  fullWidth
                >
                  {reenviando ? "Enviando..." : "Reenviar link"}
                </Button>
              </form>
            </>
          )}
          <p className="mt-6 text-center text-sm text-slate-500">
            <Link href="/login" className="font-medium text-primary-600 hover:underline">
              Voltar para o login
            </Link>
          </p>
        </Card>
      </div>
    </div>
  );
}
