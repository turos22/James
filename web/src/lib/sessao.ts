// Ciclo de vida da sessão no cliente (D6).
//
// sessionStorage e não localStorage: sobrevive a F5 e a queda de rede — que
// continuam RETOMANDO a mesma conversa, preservando a correção da Fase 6 —
// mas morre quando a janela fecha, que é o gatilho de sessão nova que o
// Arthur pediu. Também mantém celular e desktop independentes: cada aba tem
// o seu.
const CHAVE = "james.sessao";

export function sessaoGuardada(): string | null {
  try {
    return sessionStorage.getItem(CHAVE) || null;
  } catch {
    return null;
  }
}

export function guardarSessao(id: string): void {
  try {
    if (!id) {
      sessionStorage.removeItem(CHAVE);
      return;
    }
    sessionStorage.setItem(CHAVE, id);
  } catch {
    // WebView com armazenamento desligado: seguir sem persistir é melhor
    // que derrubar o app.
  }
}

export function limparSessao(): void {
  try {
    sessionStorage.removeItem(CHAVE);
  } catch {
    // idem
  }
}
