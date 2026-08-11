import type {
  DadosAprovacaoRequerida,
  DadosErro,
  DadosFerramentaConcluida,
  DadosFerramentaIniciada,
  DadosRespostaFinal,
  DadosSessaoRetomada,
  DadosTranscricao,
} from "../lib/eventos";

export interface Mensagem {
  papel: "usuario" | "james";
  texto: string;
}

export interface FerramentaNoTurno {
  nome: string;
  estado: "voando" | "concluida" | "erro";
  erro?: string;
  iniciadaEm: number;
}

export interface EstadoTurno {
  mensagens: Mensagem[];
  ferramentas: FerramentaNoTurno[];
  aprovacao: DadosAprovacaoRequerida | null;
  emAndamento: boolean;
  // Vem do próprio resposta_final desde a Task 5, e de sessao_retomada.
  // Fica aqui, e não num useState do componente, para não haver duas
  // fontes de verdade sobre em que conversa o turno está acontecendo.
  sessaoAtual: string | null;
}

export type Acao =
  | { tipo: "envio_local"; texto: string }
  | { tipo: "resposta_final"; dados: DadosRespostaFinal }
  | { tipo: "erro"; dados: DadosErro }
  | { tipo: "ferramenta_iniciada"; dados: DadosFerramentaIniciada }
  | { tipo: "ferramenta_concluida"; dados: DadosFerramentaConcluida }
  | { tipo: "aprovacao_requerida"; dados: DadosAprovacaoRequerida }
  | { tipo: "aprovacao_decidida" }
  | { tipo: "sessao_retomada"; dados: DadosSessaoRetomada }
  | { tipo: "transcricao"; dados: DadosTranscricao };

export const estadoInicial: EstadoTurno = {
  mensagens: [],
  ferramentas: [],
  aprovacao: null,
  emAndamento: false,
  sessaoAtual: null,
};

// reduzir é função pura de propósito: todo o comportamento de turno fica
// testável sem React, sem socket e sem DOM. O componente só liga eventos a
// esta função.
export function reduzir(estado: EstadoTurno, acao: Acao): EstadoTurno {
  switch (acao.tipo) {
    // envio_local e transcricao são a MESMA abertura de turno por dois
    // caminhos: no texto quem sabe o que foi dito é o próprio cliente que
    // digitou; na voz é o servidor, que transcreve e anuncia. Daí o mesmo
    // corpo — balão do usuário, ferramentas do turno anterior zeradas, turno
    // marcado em andamento.
    case "envio_local":
    case "transcricao": {
      const texto = acao.tipo === "envio_local" ? acao.texto : acao.dados.texto;
      return {
        ...estado,
        mensagens: [...estado.mensagens, { papel: "usuario", texto }],
        ferramentas: [],
        emAndamento: true,
      };
    }

    // Turno encerrado (por resposta ou por erro) não pode deixar aprovação
    // pendente para trás: o servidor já resolveu tudo que estava esperando,
    // e um card sobrevivente trava o visualizador (precedência máxima em
    // escolherConteudo) com botões que não resolvem mais nada.
    case "resposta_final":
      return {
        ...estado,
        mensagens: [...estado.mensagens, { papel: "james", texto: acao.dados.texto }],
        emAndamento: false,
        aprovacao: null,
        sessaoAtual: acao.dados.sessao_id,
      };

    case "erro":
      return {
        ...estado,
        mensagens: [...estado.mensagens, { papel: "james", texto: `Erro: ${acao.dados.mensagem}` }],
        emAndamento: false,
        aprovacao: null,
      };

    case "ferramenta_iniciada":
      return {
        ...estado,
        ferramentas: [
          ...estado.ferramentas,
          { nome: acao.dados.nome, estado: "voando", iniciadaEm: Date.now() },
        ],
      };

    case "ferramenta_concluida": {
      let fechou = false;
      const ferramentas = estado.ferramentas.map((f) => {
        if (fechou || f.nome !== acao.dados.nome || f.estado !== "voando") return f;
        fechou = true;
        return {
          ...f,
          estado: acao.dados.erro ? ("erro" as const) : ("concluida" as const),
          erro: acao.dados.erro || undefined,
        };
      });
      // A ferramenta que estava bloqueada esperando o humano acabou de
      // voltar — por aprovação, por negação ou por expiração do prazo. Em
      // qualquer um dos três casos o pedido já não está mais pendente no
      // servidor, e manter o card na tela só engana: os botões não resolvem
      // nada e o visualizador fica travado nele. Comparação por nome porque
      // ferramenta_concluida não carrega o id da aprovação; uma aprovação de
      // OUTRA ferramenta segue pendente, como deve.
      const aprovacao =
        estado.aprovacao?.ferramenta === acao.dados.nome ? null : estado.aprovacao;
      return { ...estado, ferramentas, aprovacao };
    }

    case "aprovacao_requerida":
      return { ...estado, aprovacao: acao.dados };

    case "aprovacao_decidida":
      return { ...estado, aprovacao: null };

    case "sessao_retomada": {
      // Mapeia o papel do backend (agente.Papel: "usuario"/"modelo") pro
      // papel que a tela entende ("usuario"/"james"). Papel desconhecido é
      // ignorado: o backend já filtra pra usuario/modelo com texto não
      // vazio (internal/api/eventos.go, mensagensExibiveis), mas o redutor
      // não deve confiar cegamente em payload de rede — um typo ou uma
      // versão de servidor divergente não pode virar balão quebrado na tela.
      const mensagens: Mensagem[] = [];
      for (const m of acao.dados.mensagens) {
        if (m.papel === "usuario") mensagens.push({ papel: "usuario", texto: m.texto });
        else if (m.papel === "modelo") mensagens.push({ papel: "james", texto: m.texto });
      }
      return {
        ...estado,
        sessaoAtual: acao.dados.sessao_id,
        mensagens,
        ferramentas: [],
        emAndamento: false,
      };
    }
  }
}
