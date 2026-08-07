package agente

import (
	"context"
	"errors"
	"testing"
	"time"

	"james/internal/ferramentas"
	"james/internal/permissao"
)

// providerFalso devolve as respostas roteirizadas em ordem, uma por chamada
// a Gerar. Não gasta token, não abre janela — a fronteira que o Provider
// promete.
type providerFalso struct {
	respostas []Resposta
	chamadas  int
}

func (p *providerFalso) Gerar(ctx context.Context, req Requisicao) (Resposta, error) {
	if p.chamadas >= len(p.respostas) {
		return Resposta{}, errors.New("providerFalso: sem mais respostas roteirizadas")
	}
	r := p.respostas[p.chamadas]
	p.chamadas++
	return r, nil
}

func (p *providerFalso) GerarStream(ctx context.Context, req Requisicao, emitir func(string)) (Resposta, error) {
	return p.Gerar(ctx, req)
}

type ferramentaFalsa struct {
	nome      string
	nivel     ferramentas.Nivel
	saida     string
	erro      error
	executada bool
}

func (f *ferramentaFalsa) Declaracao() ferramentas.Declaracao {
	return ferramentas.Declaracao{Nome: f.nome}
}
func (f *ferramentaFalsa) Nivel() ferramentas.Nivel { return f.nivel }
func (f *ferramentaFalsa) Executar(ctx context.Context, args map[string]any) (string, error) {
	f.executada = true
	return f.saida, f.erro
}

func novoRegistroCom(fs ...*ferramentaFalsa) ferramentas.Registro {
	r := ferramentas.NovoRegistro()
	for _, f := range fs {
		_ = r.Registrar(f)
	}
	return r
}

func TestExecutarSemChamadaDevolveTextoDireto(t *testing.T) {
	provider := &providerFalso{respostas: []Resposta{{Texto: "oi, tudo certo"}}}
	loop := &Loop{Provider: provider, Ferramentas: ferramentas.NovoRegistro(), MaxIteracoes: 5}

	resp, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "oi"}, permissao.Privilegiado, nil)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if resp.Texto != "oi, tudo certo" {
		t.Errorf("Texto = %q, queria %q", resp.Texto, "oi, tudo certo")
	}
}

func TestExecutarDespachaFerramentaSeguraSemAprovacao(t *testing.T) {
	eco := &ferramentaFalsa{nome: "eco", nivel: ferramentas.NivelSeguro, saida: "resultado do eco"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "eco"}}},
		{Texto: "feito"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(eco), MaxIteracoes: 5}

	pediuAprovacao := false
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		pediuAprovacao = true
		return true, nil
	}

	resp, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda o eco"}, permissao.Restrito, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !eco.executada {
		t.Error("ferramenta segura deveria ter sido executada")
	}
	if pediuAprovacao {
		t.Error("ferramenta segura não deveria pedir aprovação, mesmo com perfil Restrito")
	}
	if resp.Texto != "feito" {
		t.Errorf("Texto = %q, queria feito", resp.Texto)
	}
}

func TestExecutarSolicitaAprovacaoParaFerramentaPerigosaComPerfilRestrito(t *testing.T) {
	perigosa := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "executou"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "feito"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(perigosa), MaxIteracoes: 5}

	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) { return true, nil }

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz a ação"}, permissao.Restrito, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !perigosa.executada {
		t.Error("ferramenta perigosa deveria ter executado após aprovação")
	}
}

func TestExecutarNegaFerramentaPerigosaQuandoAprovacaoRecusada(t *testing.T) {
	perigosa := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "não devia rodar"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "entendido, não fiz"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(perigosa), MaxIteracoes: 5}

	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) { return false, nil }

	_, mensagens, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz a ação"}, permissao.Restrito, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if perigosa.executada {
		t.Error("ferramenta perigosa não deveria ter executado — aprovação foi negada")
	}

	achouErroDeNegacao := false
	for _, m := range mensagens {
		for _, r := range m.Resultados {
			if r.Erro == "ação negada pelo usuário" {
				achouErroDeNegacao = true
			}
		}
	}
	if !achouErroDeNegacao {
		t.Error("esperava um ResultadoFerramenta com Erro de negação no histórico")
	}
}

