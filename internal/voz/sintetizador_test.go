package voz

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGerarChamaExecutorEDevolveArquivoUnicoPorID(t *testing.T) {
	var vozRecebida, arquivoRecebido string
	s := &Sintetizador{
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			vozRecebida = voz
			arquivoRecebido = arquivoSaida
			return nil
		},
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	caminho, err := s.Gerar(context.Background(), "olá Arthur", "pt-BR-AntonioNeural", "resposta-123")
	if err != nil {
		t.Fatalf("Gerar devolveu erro: %v", err)
	}
	if vozRecebida != "pt-BR-AntonioNeural" {
		t.Errorf("voz repassada = %q", vozRecebida)
	}
	if filepath.Base(caminho) != "resposta-123.mp3" {
		t.Errorf("caminho = %q, queria terminar em resposta-123.mp3 (nome único por resposta, corrige bug #7)", caminho)
	}
	if arquivoRecebido != caminho {
		t.Errorf("arquivo passado ao Executor = %q, deveria bater com o devolvido = %q", arquivoRecebido, caminho)
	}
}

func TestGerarPropagaErroDoExecutor(t *testing.T) {
	s := &Sintetizador{
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			return errors.New("edge-tts não encontrado")
		},
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	_, err := s.Gerar(context.Background(), "teste", "pt-BR-AntonioNeural", "id-1")
	if err == nil {
		t.Fatal("esperava erro propagado do Executor (corrige bug #7: erro nunca ignorado), veio nil")
	}
}

func TestGerarRespeitaTimeout(t *testing.T) {
	s := &Sintetizador{
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		DiretorioSaida: t.TempDir(),
		Timeout:        20 * time.Millisecond,
	}

	_, err := s.Gerar(context.Background(), "teste", "pt-BR-AntonioNeural", "id-2")
	if err == nil {
		t.Fatal("esperava erro de timeout (R4), veio nil")
	}
}

func TestGerarIDsDiferentesGeramArquivosDiferentes(t *testing.T) {
	s := &Sintetizador{
		Executor:       func(ctx context.Context, texto, voz, arquivoSaida string) error { return nil },
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	a, _ := s.Gerar(context.Background(), "um", "pt-BR-AntonioNeural", "id-a")
	b, _ := s.Gerar(context.Background(), "dois", "pt-BR-AntonioNeural", "id-b")
	if a == b {
		t.Error("respostas diferentes não podem colidir no mesmo arquivo (corrige bug #7: arquivo de saída fixo)")
	}
}

// --- motor Piper (spec 2026-10-10 voz em tempo real, D8) ---

// servidorDeTTS imita o servidor de TTS do Piper: guarda o texto que recebeu
// e devolve bytes de WAV.
func servidorDeTTS(t *testing.T, status int, corpo []byte) (*httptest.Server, *[]string) {
	t.Helper()
	var recebidos []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sintetizar" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var dados struct {
			Texto string `json:"texto"`
		}
		_ = json.NewDecoder(r.Body).Decode(&dados)
		mu.Lock()
		recebidos = append(recebidos, dados.Texto)
		mu.Unlock()
		w.Header().Set("Content-Type", "audio/wav")
		w.WriteHeader(status)
		_, _ = w.Write(corpo)
	}))
	t.Cleanup(srv.Close)
	return srv, &recebidos
}

func TestPiperPostaOTextoELimpoEGravaWav(t *testing.T) {
	srv, recebidos := servidorDeTTS(t, http.StatusOK, []byte("RIFF....WAVE"))
	s := NovoSintetizadorPiper(t.TempDir(), srv.URL)

	caminho, err := s.Gerar(context.Background(), "**Canberra** é a capital.", "ignorada", "resp-1")

	if err != nil {
		t.Fatalf("Gerar devolveu erro: %v", err)
	}
	if filepath.Base(caminho) != "resp-1.wav" {
		t.Errorf("caminho = %q, queria terminar em resp-1.wav", caminho)
	}
	bruto, err := os.ReadFile(caminho)
	if err != nil || string(bruto) != "RIFF....WAVE" {
		t.Errorf("conteúdo = %q (erro %v), queria os bytes do servidor", bruto, err)
	}
	if len(*recebidos) != 1 || (*recebidos)[0] != "Canberra é a capital." {
		t.Errorf("o servidor recebeu %v; o Markdown tem de sair antes de falar", *recebidos)
	}
}

