package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"james/internal/agente"
	"james/internal/ferramentas"
	"james/internal/permissao"
)

const segredoDeTeste = "sk-" + "ant-" + "api03-" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func chamadaDeGravarEnv() agente.ChamadaFerramenta {
	return agente.ChamadaFerramenta{
		ID: "c1", Nome: "alterar_arquivos",
		Args: map[string]any{"acao": "criar", "caminho": "/home/x/p/.env", "conteudo": "CLAUDE_KEY=" + segredoDeTeste + "\n"},
	}
}

func servidorComAlterarArquivos(t *testing.T) *Servidor {
	t.Helper()
	_, s := servidorComDispositivoDeTeste(t)
	if err := s.Loop.Ferramentas.Registrar(ferramentas.NovoAlterarArquivos()); err != nil {
		t.Fatal(err)
	}
	s.TimeoutAprovacao = 20 * time.Millisecond
	return s
}

// O cartão e a linha de aprovação no Postgres não podem ter o segredo que o
// James ia gravar num .env.
func TestAprovacaoDeGravarEmEnvNaoLevaOSegredoNemParaOAppNemParaOBanco(t *testing.T) {
	s := servidorComAlterarArquivos(t)
	sessao, err := s.Sessoes.Criar(context.Background(), "aprovação com segredo")
	if err != nil {
		t.Fatal(err)
	}
	captura := &clienteCapturaIDEDados{perfil: permissao.Privilegiado}
	s.Clientes.Registrar(captura)

	if _, err := s.criarSolicitarAprovacao(sessao.ID)(context.Background(), chamadaDeGravarEnv()); err != nil {
		t.Fatal(err)
	}

	enviado, _ := json.Marshal(captura.dados)
	if strings.Contains(string(enviado), segredoDeTeste) {
		t.Errorf("o segredo foi para o app: %s", enviado)
	}
	if !strings.Contains(string(enviado), "CLAUDE_KEY=") {
		t.Errorf("o cartão perdeu a estrutura: %s", enviado)
	}
	id, err := uuid.Parse(captura.ids[0])
	if err != nil {
		t.Fatal(err)
	}
	registro, err := s.Aprovacoes.BuscarPorID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(registro.Argumentos), segredoDeTeste) {
		t.Errorf("o segredo foi para o Postgres (aprovacoes): %s", registro.Argumentos)
	}
}

// O histórico (coluna chamadas) é a outra cópia que o Postgres guarda.
func TestMensagemComChamadaParaEnvNaoPersisteOSegredo(t *testing.T) {
	s := servidorComAlterarArquivos(t)
	sessao, err := s.Sessoes.Criar(context.Background(), "histórico com segredo")
	if err != nil {
		t.Fatal(err)
	}
	original := chamadaDeGravarEnv()
	mensagens := []agente.Mensagem{{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{original}}}

	s.persistirNovasMensagens(context.Background(), sessao.ID, mensagens, 0)

	linhas, err := s.Mensagens.ListarPorSessao(context.Background(), sessao.ID, 10)
	if err != nil || len(linhas) != 1 {
		t.Fatalf("linhas=%d err=%v", len(linhas), err)
	}
	if strings.Contains(string(linhas[0].Chamadas), segredoDeTeste) {
		t.Errorf("o segredo foi para o Postgres (mensagens): %s", linhas[0].Chamadas)
	}
	if !strings.Contains(original.Args["conteudo"].(string), segredoDeTeste) {
		t.Error("a chamada em memória foi alterada")
	}
}
