export interface Sessao {
  ID: string;
  Titulo: string;
  CriadaEm: string;
  AtualizadaEm: string;
  Resumo: string;
  ResumidoAte: number | null;
}

export interface PaginaSessoes {
  limite?: number;
  // Cursor keyset: o AtualizadaEm do último item da página anterior.
  antes?: string;
  busca?: string;
}

export interface MensagemAPI {
  ID: number;
  SessaoID: string;
  Papel: string;
  Conteudo: string;
  CriadaEm: string;
}

export async function listarSessoes(pagina: PaginaSessoes = {}): Promise<Sessao[]> {
  const parametros = new URLSearchParams();
  if (pagina.limite) parametros.set("limite", String(pagina.limite));
  if (pagina.antes) parametros.set("antes", pagina.antes);
  if (pagina.busca) parametros.set("busca", pagina.busca);

  // web/src/lib/api.test.ts:18 (pré-existente) espera literalmente
  // "/api/sessoes", sem "?", na chamada sem argumentos. Só concatena
  // "?query" quando há parâmetro de verdade.
  const query = parametros.toString();
  const resp = await fetch(`/api/sessoes${query ? `?${query}` : ""}`);
  if (!resp.ok) throw new Error(`erro ao listar sessões: ${resp.status}`);
  return resp.json();
}

export async function criarSessao(titulo: string): Promise<Sessao> {
  const resp = await fetch("/api/sessoes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ titulo }),
  });
  if (!resp.ok) throw new Error(`erro ao criar sessão: ${resp.status}`);
  return resp.json();
}

export async function listarMensagens(sessaoId: string): Promise<MensagemAPI[]> {
  const resp = await fetch(`/api/sessoes/${sessaoId}/mensagens`);
  if (!resp.ok) throw new Error(`erro ao listar mensagens: ${resp.status}`);
  return resp.json();
}

export interface NotaAssunto {
  assunto: string;
  conteudo: string;
  modificado: string;
}

export interface RespostaMemoria {
  notas: NotaAssunto[];
  aviso?: string;
}

export async function lerMemoria(): Promise<RespostaMemoria> {
  const resp = await fetch("/api/memoria");
  if (!resp.ok) throw new Error(`erro ao ler memória: ${resp.status}`);
  return resp.json();
}
