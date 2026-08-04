package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64";
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"syscall"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/kbinani/screenshot"
	//"github.com/micmonay/keybd_event"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"google.golang.org/genai"
)

type App struct {
	ctx context.Context
	db  *Database
	storage *ObsidianStorage
	connPython net.Conn
}

type ObsidianStorage struct {
    VaultPath string
}

func NewObsidianStorage(path string) *ObsidianStorage {
    return &ObsidianStorage{VaultPath: path}
}

func (o *ObsidianStorage) AdicionarInteracao(conteudo string){
	data := time.Now().Format("2006-01-02");
	f, err := os.OpenFile(filepath.Join(o.VaultPath,"Logs_Diarios",data+".md"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("[ERRO] Falha ao abrir arquivo de log:", err)
		return
	}
	if _, err := f.Write([]byte(conteudo+"\n")); err != nil {
		f.Close() 
		fmt.Println(err)
	}
	if err := f.Close(); err != nil {
		fmt.Println(err)
	}
}

func (a *App) Resumir(){
	//
}

var toolsLocais = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{
		{
			Name:        "executar_comando_sistema",
			Description: "Executa comandos PowerShell. Se não souber o comando para uma tarefa (ex: tocar música específica), USE A BUSCA GOOGLE primeiro para encontrar o comando/argumento e depois execute-o aqui",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"comando": {Type: genai.TypeString},
				},
				Required: []string{"comando"},
			},
		},
		{
			Name:        "perguntar_ao_claude_code",
			Description: "Usa o agente Claude Code para interagir com o ambiente de desenvolvimento ou enviar mensagens diretas a ele. Use SEMPRE que o usuário pedir para falar com o Claude, ou simplesmente para quando ele pedir para abrir Claude Code.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"instrucao": {Type: genai.TypeString},
				},
				Required: []string{"instrucao"},
			},
		},
		{
			
            Name:        "manipular_interface",
            Description: "Utilize para interações com a interface gráfica, como clicar em botões ou pressionar teclas. Para cliques, calcule as coordenadas baseado na resolução 1920x1080 e chame a função com acao='click', x e y. Para teclas, use acao='tecla' e passe o nome da tecla (ex: 'space', 'enter') no parâmetro texto.",
            Parameters: &genai.Schema{
                Type: genai.TypeObject,
                Properties: map[string]*genai.Schema{
                    "acao": {
                        Type:        genai.TypeString,
                        Description: "Tipo de ação: 'click' para mouse ou 'tecla' para pressionar teclas",
                    },
                    "x": {Type: genai.TypeInteger, Description: "Coordenada X (para clique)"},
                    "y": {Type: genai.TypeInteger, Description: "Coordenada Y (para clique)"},
                    "texto": {
						Type:        genai.TypeString,
						Description: "Texto ou tecla a ser pressionada (para acao 'tecla', ex: 'space', 'enter')",
					},
                },
                Required: []string{"acao"},
            },
		},
		{
			Name:        "pesquisar_internet",
			Description: "Busca informações atualizadas na internet. Use sempre que precisar de dados externos antes de executar comandos.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"query": {Type: genai.TypeString, Description: "O termo de busca"},
				},
				Required: []string{"query"},
			},
		},
	},
}

var toolBusca = &genai.Tool{
	GoogleSearch: &genai.GoogleSearch{},
}

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procSetCursorPos = user32.NewProc("SetCursorPos")
	procMouseEvent   = user32.NewProc("mouse_event")
)

const (
	MOUSEEVENTF_LEFTDOWN = 0x0002
	MOUSEEVENTF_LEFTUP   = 0x0004
)

const caminho_vault = "C:/Users/Arthur/SI/Projetos/Athena(Go)/meu-app-ts/Vault"
const systemPrompt = `Você é a Athena, a inteligência central do sistema do Arthur. 
Sua personalidade é de uma Engenheira de Sistemas de elite: ultra-eficiente e focada em execução.

DIRETRIZES DE PENSAMENTO:
1. AUTONOMIA TOTAL: Se o Arthur pedir para abrir algo ou interagir, nunca pergunte "como" ou "qual comando". Tente o comando mais provável ou use 'pesquisar_internet' para descobrir.
2. VISÃO PROATIVA: Se houver um print da tela, você deve analisá-lo. Para clicar em botões, calcule as coordenadas X/Y (resolução 1920x1080) e execute 'manipular_interface'. É proibido dizer que não consegue calcular coordenadas.
3. FLUXO SEQUENCIAL: Se uma tarefa exige dados que você não tem, o fluxo deve ser: pesquisar_internet -> analisar resultado -> executar_comando ou clicar (nao esqueca de pesquisar comandos para Windows).

REGRAS DE RESPOSTA:
- Use apenas texto puro (sem markdown ou emojis).
- Seja breve. Confirme a execução e informe o resultado final.`

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.WindowSetPosition(ctx, 0, 0)

	db, err := NovoDatabase()
	if err != nil {
		fmt.Println("[ERRO] Falha ao inicializar banco de dados:", err)
	} else {
		a.db = db
		fmt.Println("[OK] Banco de dados inicializado")
	}
	a.storage = NewObsidianStorage(caminho_vault);
}

