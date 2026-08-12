-- Resumo rolante da conversa (Fase 7A, §7 da spec).
--
-- resumido_ate guarda o mensagens.id da última linha já dobrada no resumo,
-- e NÃO uma contagem: depois de uma retomada o histórico em memória é só a
-- cauda dos 200 registros mais recentes (ws.go, limiteMensagensSessao), e
-- contagem sobre ele resumiria o trecho errado — em silêncio.
--
-- DEFAULT '' e NULL permitido para a migração ser segura em banco que já
-- tem sessões (o de trabalho tem mais de mil).
ALTER TABLE sessoes ADD COLUMN resumo TEXT NOT NULL DEFAULT '';
ALTER TABLE sessoes ADD COLUMN resumido_ate BIGINT;
