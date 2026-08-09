import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Tela, interpretarSaidaVerTela } from "./tela";

// Formato real de elementos.py:formatar (linha 137) — dois espaços entre
// campos, não um. Conferido também contra o literal usado em
// dispositivo/tests/test_tela.py:166 ("e1  Button  \"Salvar\"  (820,455)")
// e contra a spec da Fase 5, seção 5.1.
const SAIDA = `superficie: DISPLAY1 1920x1080 escala 100%
e3  Button  "Um"  (1163,761)
e4  Button  "Dois"  (1229,761)
e7  Edit  "visor"  (1100,300)`;

describe("interpretarSaidaVerTela", () => {
  it("separa a superfície dos elementos", () => {
    const lido = interpretarSaidaVerTela(SAIDA);

    expect(lido.superficie).toBe("DISPLAY1 1920x1080 escala 100%");
    expect(lido.elementos).toHaveLength(3);
    expect(lido.elementos[0]).toBe('e3  Button  "Um"  (1163,761)');
  });

  it("aguenta saída sem linha de superfície", () => {
    const lido = interpretarSaidaVerTela('e1  Button  "OK"  (10,10)');

    expect(lido.superficie).toBe("");
    expect(lido.elementos).toHaveLength(1);
  });

  it("mantém o marcador (primaria) no texto da superfície", () => {
    // tela.py só acrescenta "(primaria)" para o monitor primário — o prefixo
    // "superficie:" continua sendo o único contrato que o parser exige.
    const lido = interpretarSaidaVerTela(
      'superficie: DISPLAY1 (primaria) 1920x1080 escala 100%\ne1  Button  "OK"  (10,10)'
    );

    expect(lido.superficie).toBe("DISPLAY1 (primaria) 1920x1080 escala 100%");
  });

  it("aguenta saída vazia ou só com espaços sem quebrar", () => {
    expect(interpretarSaidaVerTela("")).toEqual({ superficie: "", elementos: [] });
    expect(interpretarSaidaVerTela("   \n  \n")).toEqual({ superficie: "", elementos: [] });
  });

  it("mantém a linha de aviso de truncagem como um item de elemento", () => {
    // elementos.py:formatar anexa esta linha quando estoura o teto de 80 —
    // ela não começa com um id, mas o parser não distingue: entra como mais
    // uma linha de elemento, e é assim que ela chega até a lista na tela.
    const comAviso = `superficie: DISPLAY1 1920x1080 escala 100%
e1  Button  "Um"  (10,10)
... e mais 15 elementos (peça por nome se não estiver na lista)`;

    const lido = interpretarSaidaVerTela(comAviso);

    expect(lido.elementos).toHaveLength(2);
    expect(lido.elementos[1]).toBe("... e mais 15 elementos (peça por nome se não estiver na lista)");
  });
});

describe("Aba Tela", () => {
  it("mostra superfície e elementos", () => {
    render(<Tela saida={SAIDA} />);

    expect(screen.getByText(/DISPLAY1 1920x1080 escala 100%/)).toBeTruthy();
    // getByText normaliza espaços em sequência antes de comparar, então o
    // espaçamento duplo real da linha vira espaço simples aqui — a
    // renderização em si preserva os dois espaços (ver dump do DOM na
    // asserção anterior, quando este teste falhou antes do ajuste).
    expect(screen.getByText(/e3 Button "Um"/)).toBeTruthy();
  });

  it("explica quando nada foi capturado ainda", () => {
    render(<Tela saida={null} />);

    expect(screen.getByText(/Nenhuma captura/)).toBeTruthy();
  });
});
