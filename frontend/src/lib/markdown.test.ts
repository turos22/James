import { describe, expect, it } from "vitest";

import { separarSegmentos } from "./markdown";

describe("separarSegmentos", () => {
  it("devolve um único segmento de texto quando não há bloco de código", () => {
    expect(separarSegmentos("bom dia")).toEqual([
      { tipo: "texto", conteudo: "bom dia" },
    ]);
  });

  it("descarta os segmentos de texto vazios ao redor de um bloco", () => {
    expect(separarSegmentos("```py\nx = 1\n```")).toEqual([
      { tipo: "codigo", linguagem: "py", conteudo: "x = 1\n" },
    ]);
  });

  it("extrai a linguagem da primeira linha do bloco", () => {
    expect(separarSegmentos("veja:\n```go\nfmt.Println()\n```")).toEqual([
      { tipo: "texto", conteudo: "veja:\n" },
      { tipo: "codigo", linguagem: "go", conteudo: "fmt.Println()\n" },
    ]);
  });

  it("não trata a primeira linha como linguagem quando ela contém espaços", () => {
    expect(separarSegmentos("```isto é código\nmais\n```")).toEqual([
      { tipo: "codigo", linguagem: "", conteudo: "isto é código\nmais\n" },
    ]);
  });
});
