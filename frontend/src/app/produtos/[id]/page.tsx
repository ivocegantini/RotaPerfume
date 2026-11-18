"use client";

// ECOM-11: pagina publica do produto (/produtos/{id}), aberta pela vitrine.

import { useParams } from "next/navigation";
import { PaginaProduto } from "@/components/loja/PaginaProduto";

export default function ProdutoPage() {
  const { id } = useParams<{ id: string }>();
  return <PaginaProduto idBruto={id} />;
}
