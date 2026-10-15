package ferramentas

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

// Spec 2026-10-14 arquivos, D3, D10, D13, D16 e seção 5.

func alterar(t *testing.T, args map[string]any) (string, error) {
	t.Helper()
	return NovoAlterarArquivos().Executar(context.Background(), args)
}

func pedeAprovacao(args map[string]any) bool {
	return NovoAlterarArquivos().ConfirmacaoNecessaria(args)
}

func TestAlterarArquivosDeclaracaoENivel(t *testing.T) {
	f := NovoAlterarArquivos()
	d := f.Declaracao()

	if d.Nome != "alterar_arquivos" || f.Nivel() != NivelPerigoso {
		t.Errorf("nome %q, nível %v", d.Nome, f.Nivel())
	}
	for _, p := range []string{"acao", "caminho", "conteudo", "sobrescrever", "criar_pastas",
		"trecho_antigo", "trecho_novo", "todas", "origem", "destino"} {
		if _, ok := d.Parametros[p]; !ok {
			t.Errorf("falta o parâmetro %q", p)
		}
	}
	for _, trecho := range []string{"aprovação", "lixeira", "arquivos"} {
		if !strings.Contains(d.Descricao, trecho) {
			t.Errorf("a descrição não diz %q:\n%s", trecho, d.Descricao)
		}
	}
	if len(d.Obrigatorios) != 1 || d.Obrigatorios[0] != "acao" {
		t.Errorf("Obrigatorios = %v", d.Obrigatorios)
	}
}

func TestTodaAcaoValidaPedeAprovacao(t *testing.T) {
	pasta := t.TempDir()
	existente := filepath.Join(pasta, "a.txt")
	escrever(t, existente, "um dois três")

	validos := []map[string]any{
		{"acao": "criar", "caminho": filepath.Join(pasta, "novo.txt"), "conteudo": "x"},
		{"acao": "criar", "caminho": existente, "conteudo": "x", "sobrescrever": true},
		{"acao": "editar", "caminho": existente, "trecho_antigo": "dois", "trecho_novo": "2"},
		{"acao": "mover", "origem": existente, "destino": filepath.Join(pasta, "b.txt")},
		{"acao": "copiar", "origem": existente, "destino": filepath.Join(pasta, "c.txt")},
		{"acao": "apagar", "caminho": existente},
	}
	for _, args := range validos {
		if !pedeAprovacao(args) {
			t.Errorf("%v não pediu aprovação", args["acao"])
		}
	}
}

func TestAcaoDesconhecidaOuAusentePedeAprovacaoPorSeguranca(t *testing.T) {
	for _, args := range []map[string]any{{"acao": "formatar"}, {}, {"acao": ""}} {
		if !pedeAprovacao(args) {
			t.Errorf("%v: devia falhar para o lado seguro", args)
		}
	}
}

func TestPedidoInvalidoDeSaidaNaoPedeAprovacaoEExecutarDevolveOErro(t *testing.T) {
	comHomeDeTeste(t)
	pasta := t.TempDir()
	existente := filepath.Join(pasta, "a.txt")
	escrever(t, existente, "repetido repetido")
	outro := filepath.Join(pasta, "b.txt")
	escrever(t, outro, "x")

	invalidos := []struct {
		args map[string]any
		erro string
	}{
		{map[string]any{"acao": "editar", "caminho": existente, "trecho_antigo": "nao-existe", "trecho_novo": "y"}, "0 ocorrências"},
		{map[string]any{"acao": "editar", "caminho": existente, "trecho_antigo": "repetido", "trecho_novo": "y"}, "2 vezes"},
		{map[string]any{"acao": "criar", "caminho": existente, "conteudo": "x"}, "já existe"},
		{map[string]any{"acao": "mover", "origem": existente, "destino": outro}, "já existe"},
		{map[string]any{"acao": "copiar", "origem": existente, "destino": outro}, "já existe"},
		{map[string]any{"acao": "criar", "caminho": "relativo/x.txt", "conteudo": "x"}, "caminho completo"},
		{map[string]any{"acao": "apagar", "caminho": "/etc"}, "protegida"},
		{map[string]any{"acao": "mover", "origem": "/usr", "destino": filepath.Join(pasta, "u")}, "protegida"},
		{map[string]any{"acao": "apagar", "caminho": filepath.Join(pasta, "nao-existe")}, "não existe"},
		{map[string]any{"acao": "mover", "origem": filepath.Join(pasta, "nao-existe"), "destino": filepath.Join(pasta, "z")}, "não existe"},
	}
	for _, c := range invalidos {
		if pedeAprovacao(c.args) {
			t.Errorf("%v pediu aprovação para um pedido inválido", c.args)
		}
		if _, err := alterar(t, c.args); err == nil || !strings.Contains(err.Error(), c.erro) {
			t.Errorf("%v → err = %v, queria algo com %q", c.args, err, c.erro)
		}
	}
}

