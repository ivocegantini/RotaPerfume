// ECOM-03: helpers da confirmacao de e-mail do autocadastro.

/** Extrai o token de "#token=<hex>" (outros formatos sao ignorados). */
export function tokenDoHash(hash: string): string {
  const m = /^#token=([0-9a-f]{16,128})$/i.exec(hash.trim());
  return m ? m[1] : "";
}

/** O 403 do login de conta com e-mail ainda nao confirmado. */
export function ehEmailNaoConfirmado(msg: string): boolean {
  return /email n[aã]o confirmado/i.test(msg);
}
