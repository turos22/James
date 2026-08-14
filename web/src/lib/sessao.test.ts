import { beforeEach, describe, expect, it } from "vitest";
import { guardarSessao, limparSessao, sessaoGuardada } from "./sessao";

beforeEach(() => {
  sessionStorage.clear();
});

describe("Sessão guardada", () => {
  it("nasce vazia", () => {
    expect(sessaoGuardada()).toBeNull();
  });

  it("guarda e devolve o id", () => {
    guardarSessao("11111111-1111-1111-1111-111111111111");
    expect(sessaoGuardada()).toBe("11111111-1111-1111-1111-111111111111");
  });

  it("limpa", () => {
    guardarSessao("11111111-1111-1111-1111-111111111111");
    limparSessao();
    expect(sessaoGuardada()).toBeNull();
  });

  // O servidor manda sessao_id vazio no sessao_retomada de uma sessão nova
  // (ver §6.3 da spec). Guardar string vazia faria o cliente pedir
  // "retomar" de uma sessão que não existe no próximo carregamento.
  it("ignora id vazio", () => {
    guardarSessao("");
    expect(sessaoGuardada()).toBeNull();
  });

  // sessionStorage some em WebView com armazenamento desligado. Falhar aí
  // derrubaria o app inteiro por causa de uma conveniência.
  it("não explode quando o sessionStorage não está disponível", () => {
    const original = Object.getOwnPropertyDescriptor(window, "sessionStorage");
    Object.defineProperty(window, "sessionStorage", {
      configurable: true,
      get() {
        throw new Error("bloqueado");
      },
    });

    expect(() => guardarSessao("x")).not.toThrow();
    expect(sessaoGuardada()).toBeNull();

    if (original) Object.defineProperty(window, "sessionStorage", original);
  });
});
