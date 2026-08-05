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
// precisa — interface pra poder testar o roteamento HTTP sem uma janela
// nativa de verdade.
type chamadasJanela interface {
	DefinirTamanho(ctx context.Context, largura, altura int)
	ObterPosicao(ctx context.Context) (x, y int)
	DefinirPosicao(ctx context.Context, x, y int)
}

type chamadasJanelaWails struct{}

func (chamadasJanelaWails) DefinirTamanho(ctx context.Context, largura, altura int) {
	wailsruntime.WindowSetSize(ctx, largura, altura)
}

func (chamadasJanelaWails) ObterPosicao(ctx context.Context) (x, y int) {
	return wailsruntime.WindowGetPosition(ctx)
}

func (chamadasJanelaWails) DefinirPosicao(ctx context.Context, x, y int) {
	wailsruntime.WindowSetPosition(ctx, x, y)
}

func iniciarServidorControle(ctx context.Context, chamadas chamadasJanela) {
	mux := montarMuxControle(ctx, chamadas)
	go func() {
		_ = http.ListenAndServe(enderecoControle, comCORS(mux))
	}()
}

func montarMuxControle(ctx context.Context, chamadas chamadasJanela) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tamanho", func(w http.ResponseWriter, r *http.Request) {
		largura, altura, err := lerDimensoes(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		chamadas.DefinirTamanho(ctx, largura, altura)
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
