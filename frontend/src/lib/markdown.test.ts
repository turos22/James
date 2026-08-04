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

  it("não trata linha de separador/cabeçalho como linguagem", () => {
    expect(separarSegmentos("```----\ncodigo aqui\n```")).toEqual([
      { tipo: "codigo", linguagem: "", conteudo: "----\ncodigo aqui\n" },
    ]);
  });

  it("trata blocos de código consecutivos como segmentos distintos", () => {
    expect(separarSegmentos("```py\ncode1\n``````go\ncode2\n```")).toEqual([
      { tipo: "codigo", linguagem: "py", conteudo: "code1\n" },
      { tipo: "codigo", linguagem: "go", conteudo: "code2\n" },
    ]);
  });

  it("degrada bem quando a crase tripla não fecha", () => {
    const resultado = separarSegmentos("```py\ncode sem fechar");
    expect(resultado).toEqual([
      { tipo: "codigo", linguagem: "py", conteudo: "code sem fechar" },
    ]);
  });
});
