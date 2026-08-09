package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/permissao"
)

// clienteCapturaID grava o id real de cada evento recebido (clienteFalso, em
// clientes_test.go, só grava o tipo) — precisamos do id de verdade da
// aprovação criada dentro de criarSolicitarAprovacao pra consultar o estado
// dela no banco depois.
type clienteCapturaID struct {
	perfil permissao.Perfil
	ids    []string
}

func (c *clienteCapturaID) Perfil() permissao.Perfil { return c.perfil }
func (c *clienteCapturaID) Enviar(tipo, id string, dados any) error {
	c.ids = append(c.ids, id)
	return nil
}

// clienteCapturaIDEDados grava tanto o id do envelope quanto o dados (terceiro
// argumento) de cada evento recebido — precisamos comparar os dois pra travar
// o Bug 1 (dados.id sempre vazio no frontend, porque o backend nunca duplica
// o id do envelope dentro do mapa dados).
type clienteCapturaIDEDados struct {
	perfil permissao.Perfil
	ids    []string
	dados  []any
}

func (c *clienteCapturaIDEDados) Perfil() permissao.Perfil { return c.perfil }
func (c *clienteCapturaIDEDados) Enviar(tipo, id string, dados any) error {
	c.ids = append(c.ids, id)
	c.dados = append(c.dados, dados)
	return nil
}

func servidorDeTesteParaAprovacao(t *testing.T) *Servidor {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}
	return &Servidor{
		Sessoes:             postgres.NovoSessaoRepositorio(pool),
		Aprovacoes:          postgres.NovoAprovacaoRepositorio(pool),
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
	}
}

func TestCriarSolicitarAprovacaoExpiraELimpaAoEstourarTimeout(t *testing.T) {
	s := servidorDeTesteParaAprovacao(t)
	s.TimeoutAprovacao = 20 * time.Millisecond

	sessao, err := s.Sessoes.Criar(context.Background(), "sessão de teste aprovação — timeout")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	captura := &clienteCapturaID{perfil: permissao.Privilegiado}
	s.Clientes.Registrar(captura)

	solicitar := s.criarSolicitarAprovacao(sessao.ID)
	aprovado, err := solicitar(context.Background(), agente.ChamadaFerramenta{Nome: "ferramenta-perigosa"})
	if err != nil {
		t.Fatalf("solicitar não deveria devolver erro no caminho de timeout: %v", err)
	}
	if aprovado {
		t.Error("timeout deveria resultar em não-aprovado")
	}

	if len(captura.ids) != 1 {
		t.Fatalf("esperava 1 broadcast de aprovacao_requerida, recebeu %d", len(captura.ids))
	}
	aprovacaoID, err := uuid.Parse(captura.ids[0])
	if err != nil {
		t.Fatalf("id de aprovação inválido: %v", err)
	}

	registro, err := s.Aprovacoes.BuscarPorID(context.Background(), aprovacaoID)
	if err != nil {
		t.Fatalf("erro ao buscar aprovação: %v", err)
	}
	if registro.Estado != "expirada" {
		t.Errorf("esperava estado expirada após timeout, veio %q", registro.Estado)
	}

	if len(s.AprovacoesPendentes.canais) != 0 {
		t.Errorf("esperava mapa de pendentes limpo após timeout, restaram %d entradas", len(s.AprovacoesPendentes.canais))
	}
}

func TestCriarSolicitarAprovacaoExpiraELimpaAoDesconectar(t *testing.T) {
	s := servidorDeTesteParaAprovacao(t)
	s.TimeoutAprovacao = time.Hour // não deve disparar; queremos exercitar o ctx.Done()

	sessao, err := s.Sessoes.Criar(context.Background(), "sessão de teste aprovação — desconexão")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	captura := &clienteCapturaID{perfil: permissao.Privilegiado}
	s.Clientes.Registrar(captura)

	ctx, cancelar := context.WithCancel(context.Background())
	solicitar := s.criarSolicitarAprovacao(sessao.ID)

	tipoResultado := struct {
		aprovado bool
		err      error
	}{}
	resultado := make(chan struct {
		aprovado bool
		err      error
	}, 1)
	go func() {
		aprovado, err := solicitar(ctx, agente.ChamadaFerramenta{Nome: "ferramenta-perigosa"})
		resultado <- struct {
			aprovado bool
			err      error
		}{aprovado, err}
	}()

	time.Sleep(50 * time.Millisecond) // dá tempo do broadcast acontecer antes de cancelar
	cancelar()

	select {
	case r := <-resultado:
		tipoResultado = r
	case <-time.After(2 * time.Second):
		t.Fatal("solicitar não retornou a tempo após cancelamento do contexto")
	}
	if tipoResultado.aprovado {
		t.Error("desconexão não deveria resultar em aprovado")
	}
	if tipoResultado.err == nil {
		t.Error("esperava ctx.Err() de volta")
	}

	if len(captura.ids) != 1 {
		t.Fatalf("esperava 1 broadcast de aprovacao_requerida, recebeu %d", len(captura.ids))
	}
	aprovacaoID, err := uuid.Parse(captura.ids[0])
	if err != nil {
		t.Fatalf("id de aprovação inválido: %v", err)
	}

	registro, err := s.Aprovacoes.BuscarPorID(context.Background(), aprovacaoID)
	if err != nil {
		t.Fatalf("erro ao buscar aprovação: %v", err)
	}
	if registro.Estado != "expirada" {
		t.Errorf("esperava estado expirada após desconexão, veio %q", registro.Estado)
	}

	if len(s.AprovacoesPendentes.canais) != 0 {
		t.Errorf("esperava mapa de pendentes limpo após desconexão, restaram %d entradas", len(s.AprovacoesPendentes.canais))
	}
}

