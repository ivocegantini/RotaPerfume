"use client";

// ECOM-10: a pagina inicial do site e a vitrine de produtos (publica). A
// equipe interna entra pelo login e vai para o dashboard; o cliente da loja
// volta para ca.

import { Vitrine } from "@/components/loja/Vitrine";

export default function Home() {
  return <Vitrine />;
}
