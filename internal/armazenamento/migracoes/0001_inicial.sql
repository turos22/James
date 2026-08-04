CREATE TABLE sessoes (
    id            UUID PRIMARY KEY,
    titulo        TEXT,
    criada_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizada_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE mensagens (
    id        BIGSERIAL PRIMARY KEY,
    sessao_id UUID NOT NULL REFERENCES sessoes(id) ON DELETE CASCADE,
    papel     TEXT NOT NULL CHECK (papel IN ('usuario','modelo','ferramenta')),
    conteudo  TEXT NOT NULL,
    chamadas  JSONB,
    criada_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_mensagens_sessao ON mensagens (sessao_id, criada_em);

CREATE TABLE aprovacoes (
    id             UUID PRIMARY KEY,
    sessao_id      UUID NOT NULL REFERENCES sessoes(id) ON DELETE CASCADE,
    ferramenta     TEXT NOT NULL,
    argumentos     JSONB NOT NULL,
    solicitada_por TEXT NOT NULL,
    estado         TEXT NOT NULL CHECK (estado IN ('pendente','aprovada','negada','expirada')),
    criada_em      TIMESTAMPTZ NOT NULL DEFAULT now(),
    decidida_em    TIMESTAMPTZ
);

CREATE INDEX idx_aprovacoes_pendentes ON aprovacoes (estado, criada_em)
    WHERE estado = 'pendente';