func TestParametrosFaltandoDaoErroClaroSemTocarNoDisco(t *testing.T) {
	pasta := t.TempDir()
	alvo := filepath.Join(pasta, "a.txt")
	escrever(t, alvo, "x")

	casos := []struct {
		args map[string]any
		erro string
	}{
		{map[string]any{"acao": "criar", "caminho": filepath.Join(pasta, "n")}, "conteudo"},
		{map[string]any{"acao": "criar", "conteudo": "x"}, "caminho"},
		{map[string]any{"acao": "editar", "caminho": alvo, "trecho_novo": "y"}, "trecho_antigo"},
		{map[string]any{"acao": "editar", "caminho": alvo, "trecho_antigo": "x"}, "trecho_novo"},
		{map[string]any{"acao": "mover", "origem": alvo}, "destino"},
		{map[string]any{"acao": "copiar", "destino": alvo}, "origem"},
		{map[string]any{"acao": "apagar"}, "caminho"},
		{map[string]any{"acao": "criar", "caminho": filepath.Join(pasta, "n"), "conteudo": "x", "sobrescrever": "sim"}, "sobrescrever"},
		{map[string]any{"acao": "dançar"}, "criar, editar, mover, copiar ou apagar"},
	}
	for _, c := range casos {
		if _, err := alterar(t, c.args); err == nil || !strings.Contains(err.Error(), c.erro) {
			t.Errorf("%v → err = %v, queria %q", c.args, err, c.erro)
		}
	}
	if lerTudo(t, alvo) != "x" {
		t.Error("algum pedido inválido mexeu no arquivo")
	}
}

func TestEditarComTrechoNovoVazioApagaOTrecho(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, "fica removido fica")

	_, err := alterar(t, map[string]any{"acao": "editar", "caminho": arquivo, "trecho_antigo": " removido", "trecho_novo": ""})

	if err != nil || lerTudo(t, arquivo) != "fica fica" {
		t.Errorf("err %v, conteúdo %q", err, lerTudo(t, arquivo))
	}
}

// --- cartão de aprovação -------------------------------------------------------

func descricao(args map[string]any) string {
	return NovoAlterarArquivos().DescricaoDaConfirmacao(args)
}

func TestDescricoesDoCartao(t *testing.T) {
	pasta := t.TempDir()
	existente := filepath.Join(pasta, "a.go")
	escrever(t, existente, "func um() {}\nfunc dois() {}\n")
	novo := filepath.Join(pasta, "n.txt")
	b := filepath.Join(pasta, "b.txt")

	casos := []struct {
		args     map[string]any
		esperado string
	}{
		{map[string]any{"acao": "criar", "caminho": novo, "conteudo": strings.Repeat("x", 120)}, "criar " + novo + " (120 bytes)"},
		{map[string]any{"acao": "criar", "caminho": existente, "conteudo": "abc", "sobrescrever": true}, "sobrescrever " + existente + " (3 bytes)"},
		{map[string]any{"acao": "editar", "caminho": existente, "trecho_antigo": "dois", "trecho_novo": "DOIS"},
			"editar " + existente + `: trocar 1 trecho (de "dois" por "DOIS")`},
		{map[string]any{"acao": "mover", "origem": existente, "destino": b}, "mover " + existente + " → " + b},
		{map[string]any{"acao": "copiar", "origem": existente, "destino": b}, "copiar " + existente + " → " + b},
		{map[string]any{"acao": "apagar", "caminho": existente}, "apagar " + existente + " (vai para a lixeira)"},
	}
	for _, c := range casos {
		if got := descricao(c.args); got != c.esperado {
			t.Errorf("%v\n  descrição %q\n  esperada  %q", c.args["acao"], got, c.esperado)
		}
	}
}

