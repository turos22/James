package ferramentas

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Spec 2026-10-14 arquivos, D1, D2, D6, D7.

func executarArquivos(t *testing.T, f *Arquivos, args map[string]any) (string, error) {
	t.Helper()
	return f.Executar(context.Background(), args)
}

func TestArquivosDeclaracaoOrientaOModelo(t *testing.T) {
	d := NovoArquivos(nil).Declaracao()

	if d.Nome != "arquivos" {
		t.Errorf("Nome = %q", d.Nome)
	}
	for _, trecho := range []string{"alterar_arquivos", "cat", "••••", "revelar"} {
		if !strings.Contains(d.Descricao, trecho) {
			t.Errorf("a descrição não diz %q:\n%s", trecho, d.Descricao)
		}
	}
	for _, p := range []string{"acao", "caminho", "nome", "conteudo", "incluir_ocultos", "a_partir_da_linha", "linhas"} {
		if _, ok := d.Parametros[p]; !ok {
			t.Errorf("falta o parâmetro %q", p)
		}
	}
	if len(d.Obrigatorios) != 1 || d.Obrigatorios[0] != "acao" {
		t.Errorf("Obrigatorios = %v", d.Obrigatorios)
	}
}

func TestArquivosENivelPerigosoParaOCelularPedirAprovacao(t *testing.T) {
	if NovoArquivos(nil).Nivel() != NivelPerigoso {
		t.Error("arquivos precisa ser NivelPerigoso: o perfil Restrito (celular) só roda NivelSeguro sem pedir")
	}
}

func TestSoORevelarPedeAprovacaoNoDesktop(t *testing.T) {
	raiz := t.TempDir()
	env := filepath.Join(raiz, ".env")
	escrever(t, env, "CHAVE=valor1\n")
	f := NovoArquivos(nil)

	for _, acao := range []string{"listar", "ler", "buscar"} {
		if f.ConfirmacaoNecessaria(map[string]any{"acao": acao, "caminho": env}) {
			t.Errorf("%s pediu aprovação no desktop", acao)
		}
	}
	if !f.ConfirmacaoNecessaria(map[string]any{"acao": "revelar", "caminho": env}) {
		t.Error("revelar não pediu aprovação")
	}
}

func TestRevelarInvalidoNaoPedeAprovacao(t *testing.T) {
	// Nada a aprovar: caminho relativo ou arquivo que não existe.
	f := NovoArquivos(nil)
	for _, caminho := range []string{"x/y", "", filepath.Join(t.TempDir(), "nao-existe")} {
		if f.ConfirmacaoNecessaria(map[string]any{"acao": "revelar", "caminho": caminho}) {
			t.Errorf("revelar %q pediu aprovação", caminho)
		}
	}
}

func TestRevelarDescreveOQueVaiAcontecer(t *testing.T) {
	raiz := t.TempDir()
	env := filepath.Join(raiz, ".env")
	escrever(t, env, "CHAVE=valor1\n")
	f := NovoArquivos(nil)

	d := f.DescricaoDaConfirmacao(map[string]any{"acao": "revelar", "caminho": env})

	esperado := "mostrar os valores secretos de " + env + " ao modelo (vão para a Anthropic e ficam no histórico desta conversa)"
	if d != esperado {
		t.Errorf("descrição = %q", d)
	}
	if f.DescricaoDaConfirmacao(map[string]any{"acao": "ler", "caminho": env}) != "" {
		t.Error("ler não devia ter descrição de aprovação")
	}
}

func TestArquivosDespachaAsTresAcoes(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "a.txt"), "uma linha\nalvo aqui\n")
	f := NovoArquivos(nil)

	listada, err := executarArquivos(t, f, map[string]any{"acao": "listar", "caminho": raiz})
	if err != nil || !strings.Contains(listada, "a.txt") {
		t.Errorf("listar: %q, %v", listada, err)
	}
	lida, err := executarArquivos(t, f, map[string]any{"acao": "ler", "caminho": filepath.Join(raiz, "a.txt"), "linhas": 1.0})
	if err != nil || !strings.Contains(lida, "     1  uma linha") || strings.Contains(lida, "alvo") {
		t.Errorf("ler: %q, %v", lida, err)
	}
	achada, err := executarArquivos(t, f, map[string]any{"acao": "buscar", "caminho": raiz, "conteudo": "alvo"})
	if err != nil || !strings.Contains(achada, "a.txt:2: alvo aqui") {
		t.Errorf("buscar: %q, %v", achada, err)
	}
}

