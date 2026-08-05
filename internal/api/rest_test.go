package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"james/internal/armazenamento/postgres"
)

// TestMain carrega o .env da raiz para que os testes de integração enxerguem
// DATABASE_URL sem depender de o shell já ter exportado a variável (mesma
// convenção de internal/armazenamento/postgres/conexao_test.go).
func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env")
	os.Exit(m.Run())
}

func servidorDeTeste(t *testing.T) *Servidor {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}

	return &Servidor{
		Sessoes:        postgres.NovoSessaoRepositorio(pool),
		Mensagens:      postgres.NovoMensagemRepositorio(pool),
		DiretorioAudio: t.TempDir(),
	}
}

func TestSaudeDevolveOK(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/saude")
	if err != nil {
		t.Fatalf("GET /api/saude devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, queria 200", resp.StatusCode)
	}
}

func TestCriarEListarSessoes(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	corpo, _ := json.Marshal(map[string]string{"titulo": "sessão via REST"})
	resp, err := http.Post(servidor.URL+"/api/sessoes", "application/json", bytes.NewReader(corpo))
	if err != nil {
		t.Fatalf("POST /api/sessoes devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, queria 201", resp.StatusCode)
	}
	var criada map[string]any
	json.NewDecoder(resp.Body).Decode(&criada)
	if criada["ID"] == nil {
		t.Fatal("resposta de criação deveria incluir ID")
	}

	respLista, err := http.Get(servidor.URL + "/api/sessoes")
	if err != nil {
		t.Fatalf("GET /api/sessoes devolveu erro: %v", err)
	}
	defer respLista.Body.Close()
	var lista []map[string]any
	json.NewDecoder(respLista.Body).Decode(&lista)
	if len(lista) == 0 {
		t.Error("esperava pelo menos a sessão recém-criada na listagem")
	}
}

func TestListarMensagensDaSessao(t *testing.T) {
	s := servidorDeTeste(t)
	sessao, err := s.Sessoes.Criar(context.Background(), "sessão pra mensagens via REST")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}
	if _, err := s.Mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("erro ao salvar mensagem de apoio: %v", err)
	}

	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/sessoes/" + sessao.ID.String() + "/mensagens")
	if err != nil {
		t.Fatalf("GET mensagens devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, queria 200", resp.StatusCode)
	}
	var mensagens []map[string]any
	json.NewDecoder(resp.Body).Decode(&mensagens)
	if len(mensagens) != 1 {
		t.Errorf("esperava 1 mensagem, vieram %d", len(mensagens))
	}
}

func TestServirAudioDevolveArquivoExistente(t *testing.T) {
	s := servidorDeTeste(t)
	id := uuid.New().String()
	if err := os.WriteFile(filepath.Join(s.DiretorioAudio, id+".mp3"), []byte("audio falso"), 0o600); err != nil {
		t.Fatalf("erro ao escrever fixture de áudio: %v", err)
	}

	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/audio/" + id)
	if err != nil {
		t.Fatalf("GET /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, queria 200", resp.StatusCode)
	}
}

func TestServirAudioInexistenteDevolve404(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/audio/" + uuid.New().String())
	if err != nil {
		t.Fatalf("GET /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, queria 404", resp.StatusCode)
	}
}

func TestServirAudioComIDInvalidoDevolve400(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/audio/não-é-um-uuid")
	if err != nil {
		t.Fatalf("GET /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, queria 400", resp.StatusCode)
	}
}

func TestExcluirAudioRemoveArquivoDoDisco(t *testing.T) {
	s := servidorDeTeste(t)
	id := uuid.New().String()
	caminho := filepath.Join(s.DiretorioAudio, id+".mp3")
	if err := os.WriteFile(caminho, []byte("audio falso"), 0o600); err != nil {
		t.Fatalf("erro ao escrever fixture de áudio: %v", err)
	}

	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	req, _ := http.NewRequest(http.MethodDelete, servidor.URL+"/api/audio/"+id, nil)
	resp, err := servidor.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, queria 204", resp.StatusCode)
	}
	if _, err := os.Stat(caminho); !os.IsNotExist(err) {
		t.Error("arquivo deveria ter sido removido do disco")
	}
}

func TestExcluirAudioInexistenteAindaDevolve204(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	req, _ := http.NewRequest(http.MethodDelete, servidor.URL+"/api/audio/"+uuid.New().String(), nil)
	resp, err := servidor.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, queria 204 (idempotente — id que já não existe não é erro)", resp.StatusCode)
	}
}

func TestExcluirAudioComIDInvalidoDevolve400(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	req, _ := http.NewRequest(http.MethodDelete, servidor.URL+"/api/audio/não-é-um-uuid", nil)
	resp, err := servidor.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, queria 400", resp.StatusCode)
	}
}

// Não precisa de Postgres real — só exercita o roteamento HTTP puro do
// catch-all da SPA (Task 24), não toca em Sessoes/Mensagens.

func TestCatchAllServeConteudoDaSPAEmbutida(t *testing.T) {
	spa := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>spa embutida</html>")},
	}
	s := &Servidor{EmbutidosSPA: spa}
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/")
	if err != nil {
		t.Fatalf("GET / devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, queria 200", resp.StatusCode)
	}
	corpo, _ := io.ReadAll(resp.Body)
	if string(corpo) != "<html>spa embutida</html>" {
		t.Errorf("corpo = %q, queria o conteúdo embutido em EmbutidosSPA", corpo)
	}
}

func TestCatchAllNaoRoubaRotasDeAPI(t *testing.T) {
	spa := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>spa embutida</html>")},
	}
	s := &Servidor{EmbutidosSPA: spa}
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	// /api/saude não toca em Sessoes/Mensagens (nil aqui), então não precisa
	// de Postgres — só prova que a rota de API vence o catch-all mesmo com
	// EmbutidosSPA setado, o comportamento central que fecha o invariante I3.
	resp, err := http.Get(servidor.URL + "/api/saude")
	if err != nil {
		t.Fatalf("GET /api/saude devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, queria 200 (rota de API deveria vencer o catch-all)", resp.StatusCode)
	}
	var corpo map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
		t.Fatalf("erro ao decodificar corpo: %v", err)
	}
	if corpo["status"] != "ok" {
		t.Errorf("corpo = %v, queria o handler de saude, não o catch-all da SPA", corpo)
	}
}

func TestRotasNaoPanicaComEmbutidosSPANilOuAusente(t *testing.T) {
	s := &Servidor{}
	if s.Rotas() == nil {
		t.Fatal("Rotas() devolveu nil")
	}
}