func TestDescricaoDoEditarCortaEscapaEPluraliza(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	longo := strings.Repeat("a", 80)
	escrever(t, arquivo, longo+"\nlinha2\n"+longo)

	d := descricao(map[string]any{"acao": "editar", "caminho": arquivo, "trecho_antigo": longo, "trecho_novo": "x\ny", "todas": true})

	if !strings.Contains(d, "trocar 2 trechos") {
		t.Errorf("sem plural: %q", d)
	}
	if !strings.Contains(d, strings.Repeat("a", 60)+"…") || strings.Contains(d, strings.Repeat("a", 61)) {
		t.Errorf("o trecho não foi cortado em 60: %q", d)
	}
	if !strings.Contains(d, `por "x\ny"`) || strings.Contains(d, "\n") {
		t.Errorf("quebra de linha não escapada: %q", d)
	}
}

func TestDescricaoDoEditarEmArquivoSensivelNaoMostraTrecho(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), ".env")
	escrever(t, arquivo, "CHAVE=valorSecreto123\n")

	d := descricao(map[string]any{"acao": "editar", "caminho": arquivo, "trecho_antigo": "valorSecreto123", "trecho_novo": "outroSegredo456"})

	if d != "editar "+arquivo+": trocar 1 trecho" {
		t.Errorf("descrição = %q", d)
	}
	if strings.Contains(d, "valorSecreto123") || strings.Contains(d, "outroSegredo456") {
		t.Error("o cartão mostrou um trecho de arquivo sensível")
	}
}

func TestDescricaoVaziaParaPedidoInvalido(t *testing.T) {
	if d := descricao(map[string]any{"acao": "editar", "caminho": "relativo", "trecho_antigo": "a", "trecho_novo": "b"}); d != "" {
		t.Errorf("descrição = %q", d)
	}
}

// --- execução ------------------------------------------------------------------

func TestExecutarFazCadaAcao(t *testing.T) {
	lixeira := lixeiraFalsa(t)
	pasta := t.TempDir()
	a, b, c := filepath.Join(pasta, "a.txt"), filepath.Join(pasta, "b.txt"), filepath.Join(pasta, "c.txt")

	if _, err := alterar(t, map[string]any{"acao": "criar", "caminho": a, "conteudo": "um dois"}); err != nil || lerTudo(t, a) != "um dois" {
		t.Fatalf("criar: %v", err)
	}
	if _, err := alterar(t, map[string]any{"acao": "editar", "caminho": a, "trecho_antigo": "dois", "trecho_novo": "2"}); err != nil || lerTudo(t, a) != "um 2" {
		t.Fatalf("editar: %v", err)
	}
	if _, err := alterar(t, map[string]any{"acao": "copiar", "origem": a, "destino": b}); err != nil || lerTudo(t, b) != "um 2" {
		t.Fatalf("copiar: %v", err)
	}
	if _, err := alterar(t, map[string]any{"acao": "mover", "origem": b, "destino": c}); err != nil || lerTudo(t, c) != "um 2" {
		t.Fatalf("mover: %v", err)
	}
	if _, err := alterar(t, map[string]any{"acao": "apagar", "caminho": c}); err != nil || len(*lixeira) != 1 || (*lixeira)[0] != c {
		t.Fatalf("apagar: %v, lixeira %v", err, *lixeira)
	}
}

func TestExecutarExpandeATilde(t *testing.T) {
	home := comHomeDeTeste(t)

	if _, err := alterar(t, map[string]any{"acao": "criar", "caminho": "~/nota.txt", "conteudo": "oi"}); err != nil {
		t.Fatalf("erro: %v", err)
	}

	if lerTudo(t, filepath.Join(home, "nota.txt")) != "oi" {
		t.Error("não criou na home")
	}
}

func TestExecutarReconfereOQueFoiAprovado(t *testing.T) {
	pasta := t.TempDir()
	arquivo := filepath.Join(pasta, "a.txt")
	escrever(t, arquivo, "um dois três")
	args := map[string]any{"acao": "editar", "caminho": arquivo, "trecho_antigo": "dois", "trecho_novo": "2"}
	if !pedeAprovacao(args) {
		t.Fatal("devia pedir aprovação")
	}

	// O Arthur aprova... e o arquivo muda por fora antes do clique.
	escrever(t, arquivo, "um outra-coisa três")

	if _, err := alterar(t, args); err == nil || !strings.Contains(err.Error(), "0 ocorrências") {
		t.Errorf("err = %v", err)
	}
	if lerTudo(t, arquivo) != "um outra-coisa três" {
		t.Error("escreveu mesmo com o trecho aprovado já fora do arquivo")
	}

	// E o destino que não existia na hora do cartão passa a existir.
	novo := filepath.Join(pasta, "novo.txt")
	criar := map[string]any{"acao": "criar", "caminho": novo, "conteudo": "meu"}
	if !pedeAprovacao(criar) {
		t.Fatal("devia pedir aprovação")
	}
	escrever(t, novo, "de outra pessoa")
	if _, err := alterar(t, criar); err == nil || lerTudo(t, novo) != "de outra pessoa" {
		t.Errorf("sobrescreveu um arquivo que apareceu depois do cartão: %v", err)
	}
}

