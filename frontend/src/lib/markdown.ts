export type Segmento =
  | { tipo: "texto"; conteudo: string }
  | { tipo: "codigo"; linguagem: string; conteudo: string };

/**
 * Divide uma resposta em segmentos de texto e blocos de código delimitados
 * por crase tripla. A primeira linha do bloco vira a linguagem quando é uma
 * única palavra curta.
 */
export function separarSegmentos(conteudo: string): Segmento[] {
  const partes = conteudo.split("```");
  const segmentos: Segmento[] = [];

  partes.forEach((parte, indice) => {
    const ehCodigo = indice % 2 === 1;

    if (!ehCodigo) {
      if (parte.trim() !== "") {
        segmentos.push({ tipo: "texto", conteudo: parte });
      }
      return;
    }

    const linhas = parte.split("\n");
    const primeira = linhas[0].trim();
    const ehLinguagem =
      linhas.length > 1 &&
      primeira !== "" &&
      !primeira.includes(" ") &&
      primeira.length < 20;

    segmentos.push({
      tipo: "codigo",
      linguagem: ehLinguagem ? primeira : "",
      conteudo: ehLinguagem ? linhas.slice(1).join("\n") : parte,
    });
  });

  return segmentos;
}
