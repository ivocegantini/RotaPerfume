import type { Metadata, Viewport } from "next";
import "./globals.css";
import { RodapeAmbiente } from "@/components/layout/RodapeAmbiente";

export const metadata: Metadata = {
  title: "RotaPerfumes - Sistema de Gestao",
  description: "Sistema de gestao de clientes, estoque e pedidos",
};

// UX-01: viewport explicito para celular (o Next ja injeta esse padrao, mas
// fica documentado aqui). Zoom do usuario continua permitido.
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="pt-BR">
      <body className="antialiased">
        {children}
        {/* ECOM-18: selo DEV/QA (nada em produção) */}
        <RodapeAmbiente />
      </body>
    </html>
  );
}
