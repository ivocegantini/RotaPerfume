"use client";

// ECOM-16: detalhe de uma compra do usuario logado (itens e pagamento).

import { useParams } from "next/navigation";
import { DetalheCompra } from "@/components/loja/DetalheCompra";

export default function MinhaCompraPage() {
  const { id } = useParams<{ id: string }>();
  return <DetalheCompra idBruto={id} />;
}
