export interface LeituraTela {
  superficie: string;
  elementos: string[];
}

// interpretarSaidaVerTela lê o texto que a ferramenta ver_tela devolve:
// spec da Fase 5, seção 5.1, e a implementação real em
// dispositivo/james_device/tela.py:_ver_tela / elementos.py:formatar.
//
// A primeira linha é o cabeçalho "superficie: <ID>[ (primaria)] <LxA>
// escala <N>%"; cada linha seguinte é um elemento, formatado como
// `<id>  <tipo>  "<nome>"  (<x>,<y>)` (dois espaços entre campos — conferido
// contra elementos.py:formatar, não presumido). O parser não quebra a linha
// de elemento em campos: cada uma vira uma string opaca, então o
// espaçamento exato do device não importa aqui, só o corte entre cabeçalho
// e o resto.
//
// Quando a varredura estoura o teto de 80 elementos, elementos.py anexa uma
// última linha de aviso ("... e mais N elementos ..."). Ela não começa com
// id, mas entra na lista de elementos igual às outras — é a forma mais
// simples de fazer o aviso chegar até a tela sem inventar um segundo canal.
export function interpretarSaidaVerTela(saida: string): LeituraTela {
  const linhas = saida.split("\n").map((l) => l.trim()).filter(Boolean);
  const primeira = linhas[0] ?? "";

  if (primeira.startsWith("superficie:")) {
    return {
      superficie: primeira.slice("superficie:".length).trim(),
      elementos: linhas.slice(1),
    };
  }
  return { superficie: "", elementos: linhas };
}