func TestExecutarPrivilegiadoNuncaPedeAprovacao(t *testing.T) {
	perigosa := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "ok"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "feito"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(perigosa), MaxIteracoes: 5}

	pediuAprovacao := false
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		pediuAprovacao = true
		return true, nil
	}

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz a ação"}, permissao.Privilegiado, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if pediuAprovacao {
		t.Error("perfil Privilegiado nunca deveria pedir aprovação")
	}
	if !perigosa.executada {
		t.Error("perfil Privilegiado deveria executar a ferramenta perigosa direto")
	}
}

func TestExecutarErroDeToolNaoDerrubaLoop(t *testing.T) {
	falha := &ferramentaFalsa{nome: "falha", nivel: ferramentas.NivelSeguro, erro: errors.New("comando não encontrado")}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "falha"}}},
		{Texto: "tentei outra coisa"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(falha), MaxIteracoes: 5}

	resp, mensagens, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda"}, permissao.Privilegiado, nil)
	if err != nil {
		t.Fatalf("erro de tool não deveria derrubar o loop (R2), veio: %v", err)
	}
	if resp.Texto != "tentei outra coisa" {
		t.Errorf("Texto = %q, queria que o loop continuasse até resposta final", resp.Texto)
	}

	achouErro := false
	for _, m := range mensagens {
		for _, r := range m.Resultados {
			if r.Erro == "comando não encontrado" {
				achouErro = true
			}
		}
	}
	if !achouErro {
		t.Error("esperava o erro da tool propagado em ResultadoFerramenta.Erro")
	}
}

func TestExecutarEstouraTetoDeIteracoesRetornaErroHonesto(t *testing.T) {
	nuncaTermina := &ferramentaFalsa{nome: "nunca_termina", nivel: ferramentas.NivelSeguro, saida: "de novo"}
	respostas := make([]Resposta, 0, 4)
	for i := 0; i < 4; i++ {
		respostas = append(respostas, Resposta{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "nunca_termina"}}})
	}
	provider := &providerFalso{respostas: respostas}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(nuncaTermina), MaxIteracoes: 3}

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda"}, permissao.Privilegiado, nil)
	if !errors.Is(err, ErroTetoIteracoes) {
		t.Fatalf("erro = %v, queria ErroTetoIteracoes (R5: mensagem honesta ao estourar o teto)", err)
	}
}

func TestExecutarChamaHooksDeInicioEFimPorFerramenta(t *testing.T) {
	eco := &ferramentaFalsa{nome: "eco", nivel: ferramentas.NivelSeguro, saida: "ok"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "eco"}}},
		{Texto: "feito"},
	}}

	var iniciadas []string
	var concluidas []string
	loop := &Loop{
		Provider: provider, Ferramentas: novoRegistroCom(eco), MaxIteracoes: 5,
		AoIniciarFerramenta:  func(c ChamadaFerramenta) { iniciadas = append(iniciadas, c.Nome) },
		AoConcluirFerramenta: func(r ResultadoFerramenta) { concluidas = append(concluidas, r.Nome) },
	}

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda o eco"}, permissao.Privilegiado, nil)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if len(iniciadas) != 1 || iniciadas[0] != "eco" {
		t.Errorf("AoIniciarFerramenta chamado com %v, queria [eco]", iniciadas)
	}
	if len(concluidas) != 1 || concluidas[0] != "eco" {
		t.Errorf("AoConcluirFerramenta chamado com %v, queria [eco]", concluidas)
	}
}

