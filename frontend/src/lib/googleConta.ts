// ECOM-02: cadastro com Google em andamento (so em memoria, nunca em
// storage): o ID token e o perfil vindos do "Entrar com Google" quando o
// e-mail ainda nao tem conta. A tela /cadastro completa CPF e endereco.

export interface CadastroGooglePendente {
  credential: string;
  email: string;
  nome: string;
}

let pendente: CadastroGooglePendente | null = null;

export function definirCadastroGoogle(p: CadastroGooglePendente | null): void {
  pendente = p;
}

export function lerCadastroGoogle(): CadastroGooglePendente | null {
  return pendente;
}
