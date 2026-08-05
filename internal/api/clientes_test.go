package api

import (
	"testing"

	"james/internal/permissao"
)

type clienteFalso struct {
	perfil   permissao.Perfil
	enviados []string
}

func (c *clienteFalso) Perfil() permissao.Perfil { return c.perfil }
func (c *clienteFalso) Enviar(tipo, id string, dados any) error {
	c.enviados = append(c.enviados, tipo)
	return nil
}

func TestBroadcastAprovacaoSoAtingePrivilegiados(t *testing.T) {
	hub := NovoClientesHub()
	privilegiado := &clienteFalso{perfil: permissao.Privilegiado}
	restrito := &clienteFalso{perfil: permissao.Restrito}
	hub.Registrar(privilegiado)
	hub.Registrar(restrito)

	hub.BroadcastAprovacaoRequerida("aprov-1", map[string]string{"ferramenta": "x"})

	if len(privilegiado.enviados) != 1 || privilegiado.enviados[0] != "aprovacao_requerida" {
		t.Errorf("cliente privilegiado deveria ter recebido aprovacao_requerida, recebeu %v", privilegiado.enviados)
	}
	if len(restrito.enviados) != 0 {
		t.Errorf("cliente restrito não deveria receber aprovacao_requerida, recebeu %v", restrito.enviados)
	}
}

func TestRemoverParaDeReceberBroadcast(t *testing.T) {
	hub := NovoClientesHub()
	c := &clienteFalso{perfil: permissao.Privilegiado}
	hub.Registrar(c)
	hub.Remover(c)

	hub.BroadcastAprovacaoRequerida("aprov-1", map[string]string{})
	if len(c.enviados) != 0 {
		t.Error("cliente removido não deveria mais receber broadcast")
	}
}

func TestBroadcastParaPrivilegiadosEnviaTipoEDadosCorretos(t *testing.T) {
	hub := NovoClientesHub()
	privilegiado := &clienteFalso{perfil: permissao.Privilegiado}
	restrito := &clienteFalso{perfil: permissao.Restrito}
	hub.Registrar(privilegiado)
	hub.Registrar(restrito)

	hub.BroadcastParaPrivilegiados("resposta_final", "turno-1", map[string]string{"texto": "oi"})

	if len(privilegiado.enviados) != 1 || privilegiado.enviados[0] != "resposta_final" {
		t.Errorf("privilegiado deveria ter recebido resposta_final, recebeu %v", privilegiado.enviados)
	}
	if len(restrito.enviados) != 0 {
		t.Error("restrito não deveria receber eventos originados do device")
	}
}