func TestExecutarFerramentaDesconhecidaDevolveErroSemExecutarNada(t *testing.T) {
	naoRegistrada := &ferramentaFalsa{nome: "fantasma", nivel: ferramentas.NivelSeguro, saida: "não devia rodar"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "fantasma"}}},
		{Texto: "feito"},
	}}
	// fantasma nunca é registrada — só o provider "acha" que ela existe.
	loop := &Loop{Provider: provider, Ferramentas: ferramentas.NovoRegistro(), MaxIteracoes: 5}

	_, mensagens, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda a fantasma"}, permissao.Privilegiado, nil)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if naoRegistrada.executada {
		t.Error("ferramenta não registrada não deveria ter sido executada")
	}

	achouErro := false
	for _, m := range mensagens {
		for _, r := range m.Resultados {
			if r.Nome == "fantasma" && r.Erro != "" {
				achouErro = true
			}
		}
	}
	if !achouErro {
		t.Error("esperava um ResultadoFerramenta com Erro não vazio para ferramenta desconhecida")
	}
}

func TestExecutarErroDeAprovacaoTemPrecedenciaSobreAprovado(t *testing.T) {
	perigosa := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "não devia rodar"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "feito"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(perigosa), MaxIteracoes: 5}

	erroAprovacao := errors.New("canal de aprovação caiu")
	// Roteirizado pra devolver aprovado=true JUNTO com um erro — trava a
	// ordem exata do despacho: erro checado antes do bool, nunca aprovado
	// por omissão quando o canal de aprovação falha.
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		return true, erroAprovacao
	}

	_, mensagens, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz a ação"}, permissao.Restrito, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if perigosa.executada {
		t.Error("ferramenta perigosa não deveria ter executado — solicitarAprovacao devolveu erro")
	}

	achouErroDeAprovacao := false
	for _, m := range mensagens {
		for _, r := range m.Resultados {
			if r.Nome == "perigosa" && r.Erro != "" && r.Erro != "ação negada pelo usuário" {
				achouErroDeAprovacao = true
			}
		}
	}
	if !achouErroDeAprovacao {
		t.Error("esperava ResultadoFerramenta.Erro refletindo o erro do canal de aprovação, não um sucesso")
	}
}

func TestExecutarContextoJaCanceladoDevolveErroSemChamarProvider(t *testing.T) {
	provider := &providerFalso{respostas: []Resposta{{Texto: "nunca deveria chegar aqui"}}}
	loop := &Loop{Provider: provider, Ferramentas: ferramentas.NovoRegistro(), MaxIteracoes: 5}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := loop.Executar(ctx, "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "oi"}, permissao.Privilegiado, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("erro = %v, queria context.Canceled", err)
	}
	if provider.chamadas != 0 {
		t.Errorf("provider.chamadas = %d, queria 0 — contexto já cancelado não devia chamar o provider", provider.chamadas)
	}
}

// ferramentaQueExigeConfirmacao é NivelSeguro (passa pelo Pode de qualquer
// perfil) mas exige confirmação conforme os args — é o formato do
// ComandoSistema depois da Task 3.
type ferramentaQueExigeConfirmacao struct {
	ferramentaFalsa
	exige bool
}

func (f *ferramentaQueExigeConfirmacao) ConfirmacaoNecessaria(args map[string]any) bool {
	return f.exige
}

func TestPrivilegiadoConfirmaQuandoFerramentaExige(t *testing.T) {
	tool := &ferramentaQueExigeConfirmacao{
		ferramentaFalsa: ferramentaFalsa{nome: "escrever", nivel: ferramentas.NivelSeguro, saida: "feito"},
		exige:           true,
	}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "escrever"}}},
		{Texto: "pronto"},
	}}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("Registrar devolveu erro: %v", err)
	}
	loop := &Loop{Provider: provider, Ferramentas: registro, MaxIteracoes: 5}

	pediuAprovacao := false
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		pediuAprovacao = true
		return true, nil
	}

	if _, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "escreve"}, permissao.Privilegiado, solicitar); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !pediuAprovacao {
		t.Error("Privilegiado deveria confirmar quando a ferramenta exige, não pediu")
	}
	if !tool.executada {
		t.Error("ferramenta deveria ter executado depois da confirmação")
	}
}

