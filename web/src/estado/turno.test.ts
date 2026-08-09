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

    estado = reduzir(estado, { tipo: "ferramenta_concluida", dados: { nome: "ver_tela", erro: "" } });

    expect(estado.ferramentas[0].estado).toBe("concluida");
  });

  it("marca ferramenta com erro", () => {
    let estado = reduzir(estadoInicial, { tipo: "ferramenta_iniciada", dados: { nome: "executar_comando_sistema" } });
    estado = reduzir(estado, {
      tipo: "ferramenta_concluida",
      dados: { nome: "executar_comando_sistema", erro: "ação negada pelo usuário" },
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

  it("sessao_retomada grava a sessão atual e limpa ferramentas e andamento", () => {
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "olha a tela" });
    estado = reduzir(estado, { tipo: "ferramenta_iniciada", dados: { nome: "ver_tela" } });

    expect(estado.emAndamento).toBe(true);
    expect(estado.ferramentas).toHaveLength(1);

    estado = reduzir(estado, {
      tipo: "sessao_retomada",
      dados: { sessao_id: "22222222-2222-2222-2222-222222222222", mensagens: 4 },
    });

    expect(estado.sessaoAtual).toBe("22222222-2222-2222-2222-222222222222");
    expect(estado.ferramentas).toHaveLength(0);
    expect(estado.emAndamento).toBe(false);
  });

  it("é puro: não muta o estado recebido", () => {
    const antes = estadoInicial;
    reduzir(antes, { tipo: "envio_local", texto: "oi" });

    expect(antes.mensagens).toHaveLength(0);
    expect(antes.sessaoAtual).toBeNull();
  });
});
