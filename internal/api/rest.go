// Package api expõe o jamesd via REST (sessões, histórico, áudio) e
// WebSocket (conversa em tempo real, ponte com o device) — ver Task 17/18.
package api

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

	// EmbutidosSPA serve o build da SPA (web/dist) via catch-all — é o que
	// garante o invariante I3 (jamesd serve a UI pros dois invólucros).
	// Nil (padrão nos testes) desativa a rota sem quebrar o resto.
	EmbutidosSPA fs.FS

	Loop    *agente.Loop
	Memoria memoria.Janela
	// Resumidor é opcional: nil desliga o resumo rolante e a janela volta a
	// simplesmente truncar, que é o comportamento anterior à Fase 7A. Isso
	// mantém todo teste que monta Servidor à mão funcionando sem alteração.
	Resumidor *memoria.Resumidor
	// Notas é a memória durável (Fase 4). Ponteiro e opcional de propósito:
	// nil significa "sem memória", e é o que mantém todo teste anterior
	// funcionando sem alteração.
	Notas               *memoria.Vault
	SistemaPrompt       string
	Clientes            *ClientesHub
	AprovacoesPendentes *registroAprovacoesPendentes
	TimeoutAprovacao    time.Duration

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
	mux.HandleFunc("GET /api/memoria", s.listarMemoria)
	mux.HandleFunc("GET /api/audio/{id}", s.servirAudio)
	mux.HandleFunc("DELETE /api/audio/{id}", s.excluirAudio)
	mux.HandleFunc("GET /ws", s.ws)
	mux.HandleFunc("GET /ws/dispositivo", s.wsDispositivo)
	// Catch-all por último: /api/* e /ws* têm prioridade sobre a SPA.
	if s.EmbutidosSPA != nil {
		mux.Handle("/", http.FileServer(http.FS(s.EmbutidosSPA)))
	}
	return mux
}

func (s *Servidor) saude(w http.ResponseWriter, r *http.Request) {
	responderJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Servidor) listarSessoes(w http.ResponseWriter, r *http.Request) {
	filtro := postgres.FiltroSessoes{Busca: r.URL.Query().Get("busca")}

	if bruto := r.URL.Query().Get("limite"); bruto != "" {
		limite, err := strconv.Atoi(bruto)
		if err != nil {
			http.Error(w, "limite inválido", http.StatusBadRequest)
			return
		}
		filtro.Limite = limite
	}

	// O cursor vem em RFC 3339, que é o formato em que atualizada_em sai
	// deste mesmo endpoint — o cliente devolve o que recebeu, sem converter.
	if bruto := r.URL.Query().Get("antes"); bruto != "" {
		antes, err := time.Parse(time.RFC3339Nano, bruto)
		if err != nil {
			http.Error(w, "cursor inválido", http.StatusBadRequest)
			return
		}
		filtro.Antes = &antes
	}

	sessoes, err := s.Sessoes.ListarPaginado(r.Context(), filtro)
	if err != nil {
		// Erro cru aqui carregaria host, banco e usuário para uma tela que
		// o celular abre pela VPN — mesma regra de mensagemDeErroDoTurno.
		slog.Error("erro ao listar sessões", "erro", err)
		http.Error(w, "não consegui listar as conversas", http.StatusInternalServerError)
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
	mensagens, err := s.Mensagens.ListarPorSessao(r.Context(), id, limiteMensagensSessao)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, mensagens)
}

// listarMemoria alimenta a aba Memória. Nunca devolve 500 por vault
// quebrado: memória é melhor-esforço em todo o resto do sistema (ver o
// tratamento em processarTurno), e a aba segue a mesma regra — devolve
// lista vazia com aviso, para o front conseguir mostrar o problema sem
// tratar a resposta como falha de rede.
func (s *Servidor) listarMemoria(w http.ResponseWriter, r *http.Request) {
	type resposta struct {
		Notas []memoria.NotaAssunto `json:"notas"`
		Aviso string                `json:"aviso,omitempty"`
	}

	if s.Notas == nil {
		responderJSON(w, http.StatusOK, resposta{Notas: []memoria.NotaAssunto{}})
		return
	}

	notas, err := s.Notas.Listar()
	if err != nil {
		// O erro completo vai pro log local; o cliente recebe só a
		// notícia de que falhou. err.Error() aqui carrega o caminho
		// absoluto do vault, e este JSON é renderizado numa tela que o
		// celular também abre pela VPN.
		slog.Warn("erro ao ler memória para a aba", "erro", err)
		responderJSON(w, http.StatusOK, resposta{
			Notas: []memoria.NotaAssunto{},
			Aviso: "não consegui ler a memória",
		})
		return
	}
	if notas == nil {
		notas = []memoria.NotaAssunto{}
	}
	responderJSON(w, http.StatusOK, resposta{Notas: notas})
}

func (s *Servidor) servirAudio(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	for _, f := range formatosDeAudio {
		caminho := filepath.Join(s.DiretorioAudio, id.String()+f.extensao)
		if _, err := os.Stat(caminho); err == nil {
			// Cabeçalho explícito: o Go não conhece .wav na tabela embutida de
			// tipos, e sem ele o navegador farejaria o conteúdo.
			w.Header().Set("Content-Type", f.tipo)
			http.ServeFile(w, r, caminho)
			return
		}
	}
	http.NotFound(w, r)
}

// formatosDeAudio na ordem de preferência: o Piper gera .wav, o edge-tts
// gerava .mp3 e pode voltar por config.
var formatosDeAudio = []struct{ extensao, tipo string }{
	{".wav", "audio/wav"},
	{".mp3", "audio/mpeg"},
}

// excluirAudio apaga o arquivo de áudio gerado depois que o cliente já
// terminou de tocar (ou não conseguiu tocar) — o TTS é efêmero por design
// (spec 4.7), não deveria acumular no disco pra sempre. Idempotente: id
// que já foi apagado (ou nunca existiu) ainda devolve sucesso, não erro —
// o cliente não tem como saber se já chamou antes (ex.: retry de rede).
func (s *Servidor) excluirAudio(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	for _, f := range formatosDeAudio {
		caminho := filepath.Join(s.DiretorioAudio, id.String()+f.extensao)
		if err := os.Remove(caminho); err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func responderJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(corpo)
}
