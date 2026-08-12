// Servidor de controle local que a SPA (servida pelo jamesd, numa origem
// diferente da do Wails) usa pra mexer na janela nativa — resize e
// arraste da bolinha flutuante.
//
// Por quê: o runtime JS do Wails (window.runtime.*) só é injetado em
// páginas servidas pelo próprio AssetServer do Wails (reescrita de HTML
// no <head>, ver pkg/assetserver do módulo). Como a página real do James
// é servida pelo jamesd (processo Go separado, origem
// http://127.0.0.1:8080), essa injeção nunca chega lá — window.runtime
// simplesmente não existe na página que o usuário vê, então
// window.runtime.WindowSetSize() sempre foi um no-op silencioso.
//
// Um proxy reverso resolveria a injeção, mas quebraria o WebSocket do
// chat: o AssetServer do Wails devolve 501 pra qualquer request de
// upgrade de WS antes até de chegar num Handler customizado (conferido
// no fonte do módulo, pkg/assetserver/assetserver.go). Em vez disso,
// expomos aqui um servidor HTTP minúsculo, só em loopback, que a SPA
// chama via fetch() pra pedir resize/posição — do lado Go, que tem
// acesso ao ctx da janela, executamos a chamada real via pkg/runtime.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// enderecoControle é loopback fixo — nunca alcançável de fora da máquina,
// então liberar CORS geral nele (comCORS) não abre superfície de rede.
const enderecoControle = "127.0.0.1:17313"

// chamadasJanela é o suficiente do pkg/runtime do Wails que este arquivo
// precisa — interface pra poder testar o roteamento HTTP e a matemática
// de ancoragem sem uma janela nativa de verdade.
type chamadasJanela interface {
	DefinirTamanho(ctx context.Context, largura, altura int)
	ObterTamanho(ctx context.Context) (largura, altura int)
	ObterPosicao(ctx context.Context) (x, y int)
	DefinirPosicao(ctx context.Context, x, y int)
	// TamanhoTelaAtual devolve 0,0 (sem erro tratado à parte) se não
	// conseguir descobrir a tela — o chamador trata isso como "sem
	// clamp", não como falha (câmbio de tela é melhor-esforço).
	TamanhoTelaAtual(ctx context.Context) (largura, altura int)
	Minimizar(ctx context.Context)
	// AlternarMaximizado alterna entre maximizado e restaurado — é o que o
	// círculo verde do design faz, e é a única operação que o Wails expõe
	// (não há "maximizar" e "restaurar" separados).
	AlternarMaximizado(ctx context.Context)
	Fechar(ctx context.Context)
}

type chamadasJanelaWails struct{}

func (chamadasJanelaWails) DefinirTamanho(ctx context.Context, largura, altura int) {
	wailsruntime.WindowSetSize(ctx, largura, altura)
}

func (chamadasJanelaWails) ObterTamanho(ctx context.Context) (int, int) {
	return wailsruntime.WindowGetSize(ctx)
}

func (chamadasJanelaWails) ObterPosicao(ctx context.Context) (x, y int) {
	return wailsruntime.WindowGetPosition(ctx)
}

func (chamadasJanelaWails) DefinirPosicao(ctx context.Context, x, y int) {
	wailsruntime.WindowSetPosition(ctx, x, y)
}

func (chamadasJanelaWails) TamanhoTelaAtual(ctx context.Context) (int, int) {
	telas, err := wailsruntime.ScreenGetAll(ctx)
	if err != nil || len(telas) == 0 {
		return 0, 0
	}
	for _, t := range telas {
		if t.IsCurrent {
			return t.Size.Width, t.Size.Height
		}
	}
	return telas[0].Size.Width, telas[0].Size.Height
}

func (chamadasJanelaWails) Minimizar(ctx context.Context) {
	wailsruntime.WindowMinimise(ctx)
}

func (chamadasJanelaWails) AlternarMaximizado(ctx context.Context) {
	wailsruntime.WindowToggleMaximise(ctx)
}

func (chamadasJanelaWails) Fechar(ctx context.Context) {
	wailsruntime.Quit(ctx)
}

func iniciarServidorControle(ctx context.Context, chamadas chamadasJanela) {
	mux := montarMuxControle(ctx, chamadas)
	go func() {
		_ = http.ListenAndServe(enderecoControle, comCORS(mux))
	}()
}