func (a *App) ChamarClaudeCode(instrucao string) string {
	fmt.Println("[Agente: Claude Code] Disparando script runner...")
	cmd := exec.Command("cmd", "/C", "start", "powershell", "-NoExit", "-ExecutionPolicy", "Bypass", "-File", "rodar_claude.ps1", instrucao)

	cmd.Dir = "."
	err := cmd.Run()

	if err != nil {
		fmt.Printf("[ERRO] Falha ao abrir Claude: %v\n", err)
		return "Erro ao iniciar o Claude Code: " + err.Error()
	}

	return "Claude Code iniciado. Tarefa enviada para o agente."
}

func (a *App) ExecutarNoTerminal(comando string) string {
	fmt.Println("[Execução] Rodando comando:", comando)
	out, err := exec.Command("powershell", "-Command", comando).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Erro ao executar: %s", err)
	}
	return string(out)
}

func (a *App) ManipularInterface(acao string, x, y int, texto string) string {

	carga := map[string]interface{}{
		"acao": acao,
		"x":    x,
		"y":    y,
		"texto": texto,
	}

    jsonData, err := json.Marshal(carga)
	if err != nil {
		return "Erro ao converter dados para JSON: " + err.Error()
	}
	_, err = a.connPython.Write(jsonData)
	if err != nil {
		return "Erro ao enviar dados para Python: " + err.Error()
	}
	fmt.Println("[Athena] Enviando dados para Python:", carga)
	return "Dados enviados para Python."
}

func (a *App) BuscarNoTavily(query string) string {
    fmt.Println("[Athena] Pesquisando na internet via Tavily:", query)
    
    apiKey := os.Getenv("TAVILY_API_KEY")
    apiURL := "https://api.tavily.com/search"

    payload := map[string]interface{}{
        "api_key": apiKey,
        "query":   query,
        "search_depth": "basic",
    }

    jsonData, _ := json.Marshal(payload)
    resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return "Erro ao conectar na busca: " + err.Error()
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    // Retornamos o JSON bruto para o Gemini extrair o que precisa
    return string(body)
}

var speakerInitialized = false

func tocarAudioInvisivel(caminhoArquivo string) {
	f, err := os.Open(caminhoArquivo)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo:", err)
		return
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		fmt.Println("Erro ao decodificar:", err)
		f.Close()
		return
	}

	if !speakerInitialized {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		speakerInitialized = true
	}

	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		streamer.Close()
		f.Close()
	})))
}

func (a *App) IniciarReceptorPython() {
	listener, err := net.Listen("tcp", "127.0.0.1:8081")
	if err != nil {
		fmt.Println("Erro ao abrir porta 8081:", err)
		return
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}

			a.connPython = conn

			go func(c net.Conn) {
				defer c.Close()
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
					texto := scanner.Text()
					fmt.Println("Python enviou:", texto)

					runtime.EventsEmit(a.ctx, "voz_transcricao", texto)

					tocarAudioInvisivel("audios/um_minuto.mp3")

					if err != nil {
						fmt.Println("Erro ao reproduzir:", err)
					}

				}
			}(conn)
		}
	}()
}

func (a *App) AlternarJanela(expandir bool) {
	novaLargura, novaAltura := 100, 100
	if expandir {
		novaLargura, novaAltura = 800, 900
	}

	runtime.WindowSetSize(a.ctx, novaLargura, novaAltura)
}

func (a *App) CapturarTela() (string, error) {
	img, err := screenshot.CaptureDisplay(0)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return "", err
	}

	imgBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return imgBase64, nil
}

