import { describe, expect, it, vi } from "vitest";
import { criarAudioAnalisado } from "./audio";

function contextoFalso() {
  const analisador = { fftSize: 0, connect: vi.fn() };
  const fonte = { connect: vi.fn() };
  return {
    state: "running" as const,
    destination: { nome: "destino" },
    resume: vi.fn(),
    createAnalyser: vi.fn(() => analisador),
    createMediaElementSource: vi.fn(() => fonte),
    _analisador: analisador,
    _fonte: fonte,
  };
}

describe("criarAudioAnalisado", () => {
  it("liga elemento → fonte → analisador → destino", () => {
    const ctx = contextoFalso();

    const { analisador } = criarAudioAnalisado("/api/audio/x", ctx as unknown as AudioContext);

    expect(ctx.createMediaElementSource).toHaveBeenCalledTimes(1);
    expect(ctx._fonte.connect).toHaveBeenCalledWith(ctx._analisador);
    // Sem este connect o orbe anima em silêncio absoluto, e o sintoma
    // parece "o TTS quebrou" (R2).
    expect(ctx._analisador.connect).toHaveBeenCalledWith(ctx.destination);
    expect(analisador).toBe(ctx._analisador);
  });

  // createMediaElementSource só pode ser chamado uma vez por elemento (R1).
  // Criar o nó junto com o Audio é o que garante isso.
  it("cria um elemento e uma fonte novos a cada chamada", () => {
    const ctx = contextoFalso();

    const primeiro = criarAudioAnalisado("/api/audio/a", ctx as unknown as AudioContext);
    const segundo = criarAudioAnalisado("/api/audio/b", ctx as unknown as AudioContext);

    expect(ctx.createMediaElementSource).toHaveBeenCalledTimes(2);
    expect(primeiro.audio).not.toBe(segundo.audio);
  });

  it("retoma o contexto suspenso", () => {
    const ctx = { ...contextoFalso(), state: "suspended" as const };

    criarAudioAnalisado("/api/audio/x", ctx as unknown as AudioContext);

    expect(ctx.resume).toHaveBeenCalled();
  });
});
