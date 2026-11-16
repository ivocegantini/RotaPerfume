"use client";

// ECOM-02: botao "Entrar / Criar conta com Google" (Google Identity Services).
// Carrega o script oficial https://accounts.google.com/gsi/client via
// next/script (sem dependencia nova). O Client ID e publico e vem de
// NEXT_PUBLIC_GOOGLE_CLIENT_ID; sem ele o botao nao aparece. O ID token
// (credential) vai so para o backend, que valida assinatura, emissor,
// audiencia e expiracao; nunca e logado nem guardado em storage.

import Script from "next/script";
import { useEffect, useRef, useState } from "react";
import { vlog } from "@/lib/vlog";

const F = "GoogleButton.tsx";
const CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";
export const isGoogleEnabled = Boolean(CLIENT_ID);

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (o: {
            client_id: string;
            callback: (r: { credential?: string }) => void;
            ux_mode?: "popup" | "redirect";
            auto_select?: boolean;
            cancel_on_tap_outside?: boolean;
          }) => void;
          renderButton: (el: HTMLElement, o: Record<string, string | number>) => void;
        };
      };
    };
  }
}

interface GoogleButtonProps {
  /** Texto do botao: entrar ou criar conta. */
  texto?: "signin_with" | "signup_with" | "continue_with";
  /** Recebe o ID token do Google. */
  onCredential: (credential: string) => void;
}

export function GoogleButton({ texto = "continue_with", onCredential }: GoogleButtonProps) {
  const ref = useRef<HTMLDivElement>(null);
  const callback = useRef(onCredential);
  const [pronto, setPronto] = useState(false);

  useEffect(() => {
    callback.current = onCredential;
  }, [onCredential]);

  useEffect(() => {
    if (!pronto || !ref.current || !window.google) return;
    vlog(F, "GoogleButton.useEffect", "renderizando botao do Google");
    window.google.accounts.id.initialize({
      client_id: CLIENT_ID,
      ux_mode: "popup",
      auto_select: false,
      cancel_on_tap_outside: true,
      callback: (r) => {
        vlog(F, "GoogleButton.callback", "credencial recebida do Google (presente=%s)", !!r.credential);
        if (r.credential) callback.current(r.credential);
      },
    });
    window.google.accounts.id.renderButton(ref.current, {
      type: "standard",
      theme: "outline",
      size: "large",
      text: texto,
      shape: "rectangular",
      logo_alignment: "left",
      width: Math.min(ref.current.offsetWidth || 320, 400),
      locale: "pt-BR",
    });
  }, [pronto, texto]);

  if (!isGoogleEnabled) return null;
  return (
    <>
      <Script
        src="https://accounts.google.com/gsi/client"
        strategy="afterInteractive"
        onReady={() => setPronto(true)}
      />
      <div ref={ref} className="flex min-h-[44px] w-full justify-center" data-testid="google-button" />
    </>
  );
}
