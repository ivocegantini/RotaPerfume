"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { getUser, rotaInicial } from "@/lib/auth";
import { vlog } from "@/lib/vlog";

const FILE = "page.tsx";

export default function Home() {
  vlog(FILE, "Home", "obtendo router de navegacao");
  const router = useRouter();

  vlog(FILE, "Home", "registrando efeito de redirecionamento inicial");
  useEffect(() => {
    vlog(FILE, "Home.useEffect", "verificando se ha usuario salvo na sessao");
    const u = getUser();
    if (u) {
      // ECOM-01: o cliente da loja vai para /loja; a equipe segue no /dashboard.
      vlog(FILE, "Home.useEffect", "usuario autenticado; redirecionando conforme papel");
      router.replace(u.role === "usuario" ? rotaInicial(u.role) : "/dashboard");
    } else {
      vlog(FILE, "Home.useEffect", "sem usuario; redirecionando para /login");
      router.replace("/login");
    }
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="h-10 w-10 animate-spin rounded-full border-4 border-primary-200 border-t-primary-600" />
    </div>
  );
}
