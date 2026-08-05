// Package api expõe o jamesd via REST (sessões, histórico, áudio) e
// WebSocket (conversa em tempo real, ponte com o device) — ver Task 17/18.
package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/dispositivo"
	"james/internal/memoria"
	"james/internal/voz"
)

type Servidor struct {
	Sessoes        *postgres.SessaoRepositorio
	Mensagens      *postgres.MensagemRepositorio
	Aprovacoes     *postgres.AprovacaoRepositorio
	DiretorioAudio string

	Loop                *agente.Loop
	Memoria             memoria.Janela
	SistemaPrompt       string
	Clientes            *ClientesHub
	AprovacoesPendentes *registroAprovacoesPendentes
	TimeoutAprovacao    time.Duration
	// TimeoutTurno limita cada chamada ao Provider dentro do loop do
	// agente (R4 — timeout em tudo). Zero desativa o limite (usado nos
	// testes deste pacote, que não têm rede real pra travar).
	TimeoutTurno time.Duration

	// Sintetizador é opcional — nil (padrão nos testes) pula a geração de
	// áudio sem quebrar o turno (spec 4.7, Task 15).
	Sintetizador *voz.Sintetizador
	VozTTS       string

	// DispositivoHub é a ponte com o james-device (Task 18): registra a
	// conexão WS do device e casa acao_tela/resultado_acao.
	DispositivoHub *dispositivo.Hub
}

func (s *Servidor) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/saude", s.saude)
	mux.HandleFunc("GET /api/sessoes", s.listarSessoes)
	mux.HandleFunc("POST /api/sessoes", s.criarSessao)
	mux.HandleFunc("GET /api/sessoes/{id}/mensagens", s.listarMensagens)
	mux.HandleFunc("GET /api/audio/{id}", s.servirAudio)
	mux.HandleFunc("GET /ws", s.ws)
	mux.HandleFunc("GET /ws/dispositivo", s.wsDispositivo)
	return mux
}

func (s *Servidor) saude(w http.ResponseWriter, r *http.Request) {
	responderJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Servidor) listarSessoes(w http.ResponseWriter, r *http.Request) {
	sessoes, err := s.Sessoes.Listar(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, sessoes)
}

func (s *Servidor) criarSessao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Titulo string `json:"titulo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		http.Error(w, "corpo inválido", http.StatusBadRequest)
		return
	}
	sessao, err := s.Sessoes.Criar(r.Context(), corpo.Titulo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusCreated, sessao)
}

func (s *Servidor) listarMensagens(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	mensagens, err := s.Mensagens.ListarPorSessao(r.Context(), id, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, mensagens)
}

func (s *Servidor) servirAudio(w http.ResponseWriter, r *http.Request) {
	caminho := filepath.Join(s.DiretorioAudio, r.PathValue("id")+".mp3")
	http.ServeFile(w, r, caminho)
}

func responderJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(corpo)
}