func TestPrivilegiadoNaoConfirmaQuandoFerramentaNaoExige(t *testing.T) {
	tool := &ferramentaQueExigeConfirmacao{
		ferramentaFalsa: ferramentaFalsa{nome: "ler", nivel: ferramentas.NivelSeguro, saida: "conteúdo"},
		exige:           false,
	}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "ler"}}},
		{Texto: "pronto"},
	}}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("Registrar devolveu erro: %v", err)
	}
	loop := &Loop{Provider: provider, Ferramentas: registro, MaxIteracoes: 5}

	pediuAprovacao := false
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		pediuAprovacao = true
		return true, nil
	}

	if _, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "lê"}, permissao.Privilegiado, solicitar); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if pediuAprovacao {
		t.Error("Privilegiado não deveria confirmar leitura, pediu")
	}
}

// TestRestritoContinuaAprovandoMesmoSemExigencia trava a Decisão D3 da spec:
// ExigeConfirmacao só acrescenta aprovação, nunca remove a que o Nivel() já
// impõe. Sem esta trava, um ConfirmacaoNecessaria devolvendo false poderia
// liberar o celular.
func TestRestritoContinuaAprovandoMesmoSemExigencia(t *testing.T) {
	tool := &ferramentaQueExigeConfirmacao{
		ferramentaFalsa: ferramentaFalsa{nome: "ler", nivel: ferramentas.NivelPerigoso, saida: "conteúdo"},
		exige:           false,
	}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "ler"}}},
		{Texto: "pronto"},
	}}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("Registrar devolveu erro: %v", err)
	}
	loop := &Loop{Provider: provider, Ferramentas: registro, MaxIteracoes: 5}

	pediuAprovacao := false
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		pediuAprovacao = true
		return true, nil
	}

	if _, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "lê"}, permissao.Restrito, solicitar); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !pediuAprovacao {
		t.Error("Restrito deve continuar pedindo aprovação para NivelPerigoso mesmo sem exigência extra")
	}
}

// TestTimeoutDoProviderNaoMataEsperaPorAprovacao trava o bug real reproduzido
// pelo Arthur em 2026-08-06: o timeout do turno embrulhava o loop inteiro,
// incluindo a espera pela decisão humana (até 300s de orçamento). Aprovação
// que demorasse mais que o teto de uma chamada ao Provider (30s por padrão)
// morria antes de chegar — a ação era aprovada e nunca executava.
//
// O teto de tempo vale por chamada ao Provider. Espera humana acontece ENTRE
// chamadas e não pode ter teto de segundos.
func TestTimeoutDoProviderNaoMataEsperaPorAprovacao(t *testing.T) {
	tool := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "executou"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "pronto"},
	}}
	loop := &Loop{
		Provider:        provider,
		Ferramentas:     novoRegistroCom(tool),
		MaxIteracoes:    5,
		TimeoutProvider: 50 * time.Millisecond,
	}

	// Humano lento: demora mais que o teto de uma chamada ao Provider.
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		select {
		case <-time.After(300 * time.Millisecond):
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}

	if _, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz"}, permissao.Restrito, solicitar); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !tool.executada {
		t.Error("a espera pela aprovação foi morta pelo teto do Provider; a ação nunca executou")
	}
}

// TestDesconexaoDoClienteAindaCancelaOLoop garante que a correção não jogue
// fora a defesa que já existia: cliente que desconecta cancela o ctx, e o
// loop não pode seguir despachando ferramenta perigosa.
func TestDesconexaoDoClienteAindaCancelaOLoop(t *testing.T) {
	tool := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "executou"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "pronto"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(tool), MaxIteracoes: 5}

	ctx, cancelar := context.WithCancel(context.Background())
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		cancelar() // o cliente cai enquanto a aprovação está pendente
		return false, ctx.Err()
	}

	if _, _, err := loop.Executar(ctx, "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz"}, permissao.Restrito, solicitar); err == nil {
		t.Error("loop deveria parar quando o cliente desconecta, seguiu normal")
	}
	if tool.executada {
		t.Error("ferramenta perigosa executou depois do cliente desconectar")
	}
}

