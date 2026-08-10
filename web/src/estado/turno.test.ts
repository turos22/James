import { describe, expect, it } from "vitest";
import { estadoInicial, reduzir } from "./turno";

describe("estado do turno", () => {
  it("registra a mensagem enviada e marca o turno em andamento", () => {
    const estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "abre a calculadora" });

    expect(estado.mensagens).toHaveLength(1);
    expect(estado.mensagens[0]).toMatchObject({ papel: "usuario", texto: "abre a calculadora" });
    expect(estado.emAndamento).toBe(true);
  });

  it("abre e fecha ferramenta em voo", () => {
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "olha a tela" });
    estado = reduzir(estado, { tipo: "ferramenta_iniciada", dados: { nome: "ver_tela" } });

    expect(estado.ferramentas).toHaveLength(1);
    expect(estado.ferramentas[0]).toMatchObject({ nome: "ver_tela", estado: "voando" });

    estado = reduzir(estado, {
      tipo: "ferramenta_concluida",
      dados: { nome: "ver_tela", erro: "", saida: "superficie: DISPLAY1" },
    });

    expect(estado.ferramentas[0].estado).toBe("concluida");
  });

  it("marca ferramenta com erro", () => {
    let estado = reduzir(estadoInicial, { tipo: "ferramenta_iniciada", dados: { nome: "executar_comando_sistema" } });
    estado = reduzir(estado, {
      tipo: "ferramenta_concluida",
      dados: { nome: "executar_comando_sistema", erro: "ação negada pelo usuário", saida: "" },
    });

    expect(estado.ferramentas[0].estado).toBe("erro");
    expect(estado.ferramentas[0].erro).toBe("ação negada pelo usuário");
  });

  it("resposta final encerra o turno, acrescenta a mensagem e grava a sessão atual", () => {
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "oi" });
    estado = reduzir(estado, {
      tipo: "resposta_final",
      dados: { texto: "oi, Arthur", sessao_id: "11111111-1111-1111-1111-111111111111" },
    });

    expect(estado.emAndamento).toBe(false);
    expect(estado.mensagens[1]).toMatchObject({ papel: "james", texto: "oi, Arthur" });
    expect(estado.sessaoAtual).toBe("11111111-1111-1111-1111-111111111111");
  });

  it("erro encerra o turno e vira mensagem visível", () => {
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "oi" });
    estado = reduzir(estado, { tipo: "erro", dados: { mensagem: "deu ruim" } });

    expect(estado.emAndamento).toBe(false);
    expect(estado.mensagens[1].texto).toContain("deu ruim");
  });

  it("guarda a aprovação pendente e a limpa quando decidida", () => {
    let estado = reduzir(estadoInicial, {
      tipo: "aprovacao_requerida",
      dados: { id: "a1", ferramenta: "executar_comando_sistema", argumentos: { comando: "rm -rf" } },
    });

    expect(estado.aprovacao?.id).toBe("a1");

    estado = reduzir(estado, { tipo: "aprovacao_decidida" });

    expect(estado.aprovacao).toBeNull();
  });

  it("sessao_retomada preenche a tela com o histórico, mapeia modelo para james e limpa ferramenta velha", () => {
    // Achado da verificação ao vivo da Fase 6: o servidor reconstruía o
    // histórico certo pro Gemini, mas o evento sessao_retomada só mandava a
    // CONTAGEM de mensagens — a tela ficava em branco. Agora o payload leva
    // o conteúdo, e este teste prova que o redutor usa esse conteúdo pra
    // preencher `mensagens`, não só `sessaoAtual`.
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "olha a tela" });
    estado = reduzir(estado, { tipo: "ferramenta_iniciada", dados: { nome: "ver_tela" } });

    expect(estado.emAndamento).toBe(true);
    expect(estado.ferramentas).toHaveLength(1);

    estado = reduzir(estado, {
      tipo: "sessao_retomada",
      dados: {
        sessao_id: "22222222-2222-2222-2222-222222222222",
        mensagens: [
          { papel: "usuario", texto: "lembra do bolo?" },
          { papel: "modelo", texto: "lembro, de cenoura" },
        ],
      },
    });

    expect(estado.sessaoAtual).toBe("22222222-2222-2222-2222-222222222222");
    expect(estado.ferramentas).toHaveLength(0);
    expect(estado.emAndamento).toBe(false);
    expect(estado.mensagens).toEqual([
      { papel: "usuario", texto: "lembra do bolo?" },
      { papel: "james", texto: "lembro, de cenoura" },
    ]);
  });

  it("sessao_retomada ignora papel desconhecido no payload (defensivo contra rede)", () => {
    const estado = reduzir(estadoInicial, {
      tipo: "sessao_retomada",
      dados: {
        sessao_id: "33333333-3333-3333-3333-333333333333",
        // O backend já filtra pra usuario/modelo (internal/api/eventos.go,
        // mensagensExibiveis) — este teste cobre o redutor não confiando
        // cegamente no payload de rede mesmo assim.
        mensagens: [
          { papel: "usuario", texto: "oi" },
          { papel: "ferramenta", texto: "não devia chegar aqui" },
        ],
      },
    });

    expect(estado.mensagens).toEqual([{ papel: "usuario", texto: "oi" }]);
  });

  it("é puro: não muta o estado recebido", () => {
    const antes = estadoInicial;
    reduzir(antes, { tipo: "envio_local", texto: "oi" });

    expect(antes.mensagens).toHaveLength(0);
    expect(antes.sessaoAtual).toBeNull();
  });
});