func TestPiperServidorForaDoArDevolveErro(t *testing.T) {
	srv, _ := servidorDeTTS(t, http.StatusOK, []byte("RIFF"))
	url := srv.URL
	srv.Close()
	s := NovoSintetizadorPiper(t.TempDir(), url)

	if _, err := s.Gerar(context.Background(), "oi", "", "id-1"); err == nil {
		t.Fatal("esperava erro com o servidor de TTS fora do ar, veio nil")
	}
}

func TestPiperRespostaDeErroNaoDeixaArquivo(t *testing.T) {
	srv, _ := servidorDeTTS(t, http.StatusInternalServerError, []byte("falhou"))
	dir := t.TempDir()
	s := NovoSintetizadorPiper(dir, srv.URL)

	if _, err := s.Gerar(context.Background(), "oi", "", "id-1"); err == nil {
		t.Fatal("HTTP 500 deveria virar erro")
	}
	if itens, _ := os.ReadDir(dir); len(itens) != 0 {
		t.Errorf("sobrou arquivo depois do erro: %v", itens)
	}
}

func TestPiperRespostaVaziaEErro(t *testing.T) {
	srv, _ := servidorDeTTS(t, http.StatusOK, nil)
	dir := t.TempDir()
	s := NovoSintetizadorPiper(dir, srv.URL)

	if _, err := s.Gerar(context.Background(), "oi", "", "id-1"); err == nil {
		t.Fatal("áudio vazio deveria virar erro (tocar um arquivo vazio não diz nada)")
	}
	if itens, _ := os.ReadDir(dir); len(itens) != 0 {
		t.Errorf("sobrou arquivo vazio: %v", itens)
	}
}

func TestTextoSoDeDecoracaoNaoVaiAoServidor(t *testing.T) {
	srv, recebidos := servidorDeTTS(t, http.StatusOK, []byte("RIFF"))
	s := NovoSintetizadorPiper(t.TempDir(), srv.URL)

	if _, err := s.Gerar(context.Background(), "---", "", "id-1"); err == nil {
		t.Fatal("texto sem nada pra falar deveria devolver erro")
	}
	if len(*recebidos) != 0 {
		t.Errorf("o servidor foi chamado com %v", *recebidos)
	}
}

// A limpeza vale pros dois motores: o edge também leria os asteriscos.
func TestEdgeTambemRecebeOTextoLimpo(t *testing.T) {
	var recebido string
	s := &Sintetizador{
		DiretorioSaida: t.TempDir(),
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			recebido = texto
			return nil
		},
	}

	if _, err := s.Gerar(context.Background(), "veja **isto**", "pt-BR-AntonioNeural", "id-1"); err != nil {
		t.Fatalf("Gerar devolveu erro: %v", err)
	}
	if recebido != "veja isto" {
		t.Errorf("o executor recebeu %q, queria o texto sem Markdown", recebido)
	}
}

// Truncar em silêncio gravaria um WAV com cabeçalho inconsistente, e o app
// receberia audio_pronto de um arquivo que não toca.
func TestPiperAudioMaiorQueOLimiteEErroENaoArquivoTruncado(t *testing.T) {
	anterior := tamanhoMaximoDeAudio
	tamanhoMaximoDeAudio = 100
	t.Cleanup(func() { tamanhoMaximoDeAudio = anterior })

	srv, _ := servidorDeTTS(t, http.StatusOK, make([]byte, 101))
	dir := t.TempDir()
	s := NovoSintetizadorPiper(dir, srv.URL)

	if _, err := s.Gerar(context.Background(), "oi", "", "id-1"); err == nil {
		t.Fatal("áudio acima do limite deveria virar erro")
	}
	if itens, _ := os.ReadDir(dir); len(itens) != 0 {
		t.Errorf("sobrou arquivo truncado: %v", itens)
	}
}

