package ferramentas_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"james/internal/ferramentas"
	"james/internal/memoria"
)

func TestSalvarNaMemoriaGravaNota(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	f := ferramentas.NovoSalvarNaMemoria(memoria.NovoVault(raiz, 2000))

	saida, err := f.Executar(context.Background(), map[string]any{
		"assunto": "Preferências de Código",
		"texto":   "O Arthur escreve tudo em português.",
	})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !strings.Contains(saida, "preferencias-de-codigo") {
		t.Errorf("saída = %q, não diz onde guardou", saida)
	}

	bruto, err := os.ReadFile(filepath.Join(raiz, "preferencias-de-codigo.md"))
	if err != nil {
		t.Fatalf("nota não foi gravada: %v", err)
	}
	if !strings.Contains(string(bruto), "O Arthur escreve tudo em português.") {
		t.Errorf("conteúdo gravado = %q", bruto)
	}
}

func TestSalvarNaMemoriaEhNivelSeguro(t *testing.T) {
	f := ferramentas.NovoSalvarNaMemoria(memoria.NovoVault(t.TempDir(), 2000))
	if f.Nivel() != ferramentas.NivelSeguro {
		t.Errorf("Nivel = %v, queria NivelSeguro — escrever no próprio vault não controla a máquina", f.Nivel())
	}
}

func TestSalvarNaMemoriaRejeitaParametroFaltando(t *testing.T) {
	f := ferramentas.NovoSalvarNaMemoria(memoria.NovoVault(t.TempDir(), 2000))

	casos := []map[string]any{
		{"assunto": "teste"},
		{"texto": "algo"},
		{"assunto": "", "texto": "algo"},
		{"assunto": "teste", "texto": ""},
		{},
	}
	for _, args := range casos {
		if _, err := f.Executar(context.Background(), args); err == nil {
			t.Errorf("esperava erro para args %v, veio nil", args)
		}
	}
}

func TestSalvarNaMemoriaRejeitaTravessia(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	f := ferramentas.NovoSalvarNaMemoria(memoria.NovoVault(raiz, 2000))

	_, err := f.Executar(context.Background(), map[string]any{
		"assunto": "../../../prompts/sistema",
		"texto":   "conteúdo hostil",
	})
	if err != nil {
		return // rejeitar já resolve
	}
	// Se não rejeitou, o arquivo tem que ter caído dentro da raiz mesmo assim.
	entradas, _ := os.ReadDir(raiz)
	if len(entradas) == 0 {
		t.Fatal("não rejeitou e não gravou dentro da raiz — para onde foi o arquivo?")
	}
}

func TestDeclaracaoListaAssuntosExistentes(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	v := memoria.NovoVault(raiz, 2000)
	if err := v.Acrescentar("projeto-james", "nota"); err != nil {
		t.Fatalf("Acrescentar devolveu erro: %v", err)
	}

	d := ferramentas.NovoSalvarNaMemoria(v).Declaracao()
	if d.Nome != "salvar_na_memoria" {
		t.Errorf("Nome = %q", d.Nome)
	}
	if !strings.Contains(d.Descricao, "projeto-james") {
		t.Errorf("descrição não lista os assuntos existentes: %q", d.Descricao)
	}
}