func montarMuxControle(ctx context.Context, chamadas chamadasJanela) *http.ServeMux {
	mux := http.NewServeMux()
	// /redimensionar (não /tamanho simples) porque um resize ingênuo
	// sempre cresce a partir do canto superior-esquerdo atual — se a
	// bolinha estiver perto da borda inferior da tela (ex.: perto da
	// barra de tarefas), o chat abre pra baixo e sai da área visível.
	// Ancora no canto onde a bolinha já está (ela fica fixa no canto
	// inferior-direito da janela via CSS) e ainda limita contra o
	// tamanho real da tela.
	mux.HandleFunc("POST /redimensionar", func(w http.ResponseWriter, r *http.Request) {
		novaLargura, novaAltura, err := lerDimensoes(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		x, y := chamadas.ObterPosicao(ctx)
		larguraAtual, alturaAtual := chamadas.ObterTamanho(ctx)
		larguraTela, alturaTela := chamadas.TamanhoTelaAtual(ctx)
		novoX, novoY := calcularRedimensionamentoAncorado(
			x, y, larguraAtual, alturaAtual, novaLargura, novaAltura, larguraTela, alturaTela,
		)
		chamadas.DefinirPosicao(ctx, novoX, novoY)
		chamadas.DefinirTamanho(ctx, novaLargura, novaAltura)
	})
	mux.HandleFunc("GET /posicao", func(w http.ResponseWriter, r *http.Request) {
		x, y := chamadas.ObterPosicao(ctx)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"x": x, "y": y})
	})
	mux.HandleFunc("POST /posicao", func(w http.ResponseWriter, r *http.Request) {
		x, y, err := lerPosicao(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		chamadas.DefinirPosicao(ctx, x, y)
	})
	mux.HandleFunc("POST /minimizar", func(w http.ResponseWriter, r *http.Request) {
		chamadas.Minimizar(ctx)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /maximizar", func(w http.ResponseWriter, r *http.Request) {
		chamadas.AlternarMaximizado(ctx)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /fechar", func(w http.ResponseWriter, r *http.Request) {
		// Responde ANTES de fechar: Quit derruba o processo, e o cliente
		// ficaria com um fetch pendurado até o timeout.
		w.WriteHeader(http.StatusNoContent)
		go chamadas.Fechar(ctx)
	})
	return mux
}

// comCORS libera a origem pra SPA (http://127.0.0.1:8080, origem
// diferente deste servidor) conseguir chamar fetch() sem ser barrada.
func comCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// calcularRedimensionamentoAncorado devolve a nova posição (canto
// superior-esquerdo) pra que o redimensionamento pareça crescer/encolher
// a partir do canto inferior-direito atual — onde a bolinha sempre está
// — em vez do canto superior-esquerdo (comportamento padrão do Windows).
// Se larguraTela/alturaTela vierem 0 (tela não descoberta), pula o
// clamp e só faz a ancoragem.
func calcularRedimensionamentoAncorado(
	x, y, larguraAtual, alturaAtual, novaLargura, novaAltura, larguraTela, alturaTela int,
) (novoX, novoY int) {
	cantoX := x + larguraAtual
	cantoY := y + alturaAtual
	novoX = cantoX - novaLargura
	novoY = cantoY - novaAltura

	if larguraTela > 0 {
		if novoX+novaLargura > larguraTela {
			novoX = larguraTela - novaLargura
		}
		if novoX < 0 {
			novoX = 0
		}
	}
	if alturaTela > 0 {
		if novoY+novaAltura > alturaTela {
			novoY = alturaTela - novaAltura
		}
		if novoY < 0 {
			novoY = 0
		}
	}
	return novoX, novoY
}

func lerDimensoes(r *http.Request) (largura, altura int, err error) {
	largura, err = strconv.Atoi(r.URL.Query().Get("largura"))
	if err != nil {
		return 0, 0, err
	}
	altura, err = strconv.Atoi(r.URL.Query().Get("altura"))
	if err != nil {
		return 0, 0, err
	}
	return largura, altura, nil
}

func lerPosicao(r *http.Request) (x, y int, err error) {
	x, err = strconv.Atoi(r.URL.Query().Get("x"))
	if err != nil {
		return 0, 0, err
	}
	y, err = strconv.Atoi(r.URL.Query().Get("y"))
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}