func TestPiperAudioExatamenteNoLimiteAindaVale(t *testing.T) {
	anterior := tamanhoMaximoDeAudio
	tamanhoMaximoDeAudio = 100
	t.Cleanup(func() { tamanhoMaximoDeAudio = anterior })

	srv, _ := servidorDeTTS(t, http.StatusOK, make([]byte, 100))
	s := NovoSintetizadorPiper(t.TempDir(), srv.URL)

	if _, err := s.Gerar(context.Background(), "oi", "", "id-1"); err != nil {
		t.Fatalf("áudio no limite devia passar: %v", err)
	}
}

// --- DuracaoDoWAV: quanto tempo o app vai falar (anti-eco do device) ----------

func wavDe(t *testing.T, amostras int, taxa uint32) string {
	t.Helper()
	dados := make([]byte, amostras*2)
	cab := make([]byte, 44)
	copy(cab[0:], "RIFF")
	binary.LittleEndian.PutUint32(cab[4:], uint32(36+len(dados)))
	copy(cab[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(cab[16:], 16)
	binary.LittleEndian.PutUint16(cab[20:], 1)
	binary.LittleEndian.PutUint16(cab[22:], 1)
	binary.LittleEndian.PutUint32(cab[24:], taxa)
	binary.LittleEndian.PutUint32(cab[28:], taxa*2)
	binary.LittleEndian.PutUint16(cab[32:], 2)
	binary.LittleEndian.PutUint16(cab[34:], 16)
	copy(cab[36:], "data")
	binary.LittleEndian.PutUint32(cab[40:], uint32(len(dados)))
	arquivo := filepath.Join(t.TempDir(), "a.wav")
	if err := os.WriteFile(arquivo, append(cab, dados...), 0o600); err != nil {
		t.Fatal(err)
	}
	return arquivo
}

func TestDuracaoDoWAVLeOCabecalho(t *testing.T) {
	d, err := DuracaoDoWAV(wavDe(t, 22050*3, 22050))
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if d != 3*time.Second {
		t.Errorf("duração = %v, queria 3s", d)
	}
}

func TestDuracaoDoWAVPulaChunksAntesDoData(t *testing.T) {
	arquivo := wavDe(t, 22050, 22050)
	bruto, _ := os.ReadFile(arquivo)
	extra := append([]byte("LIST"), 4, 0, 0, 0, 'a', 'b', 'c', 'd')
	comLista := append(append(append([]byte{}, bruto[:36]...), extra...), bruto[36:]...)
	if err := os.WriteFile(arquivo, comLista, 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := DuracaoDoWAV(arquivo)
	if err != nil || d != time.Second {
		t.Errorf("duração = %v, err = %v", d, err)
	}
}

func TestDuracaoDoWAVInvalidoDaErro(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "x.wav")
	_ = os.WriteFile(arquivo, []byte("RIFF"), 0o600)
	if _, err := DuracaoDoWAV(arquivo); err == nil {
		t.Error("WAV truncado não deu erro")
	}
	if _, err := DuracaoDoWAV(filepath.Join(t.TempDir(), "nao-existe.wav")); err == nil {
		t.Error("arquivo inexistente não deu erro")
	}
}

func TestEstimarDuracaoPeloTexto(t *testing.T) {
	if d := EstimarDuracao(strings.Repeat("a", 120)); d < 5*time.Second || d > 15*time.Second {
		t.Errorf("120 caracteres → %v, esperava algo entre 5s e 15s", d)
	}
	if EstimarDuracao("") != 0 {
		t.Error("texto vazio devia dar 0")
	}
}
