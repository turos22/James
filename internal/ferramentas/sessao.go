package ferramentas

import "context"

// chaveSessao é um tipo privado pra que nenhum outro pacote consiga colidir
// com esta chave no context — é o padrão recomendado pra valores de contexto.
type chaveSessao struct{}

// ComSessao carimba o id da sessão do James no contexto. O internal/api chama
// isso antes de rodar o loop; ferramentas que precisam saber em qual conversa
// estão leem com SessaoDo.
//
// Vai pelo contexto e não por campo de struct porque cada ferramenta é
// construída uma vez e compartilhada por todas as sessões — um campo seria
// global, e conversas diferentes se misturariam.
func ComSessao(ctx context.Context, sessaoID string) context.Context {
	return context.WithValue(ctx, chaveSessao{}, sessaoID)
}

// SessaoDo devolve o id da sessão carimbado no contexto, ou "" se não houver.
func SessaoDo(ctx context.Context) string {
	id, _ := ctx.Value(chaveSessao{}).(string)
	return id
}