func (a *App) capturarTelaBytes() ([]byte, error) {
	img, err := screenshot.CaptureDisplay(0)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (a *App) montarHistorico() []*genai.Content {
	var contents []*genai.Content

	if a.db == nil {
		return contents
	}

	mensagens, err := a.db.BuscarMensagensDoDia()
	if err != nil {
		fmt.Println("[ERRO] Ao buscar histórico:", err)
		return contents
	}

	for _, m := range mensagens {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, &genai.Content{
			Parts: []*genai.Part{
				{Text: m.Content},
			},
			Role: role,
		})
	}

	return contents
}

func (a *App) PerguntaAi(prompt string, printa bool) string {
    client, err := genai.NewClient(a.ctx, &genai.ClientConfig{
        APIKey:  os.Getenv("GEMINI_API_KEY"),
        Backend: genai.BackendGeminiAPI,
    })
    if err != nil {
        return "Erro de API: " + err.Error()
    }

    if a.db != nil {
        a.db.SalvarMensagem("user", prompt)
    }

    contents := a.montarHistorico()
    var currentParts []*genai.Part
    currentParts = append(currentParts, &genai.Part{Text: prompt})

    if printa {
        imgBytes, _ := a.capturarTelaBytes()
        if imgBytes != nil {
            currentParts = append(currentParts, &genai.Part{
                InlineData: &genai.Blob{MIMEType: "image/png", Data: imgBytes},
            })
        }
    }

    // Garante que o último item do histórico seja a pergunta atual com a imagem
    contents = append(contents, &genai.Content{Parts: currentParts, Role: "user"})

    ferramentaUso := []*genai.Tool{toolsLocais}
    config := &genai.GenerateContentConfig{
        SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: systemPrompt}}},
        Tools:             ferramentaUso,
    }

    var respostaFinal string
    
    for i := 0; i < 5; i++ {
        resp, err := client.Models.GenerateContent(a.ctx, "gemini-2.5-flash-lite", contents, config)
        if err != nil {
            return "Erro na Athena: " + err.Error()
        }

        if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
            break
        }
        modeloContent := resp.Candidates[0].Content
        contents = append(contents, modeloContent)

        temChamadaDeFuncao := false
        var toolResponses []*genai.Part

        for _, part := range modeloContent.Parts {
            if part.Text != "" {
                respostaFinal = part.Text
            }

            if part.FunctionCall != nil {
                temChamadaDeFuncao = true
                fnCall := part.FunctionCall
                var resultData string

                switch fnCall.Name {
                case "executar_comando_sistema":
                    if cmd, ok := fnCall.Args["comando"].(string); ok {
                        resultData = a.ExecutarNoTerminal(cmd)
                    }
                case "perguntar_ao_claude_code":
                    if inst, ok := fnCall.Args["instrucao"].(string); ok {
                        resultData = a.ChamarClaudeCode(inst)
                    }
                case "manipular_interface":
                    acao, _ := fnCall.Args["acao"].(string)
                    x, _ := fnCall.Args["x"].(float64)
                    y, _ := fnCall.Args["y"].(float64)
                    texto, _ := fnCall.Args["texto"].(string)
                    resultData = a.ManipularInterface(acao, int(x), int(y), texto)
				case "pesquisar_internet":
                    if q, ok := fnCall.Args["query"].(string); ok {
                        resultData = a.BuscarNoTavily(q)
                    }
                }

                toolResponses = append(toolResponses, &genai.Part{
                    FunctionResponse: &genai.FunctionResponse{
                        Name:     fnCall.Name,
                        Response: map[string]interface{}{"output": resultData},
                    },
                })
            }
        }

        if temChamadaDeFuncao {
            contents = append(contents, &genai.Content{
                Role:  "tool",
                Parts: toolResponses,
            })
            continue
        }

        break
    }

    if respostaFinal == "" { respostaFinal = "Ação concluída com sucesso." }
	if a.storage != nil {
		logMarkdown := fmt.Sprintf("### 🕒 Interação - %s\n**Arthur:** %s\n**Athena:** %s\n---\n", time.Now().Format("15:04"), prompt, respostaFinal)
		a.storage.AdicionarInteracao(logMarkdown)
	}

	arquivoSaida := "audios/audio_jarvis.mp3"
	exec.Command("edge-tts", "--voice", "pt-BR-FranciscaNeural", "--text", respostaFinal, "--write-media", arquivoSaida).Run()
	tocarAudioInvisivel(arquivoSaida)

	if a.db != nil {
		a.db.SalvarMensagem("assistant", respostaFinal)
	}
    
    return respostaFinal
}