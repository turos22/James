package ferramentas

import (
	"context"
	"testing"
)

type ferramentaFalsa struct {
	nome  string
	nivel Nivel
}

func (f ferramentaFalsa) Declaracao() Declaracao {
	return Declaracao{Nome: f.nome, Descricao: "ferramenta de teste"}
}
func (f ferramentaFalsa) Nivel() Nivel { return f.nivel }
func (f ferramentaFalsa) Executar(ctx context.Context, args map[string]any) (string, error) {
	return "ok", nil
}

func TestRegistrarEBuscar(t *testing.T) {
	r := NovoRegistro()
	f := ferramentaFalsa{nome: "teste", nivel: NivelSeguro}

	if err := r.Registrar(f); err != nil {
		t.Fatalf("Registrar devolveu erro: %v", err)
	}

	achada, ok := r.Buscar("teste")
	if !ok {
		t.Fatal("Buscar não achou a ferramenta registrada")
	}
	if achada.Declaracao().Nome != "teste" {
		t.Errorf("nome = %q, queria teste", achada.Declaracao().Nome)
	}
}

func TestRegistrarRejeitaNomeDuplicado(t *testing.T) {
	r := NovoRegistro()
	f := ferramentaFalsa{nome: "teste", nivel: NivelSeguro}

	if err := r.Registrar(f); err != nil {
		t.Fatalf("primeiro Registrar devolveu erro: %v", err)
	}
	if err := r.Registrar(f); err == nil {
		t.Fatal("esperava erro no registro duplicado, veio nil")
	}
}

func TestBuscarNomeInexistenteDevolveFalse(t *testing.T) {
	r := NovoRegistro()
	_, ok := r.Buscar("não-existe")
	if ok {
		t.Fatal("Buscar deveria devolver false pra nome não registrado")
	}
}

func TestDeclaracoesListaTodas(t *testing.T) {
	r := NovoRegistro()
	_ = r.Registrar(ferramentaFalsa{nome: "a", nivel: NivelSeguro})
	_ = r.Registrar(ferramentaFalsa{nome: "b", nivel: NivelPerigoso})

	decls := r.Declaracoes()
	if len(decls) != 2 {
		t.Fatalf("esperava 2 declarações, vieram %d", len(decls))
	}
}
