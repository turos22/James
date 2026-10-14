package ferramentas

import "testing"

func TestDestrutivoCasaOQueApagaOuDescarta(t *testing.T) {
	for _, nome := range []string{
		"Excluir", "excluir arquivo", "Mover para a lixeira", "Esvaziar lixeira", "Apagar", "Remover",
		"Deletar", "Delete", "Remove", "Descartar alterações", "Desinstalar", "Formatar", "Não salvar",
		"Don't Save", "Move to Trash", "EXCLUÍR", "  excluir permanentemente  ", "Erase",
	} {
		if !Destrutivo(nome) {
			t.Errorf("Destrutivo(%q) = false, queria true", nome)
		}
	}
}

func TestDestrutivoNaoCasaOQueSoParece(t *testing.T) {
	for _, nome := range []string{
		"Removedor de manchas", "Salvar", "Abrir", "Excelente", "Novo", "", "Lixeirinha fofa", "Formatação",
		"Salvar e não fechar",
	} {
		if Destrutivo(nome) {
			t.Errorf("Destrutivo(%q) = true, queria false", nome)
		}
	}
}