func TestArquivosRecusaPedidoMalFeito(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "a.txt"), "x\n")
	f := NovoArquivos(nil)

	casos := []struct {
		args map[string]any
		erro string
	}{
		{map[string]any{}, "acao"},
		{map[string]any{"acao": "dançar", "caminho": raiz}, "listar, ler, buscar ou revelar"},
		{map[string]any{"acao": "listar", "caminho": "relativo/x"}, "caminho completo"},
		{map[string]any{"acao": "listar"}, "caminho completo"},
		{map[string]any{"acao": "ler", "caminho": raiz + "/a.txt", "linhas": 3.5}, "linhas"},
		{map[string]any{"acao": "ler", "caminho": raiz + "/a.txt", "linhas": -1.0}, "linhas"},
		{map[string]any{"acao": "ler", "caminho": raiz + "/a.txt", "linhas": "12"}, "linhas"},
		{map[string]any{"acao": "ler", "caminho": raiz + "/a.txt", "a_partir_da_linha": 0.0}, "a_partir_da_linha"},
		{map[string]any{"acao": "listar", "caminho": raiz, "incluir_ocultos": "sim"}, "incluir_ocultos"},
		{map[string]any{"acao": "buscar", "caminho": raiz}, "nome ou conteudo"},
	}
	for _, c := range casos {
		_, err := executarArquivos(t, f, c.args)
		if err == nil || !strings.Contains(err.Error(), c.erro) {
			t.Errorf("%v → err = %v, queria algo com %q", c.args, err, c.erro)
		}
	}
}

func TestArquivosExpandeATilde(t *testing.T) {
	home := comHomeDeTeste(t)
	escrever(t, filepath.Join(home, "nota.txt"), "oi\n")

	saida, err := executarArquivos(t, NovoArquivos(nil), map[string]any{"acao": "listar", "caminho": "~"})

	if err != nil || !strings.Contains(saida, "nota.txt") {
		t.Errorf("saida = %q, err = %v", saida, err)
	}
}

func TestLerNaoRegistraSegredoEORevelarRegistra(t *testing.T) {
	raiz := t.TempDir()
	env := filepath.Join(raiz, ".env")
	escrever(t, env, "CHAVE=valorSecreto123\n")
	segredos := NovoSegredosLidos()
	f := NovoArquivos(segredos)

	lido, err := executarArquivos(t, f, map[string]any{"acao": "ler", "caminho": env})
	if err != nil || strings.Contains(lido, "valorSecreto123") {
		t.Fatalf("ler: %q, %v", lido, err)
	}
	if segredos.Contem("valorSecreto123") {
		t.Error("ler registrou um valor que nunca foi mostrado")
	}

	revelado, err := executarArquivos(t, f, map[string]any{"acao": "revelar", "caminho": env})
	if err != nil || !strings.Contains(revelado, "CHAVE=valorSecreto123") {
		t.Fatalf("revelar: %q, %v", revelado, err)
	}
	if !segredos.Contem("valorSecreto123") {
		t.Error("revelar não registrou o valor no SegredosLidos")
	}
}

func TestRevelarSemSegredosNaoQuebra(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), ".env")
	escrever(t, arquivo, "CHAVE=valor123456\n")

	saida, err := executarArquivos(t, NovoArquivos(nil), map[string]any{"acao": "revelar", "caminho": arquivo})

	if err != nil || !strings.Contains(saida, "valor123456") {
		t.Errorf("saida = %q, err = %v", saida, err)
	}
}

func TestRevelarDeUmArquivoQueNaoExisteEErro(t *testing.T) {
	_, err := executarArquivos(t, NovoArquivos(nil), map[string]any{"acao": "revelar", "caminho": filepath.Join(t.TempDir(), "x")})
	if err == nil || !strings.Contains(err.Error(), "não existe") {
		t.Errorf("err = %v", err)
	}
}