// TestCriarSolicitarAprovacaoIncluiIDDentroDeDados trava o Bug 1 do spike de
// 2026-08-06: o frontend (App.tsx) lê dados.id, mas o ClienteWS só repassa o
// campo "dados" do envelope pros manipuladores — o id do envelope nunca
// chegava até o handler. Sem o id duplicado dentro de "dados", o clique em
// "Aprovar" no desktop manda id="" de volta, que nunca resolve a aprovação
// pendente de verdade.
func TestCriarSolicitarAprovacaoIncluiIDDentroDeDados(t *testing.T) {
	s := servidorDeTesteParaAprovacao(t)
	s.TimeoutAprovacao = 20 * time.Millisecond

	sessao, err := s.Sessoes.Criar(context.Background(), "sessão de teste aprovação — id nos dados")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	captura := &clienteCapturaIDEDados{perfil: permissao.Privilegiado}
	s.Clientes.Registrar(captura)

	solicitar := s.criarSolicitarAprovacao(sessao.ID)
	if _, err := solicitar(context.Background(), agente.ChamadaFerramenta{Nome: "ferramenta-perigosa"}); err != nil {
		t.Fatalf("solicitar não deveria devolver erro no caminho de timeout: %v", err)
	}

	if len(captura.ids) != 1 || len(captura.dados) != 1 {
		t.Fatalf("esperava 1 broadcast de aprovacao_requerida, recebeu %d ids e %d dados", len(captura.ids), len(captura.dados))
	}

	mapa, ok := captura.dados[0].(map[string]any)
	if !ok {
		t.Fatalf("dados do evento não é map[string]any: %#v", captura.dados[0])
	}
	idNosDados, ok := mapa["id"].(string)
	if !ok || idNosDados == "" {
		t.Fatalf(`dados["id"] ausente ou vazio: %#v`, mapa)
	}
	if idNosDados != captura.ids[0] {
		t.Errorf(`dados["id"] = %q, queria o mesmo id do envelope (%q)`, idNosDados, captura.ids[0])
	}
}

// clienteCapturaJSON serializa `dados` pra JSON do mesmo jeito que
// conexaoCliente.Enviar faz de verdade (via codificarDados, em envelope.go)
// — é o único jeito de pegar a diferença entre map[string]any nil e vazio.
// Em memória as duas formas são indistinguíveis pros testes acima
// (clienteCapturaIDEDados guarda o `any` cru: len() e indexação de um mapa
// nil funcionam igual a um mapa vazio, e uma asserção sobre a struct
// passaria mesmo com o bug). Só depois de passar por json.Marshal a
// diferença aparece: nil vira `null`, vazio vira `{}`.
type clienteCapturaJSON struct {
	perfil permissao.Perfil
	brutos []string
}

func (c *clienteCapturaJSON) Perfil() permissao.Perfil { return c.perfil }
func (c *clienteCapturaJSON) Enviar(tipo, id string, dados any) error {
	bruto, err := json.Marshal(dados)
	if err != nil {
		return err
	}
	c.brutos = append(c.brutos, string(bruto))
	return nil
}

// TestCriarSolicitarAprovacaoArgumentosNuloViraObjetoVazio trava a
// divergência entre o tipo TS (DadosAprovacaoRequerida.argumentos:
// Record<string, unknown>, não-nulável) e o que o Go podia emitir de fato.
// chamada.Args é map[string]any (internal/agente/tipos.go), e o gate de
// aprovação roda ANTES de qualquer validação de argumento (internal/agente/
// loop.go) — uma ferramenta perigosa chamada sem parâmetro nenhum chega
// aqui com Args == nil. Map nil serializa como `null` em JSON, não `{}`; o
// card de aprovação no front faz JSON.stringify/itera esse valor, e
// "argumentos": null estoura em runtime mesmo com o tsc calado, porque o
// tipo prometia objeto.
func TestCriarSolicitarAprovacaoArgumentosNuloViraObjetoVazio(t *testing.T) {
	s := servidorDeTesteParaAprovacao(t)
	s.TimeoutAprovacao = 20 * time.Millisecond

	sessao, err := s.Sessoes.Criar(context.Background(), "sessão de teste aprovação — argumentos nulos")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	captura := &clienteCapturaJSON{perfil: permissao.Privilegiado}
	s.Clientes.Registrar(captura)

	solicitar := s.criarSolicitarAprovacao(sessao.ID)
	// Args nil de propósito: é o zero value de ChamadaFerramenta, o que uma
	// ferramenta sem parâmetro nenhum produz.
	if _, err := solicitar(context.Background(), agente.ChamadaFerramenta{Nome: "ferramenta-perigosa"}); err != nil {
		t.Fatalf("solicitar não deveria devolver erro no caminho de timeout: %v", err)
	}

	if len(captura.brutos) != 1 {
		t.Fatalf("esperava 1 broadcast de aprovacao_requerida, recebeu %d", len(captura.brutos))
	}

	var decodificado map[string]json.RawMessage
	if err := json.Unmarshal([]byte(captura.brutos[0]), &decodificado); err != nil {
		t.Fatalf("dados do evento não decodificaram como objeto JSON: %v", err)
	}

	argumentos, ok := decodificado["argumentos"]
	if !ok {
		t.Fatalf(`payload sem a chave "argumentos": %s`, captura.brutos[0])
	}
	if string(argumentos) != "{}" {
		t.Errorf(`argumentos = %s, queria objeto vazio "{}" (não "null") — JSON completo: %s`, string(argumentos), captura.brutos[0])
	}
}
