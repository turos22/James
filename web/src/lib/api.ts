export interface Sessao {
  ID: string;
  Titulo: string;
  CriadaEm: string;
  AtualizadaEm: string;
}

export interface MensagemAPI {
  ID: number;
  SessaoID: string;
  Papel: string;
  Conteudo: string;
  CriadaEm: string;
}

export async function listarSessoes(): Promise<Sessao[]> {
  const resp = await fetch("/api/sessoes");
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