// ferramentaComImagemFalsa embute ferramentaFalsa (mesmo padrão de
// ferramentaQueExigeConfirmacao) e implementa as duas interfaces, igual
// ver_tela vai implementar: o Registro só aceita Ferramenta, mas o loop
// prefere ExecutarComImagem quando ela existe. Embutir em vez de reimplementar
// do zero dá acesso de graça a nome/nivel/saida/erro já prontos em
// ferramentaFalsa — inclusive erro, essencial pro teste do caminho de falha.
type ferramentaComImagemFalsa struct {
	*ferramentaFalsa
	imagem               []byte
	usouCaminhoComImagem bool
}

func (f *ferramentaComImagemFalsa) ExecutarComImagem(ctx context.Context, args map[string]any) (string, []byte, error) {
	f.usouCaminhoComImagem = true
	if f.erro != nil {
		return "", nil, f.erro
	}
	return f.saida, f.imagem, nil
}

func TestDespachoPrefereExecutarComImagem(t *testing.T) {
	tool := &ferramentaComImagemFalsa{
		ferramentaFalsa: &ferramentaFalsa{nome: "ver_tela", saida: "lista", nivel: ferramentas.NivelSeguro},
		imagem:          []byte{1, 2, 3},
	}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar: %v", err)
	}

	loop := &Loop{Ferramentas: registro, MaxIteracoes: 3}
	resultado := loop.despacharSemHook(context.Background(),
		ChamadaFerramenta{ID: "1", Nome: "ver_tela"}, permissao.Privilegiado, nil)

	if !tool.usouCaminhoComImagem {
		t.Error("o loop chamou Executar em vez de ExecutarComImagem")
	}
	if resultado.Saida != "lista" {
		t.Errorf("Saida = %q, queria lista", resultado.Saida)
	}
	if len(resultado.Imagem) != 3 {
		t.Errorf("Imagem tem %d bytes, queria 3", len(resultado.Imagem))
	}
}

func TestDespachoDeFerramentaSemImagemSegueIgual(t *testing.T) {
	tool := &ferramentaFalsa{nome: "eco", saida: "oi", nivel: ferramentas.NivelSeguro}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar: %v", err)
	}

	loop := &Loop{Ferramentas: registro, MaxIteracoes: 3}
	resultado := loop.despacharSemHook(context.Background(),
		ChamadaFerramenta{ID: "1", Nome: "eco"}, permissao.Privilegiado, nil)

	if resultado.Saida != "oi" {
		t.Errorf("Saida = %q, queria oi", resultado.Saida)
	}
	if resultado.Imagem != nil {
		t.Error("ferramenta sem imagem produziu imagem")
	}
}

// TestDespachoDeErroDeFerramentaComImagemNaoVazaImagemParcial cobre o
// caminho quente que TestDespachoPrefereExecutarComImagem não alcança: o
// `if err != nil` dentro do branch de ExecutarComImagem em despacharSemHook.
// ver_tela vai bater aqui em produção (device offline, timeout de captura) —
// não é caminho frio.
func TestDespachoDeErroDeFerramentaComImagemNaoVazaImagemParcial(t *testing.T) {
	tool := &ferramentaComImagemFalsa{
		ferramentaFalsa: &ferramentaFalsa{nome: "ver_tela", nivel: ferramentas.NivelSeguro, erro: errors.New("device offline")},
	}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar: %v", err)
	}

	loop := &Loop{Ferramentas: registro}
	resultado := loop.despacharSemHook(context.Background(),
		ChamadaFerramenta{ID: "1", Nome: "ver_tela"}, permissao.Privilegiado, nil)

	if resultado.Erro != "device offline" {
		t.Errorf("Erro = %q, queria device offline", resultado.Erro)
	}
	if resultado.Imagem != nil {
		t.Error("erro de ExecutarComImagem não deveria deixar Imagem preenchida")
	}
	if !tool.usouCaminhoComImagem {
		t.Error("deveria ter tentado o caminho com imagem")
	}
}
