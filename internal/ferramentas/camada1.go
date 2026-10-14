package ferramentas

import (
	"context"
	"fmt"
	"math"
	"strings"

	"james/internal/dispositivo"
)

// Camada 1 da spec 2026-10-13 B2: ações por significado, sem pixel. O device
// faz por D-Bus (KWin, MPRIS, Gio); aqui só se valida e se manda pela ponte.
// No Windows o device responde "indisponível neste sistema" (D13).

// Janelas lista, foca e fecha janelas.
type Janelas struct {
	Ponte dispositivo.Ponte
}

func NovoJanelas(ponte dispositivo.Ponte) *Janelas { return &Janelas{Ponte: ponte} }

func (j *Janelas) Declaracao() Declaracao {
	return Declaracao{
		Nome: "janelas",
		Descricao: "Lista as janelas abertas, traz uma para frente ou fecha uma. Use para dar foco a um app " +
			"(ex: 'coloca o foco no Firefox') em vez de clicar na tela.",
		Parametros: map[string]Parametro{
			"acao": {Tipo: "string", Descricao: "'listar', 'focar' ou 'fechar'"},
			"alvo": {Tipo: "string", Descricao: "parte do título ou o nome do app (ex: 'firefox', 'anotações'). Obrigatório para focar e fechar"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (j *Janelas) Nivel() Nivel { return NivelPerigoso }

// ConfirmacaoNecessaria: fechar pede aprovação (o app pode perder o que não
// foi salvo); listar e focar rodam livres.
func (j *Janelas) ConfirmacaoNecessaria(args map[string]any) bool {
	acao, _ := args["acao"].(string)
	return acao == "fechar"
}

var acoesDeJanela = map[string]string{"listar": "janelas_listar", "focar": "janela_focar", "fechar": "janela_fechar"}

func (j *Janelas) Executar(ctx context.Context, args map[string]any) (string, error) {
	acao, _ := args["acao"].(string)
	destino, ok := acoesDeJanela[acao]
	if !ok {
		return "", fmt.Errorf("acao %q desconhecida: use listar, focar ou fechar", acao)
	}
	alvo, _ := args["alvo"].(string)
	if acao != "listar" && strings.TrimSpace(alvo) == "" {
		return "", fmt.Errorf("informe o alvo (parte do título ou o nome do app) para %s", acao)
	}
	r, err := j.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: destino, Alvo: alvo})
	if err != nil {
		return "", err
	}
	return r.Saida, nil
}

// Midia controla o player de mídia (Spotify, navegador...) sem tela.
type Midia struct {
	Ponte dispositivo.Ponte
}

func NovoMidia(ponte dispositivo.Ponte) *Midia { return &Midia{Ponte: ponte} }

func (m *Midia) Declaracao() Declaracao {
	return Declaracao{
		Nome: "midia",
		Descricao: "Controla a música ou o vídeo que está tocando (Spotify, navegador, qualquer player). " +
			"Use para tocar, pausar, trocar de faixa ou dizer o que está tocando, em vez de clicar na tela.",
		Parametros: map[string]Parametro{
			"acao":   {Tipo: "string", Descricao: "'tocar', 'pausar', 'alternar', 'proxima', 'anterior' ou 'estado'"},
			"player": {Tipo: "string", Descricao: "nome do player (ex: 'spotify'). Omitir para usar o que está tocando"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (m *Midia) Nivel() Nivel { return NivelSeguro }

var acoesDeMidia = map[string]bool{"tocar": true, "pausar": true, "alternar": true, "proxima": true, "anterior": true, "estado": true}

func (m *Midia) Executar(ctx context.Context, args map[string]any) (string, error) {
	acao, _ := args["acao"].(string)
	if !acoesDeMidia[acao] {
		return "", fmt.Errorf("acao %q desconhecida: use tocar, pausar, alternar, proxima, anterior ou estado", acao)
	}
	player, _ := args["player"].(string)
	r, err := m.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: "midia", Texto: acao, Alvo: player})
	if err != nil {
		return "", err
	}
	return r.Saida, nil
}

// Abrir abre app, arquivo, pasta ou URL; com linha, abre o arquivo no VS Code.
type Abrir struct {
	Ponte dispositivo.Ponte
}

func NovoAbrir(ponte dispositivo.Ponte) *Abrir { return &Abrir{Ponte: ponte} }

func (a *Abrir) Declaracao() Declaracao {
	return Declaracao{
		Nome: "abrir",
		Descricao: "Abre um app pelo nome (ex: 'dolphin', 'firefox'), um arquivo, uma pasta ou uma URL no app padrão. " +
			"Com 'linha', abre o arquivo nessa linha no VS Code.",
		Parametros: map[string]Parametro{
			"alvo":  {Tipo: "string", Descricao: "nome do app, caminho de arquivo ou pasta (pode começar com ~), ou URL"},
			"linha": {Tipo: "integer", Descricao: "linha do arquivo para abrir no VS Code (a partir de 1). Omitir no resto"},
		},
		Obrigatorios: []string{"alvo"},
	}
}

func (a *Abrir) Nivel() Nivel { return NivelPerigoso }

func (a *Abrir) Executar(ctx context.Context, args map[string]any) (string, error) {
	alvo, _ := args["alvo"].(string)
	if strings.TrimSpace(alvo) == "" {
		return "", fmt.Errorf("parâmetro alvo ausente ou vazio")
	}
	linha := 0
	if bruto, existe := args["linha"]; existe && bruto != nil {
		numero, ok := bruto.(float64)
		if !ok || numero < 1 || numero != math.Trunc(numero) {
			return "", fmt.Errorf("linha inválida: %v (um número inteiro a partir de 1)", bruto)
		}
		linha = int(numero)
	}
	r, err := a.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: "abrir", Alvo: alvo, Linha: linha})
	if err != nil {
		return "", err
	}
	return r.Saida, nil
}
