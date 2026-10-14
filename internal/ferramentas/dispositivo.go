package ferramentas

import (
	"context"
	"fmt"

	"james/internal/dispositivo"
)

// ManipularInterface porta app.go:194 (ManipularInterface) atrás da
// interface Ferramenta, trocando o socket TCP cru por internal/dispositivo.
// Coordenadas de clicar_elemento vêm resolvidas por id/nome via ver_tela
// (Anexo A #20, consertado nesta fase); o clique por pixel cru ('click')
// continua existindo como escape para canvas/jogo, sem árvore de elementos.
type ManipularInterface struct {
	Ponte dispositivo.Ponte
	// Mapa dá o nome de "e7" antes do clique, para a aprovação do clique
	// destrutivo (D10 da spec 2026-10-13 B2). Nil: só o nome passado nos
	// argumentos é conferido.
	Mapa FonteDeMapa
}

// FonteDeMapa é o que dá a lista do último ver_tela — *dispositivo.Hub implementa.
type FonteDeMapa interface {
	UltimoMapa() []dispositivo.Elemento
}

// NovoManipularInterface usa a própria ponte como fonte do mapa quando ela
// sabe dar um (o Hub sabe).
func NovoManipularInterface(ponte dispositivo.Ponte) *ManipularInterface {
	m := &ManipularInterface{Ponte: ponte}
	if fonte, ok := ponte.(FonteDeMapa); ok {
		m.Mapa = fonte
	}
	return m
}

// ConfirmacaoNecessaria implementa ExigeConfirmacao: clique em elemento de
// nome destrutivo e tecla Delete pedem aprovação; o resto roda livre
// (decisão do Arthur). Clique por coordenada não entra aqui: o device recusa
// clique por coordenada em cima de elemento listado (D10 d), e é isso que
// impede de contornar esta checagem.
func (m *ManipularInterface) ConfirmacaoNecessaria(args map[string]any) bool {
	acao, _ := args["acao"].(string)
	switch acao {
	case "clicar_elemento":
		return Destrutivo(m.nomeDoElemento(args))
	case "tecla":
		texto, _ := args["texto"].(string)
		return teclaApaga(texto)
	}
	return false
}

// nomeDoElemento resolve o nome pelo id no último mapa, ou usa o nome dos
// argumentos. Os dois contam: se qualquer um for destrutivo, pede.
func (m *ManipularInterface) nomeDoElemento(args map[string]any) string {
	nome, _ := args["nome"].(string)
	id, _ := args["elemento_id"].(string)
	if id == "" || m.Mapa == nil {
		return nome
	}
	for _, e := range m.Mapa.UltimoMapa() {
		if e.ID == id {
			return e.Nome + " " + nome
		}
	}
	return nome
}

func (m *ManipularInterface) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "manipular_interface",
		Descricao: "Interações com a interface gráfica. Prefira 'clicar_elemento' com o id ou nome vindo de ver_tela; clique por coordenada só quando o elemento não aparecer na lista.",
		Parametros: map[string]Parametro{
			"acao":        {Tipo: "string", Descricao: "'clicar_elemento', 'click' para clique por coordenada, 'tecla' para uma tecla nomeada sozinha, ou 'digitar' para escrever texto"},
			"elemento_id": {Tipo: "string", Descricao: "id do elemento vindo de ver_tela (ex: e7), para acao 'clicar_elemento'"},
			"nome":        {Tipo: "string", Descricao: "nome do elemento, alternativa ao elemento_id"},
			"tipo":        {Tipo: "string", Descricao: "tipo do controle (ex: Button), usado junto com nome para desambiguar"},
			"x":           {Tipo: "integer", Descricao: "coordenada X local à superfície (para acao 'click')"},
			"y":           {Tipo: "integer", Descricao: "coordenada Y local à superfície (para acao 'click')"},
			"texto":       {Tipo: "string", Descricao: "para acao 'tecla', uma tecla nomeada sozinha (ex: 'space', 'enter', 'tab') — nunca uma frase. Para acao 'digitar', o texto livre a escrever no campo em foco (ex: um termo de busca)"},
			"superficie":  {Tipo: "string", Descricao: "id do monitor. Omitir para usar o monitor da janela em foco"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (m *ManipularInterface) Nivel() Nivel { return NivelPerigoso }

func (m *ManipularInterface) Executar(ctx context.Context, args map[string]any) (string, error) {
	acao, ok := args["acao"].(string)
	if !ok || acao == "" {
		return "", fmt.Errorf("parâmetro acao ausente ou vazio")
	}
	x, _ := args["x"].(float64)
	y, _ := args["y"].(float64)
	texto, _ := args["texto"].(string)
	superficie, _ := args["superficie"].(string)
	elementoID, _ := args["elemento_id"].(string)
	nome, _ := args["nome"].(string)
	tipo, _ := args["tipo"].(string)

	r, err := m.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{
		Acao: acao, X: int(x), Y: int(y), Texto: texto,
		Superficie: superficie, ElementoID: elementoID, Nome: nome, Tipo: tipo,
	})
	if err != nil {
		return "", err
	}
	return r.Saida, nil // clique/tecla não devolvem imagem
}