func TestLogRegistraAcaoECaminhoNuncaOConteudo(t *testing.T) {
	var saida bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&saida, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	pasta := t.TempDir()
	arquivo := filepath.Join(pasta, "a.txt")

	if _, err := alterar(t, map[string]any{"acao": "criar", "caminho": arquivo, "conteudo": "SEGREDO-NO-CONTEUDO"}); err != nil {
		t.Fatal(err)
	}
	if _, err := alterar(t, map[string]any{"acao": "editar", "caminho": arquivo, "trecho_antigo": "SEGREDO-NO-CONTEUDO", "trecho_novo": "OUTRO-SEGREDO-NOVO"}); err != nil {
		t.Fatal(err)
	}

	log := saida.String()
	if !strings.Contains(log, "criar") || !strings.Contains(log, "editar") || !strings.Contains(log, arquivo) {
		t.Errorf("o log não registra ação e caminho:\n%s", log)
	}
	for _, vazou := range []string{"SEGREDO-NO-CONTEUDO", "OUTRO-SEGREDO-NOVO"} {
		if strings.Contains(log, vazou) {
			t.Errorf("o log guardou conteúdo: %q\n%s", vazou, log)
		}
	}
}

// O que o James grava num arquivo de segredo não pode ir parar no Postgres em
// claro: só a estrutura fica legível. (O cartão mostra o texto real.)
func TestArgumentosParaRegistroMascaraConteudoDeArquivoSensivel(t *testing.T) {
	const segredo = "sk-" + "ant-" + "api03-" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	original := map[string]any{
		"acao": "criar", "caminho": "/home/x/projeto/.env",
		"conteudo": "CLAUDE_KEY=" + segredo + "\nPORTA=8080\n",
	}
	saida := NovoAlterarArquivos().ArgumentosParaRegistro(original)

	conteudo, _ := saida["conteudo"].(string)
	if strings.Contains(conteudo, segredo) {
		t.Errorf("o segredo continua no registro: %q", conteudo)
	}
	if !strings.Contains(conteudo, "CLAUDE_KEY=") || !strings.Contains(conteudo, "PORTA=") {
		t.Errorf("a estrutura sumiu junto: %q", conteudo)
	}
	if saida["acao"] != "criar" || saida["caminho"] != "/home/x/projeto/.env" {
		t.Errorf("campos que não são conteúdo mudaram: %v", saida)
	}
	if !strings.Contains(original["conteudo"].(string), segredo) {
		t.Error("o mapa ORIGINAL foi alterado: a execução gravaria o conteúdo mascarado")
	}
}

func TestArgumentosParaRegistroMascaraTrechosDaEdicao(t *testing.T) {
	saida := NovoAlterarArquivos().ArgumentosParaRegistro(map[string]any{
		"acao": "editar", "caminho": "/home/x/.aws/credentials",
		"trecho_antigo": "aws_secret_access_key = ANTIGAANTIGAANTIGA1",
		"trecho_novo":   "aws_secret_access_key = NOVANOVANOVANOVA22",
	})
	for _, campo := range []string{"trecho_antigo", "trecho_novo"} {
		v, _ := saida[campo].(string)
		if strings.Contains(v, "ANTIGA") || strings.Contains(v, "NOVA") || !strings.Contains(v, "aws_secret_access_key") {
			t.Errorf("%s = %q", campo, v)
		}
	}
}

func TestArgumentosParaRegistroNaoMexeEmArquivoComum(t *testing.T) {
	original := map[string]any{"acao": "criar", "caminho": "/home/x/notas.md", "conteudo": "a = b\n"}
	saida := NovoAlterarArquivos().ArgumentosParaRegistro(original)
	if saida["conteudo"] != "a = b\n" {
		t.Errorf("mascarou arquivo comum: %v", saida["conteudo"])
	}
}
