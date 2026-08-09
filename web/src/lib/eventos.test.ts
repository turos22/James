import { describe, expect, it } from "vitest";
import { EVENTOS_CONHECIDOS, ehEventoConhecido, type DadosRespostaFinal } from "./eventos";

describe("eventos", () => {
  it("conhece exatamente os eventos que o backend emite hoje", () => {
    // Se o backend ganhar ou perder um evento, este teste é o lugar onde a
    // divergência aparece antes de virar tela em branco. `token` foi
    // removido do ws.go na Fase 6 e não pode voltar para cá sem decisão.
    expect([...EVENTOS_CONHECIDOS].sort()).toEqual([
      "aprovacao_requerida",
      "audio_pronto",
      "erro",
      "ferramenta_concluida",
      "ferramenta_iniciada",
      "resposta_final",
      "sessao_retomada",
    ]);
  });

  it("reconhece evento conhecido e rejeita desconhecido", () => {
    expect(ehEventoConhecido("resposta_final")).toBe(true);
    expect(ehEventoConhecido("token")).toBe(false);
    expect(ehEventoConhecido("qualquer_coisa")).toBe(false);
  });

  it("DadosRespostaFinal exige texto e sessao_id juntos", () => {
    // Este teste só pega a regressão de verdade em `tsc` (npm run build),
    // não em `vitest run`: o transform do vite/esbuild apaga tipos sem
    // checá-los, então rodar só `npm test` deixaria passar mesmo se
    // sessao_id sumisse da interface. O que trava a compilação aqui é a
    // checagem de excess property do TypeScript sobre objeto literal
    // atribuído direto a uma variável tipada: se sessao_id for removido de
    // DadosRespostaFinal, esta atribuição vira erro TS2353 ("does not
    // exist in type"); se texto for removido, vira TS2741 ("missing").
    // Simula o envelope que ws.go realmente manda (ver Enviar("resposta_final"
    // em ws.go:121 e dispositivo.go:134, ambos com texto e sessao_id).
    const envelopeDoBackend: DadosRespostaFinal = {
      texto: "oi",
      sessao_id: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    };

    expect(envelopeDoBackend.texto).toBe("oi");
    expect(envelopeDoBackend.sessao_id).toBe("3fa85f64-5717-4562-b3fc-2c963f66afa6");
  });
});
