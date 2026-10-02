-- +goose Up
CREATE SCHEMA IF NOT EXISTS ai;

-- Chat name and nick are copied here so usage reports don't depend on the
-- history service's tables.
CREATE TABLE ai.usage (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ts                    timestamptz NOT NULL,
    chat_id               bigint      NOT NULL,
    chat_name             text        NOT NULL,
    nick                  text        NOT NULL,
    model                 text        NOT NULL,
    prompt_message_id     bigint,
    completion_message_id bigint,
    prompt_tokens         integer     NOT NULL DEFAULT 0,
    completion_tokens     integer     NOT NULL DEFAULT 0,
    total_tokens          integer     NOT NULL DEFAULT 0
);

CREATE INDEX usage_ts ON ai.usage (ts);
CREATE INDEX usage_prompt ON ai.usage (prompt_message_id);

CREATE TABLE ai.blocklist (
    jid_or_nick text PRIMARY KEY
);

CREATE TABLE ai.preludes (
    chat_id bigint PRIMARY KEY,
    text    text NOT NULL
);

-- Import data from the Python (Pony ORM) version when its tables exist.
-- +goose StatementBegin
DO $$
BEGIN
    IF to_regclass('public.aiusage') IS NOT NULL THEN
        INSERT INTO ai.usage (ts, chat_id, chat_name, nick, model, prompt_message_id, completion_message_id,
                              prompt_tokens, completion_tokens, total_tokens)
        SELECT m.utctime AT TIME ZONE 'UTC', m.chat, c.name, m.nick, am.name, u.prompt, u.completion,
               coalesce(u.prompt_tokens, 0), coalesce(u.completion_tokens, 0), coalesce(u.total_tokens, 0)
        FROM public.aiusage u
        JOIN public.message m ON m.id = u.prompt
        JOIN public.chat c ON c.id = m.chat
        JOIN public.aimodel am ON am.id = u.model;
    END IF;

    IF to_regclass('public.blockedusers') IS NOT NULL THEN
        INSERT INTO ai.blocklist (jid_or_nick)
        SELECT jid_or_nick FROM public.blockedusers
        ON CONFLICT DO NOTHING;
    END IF;

    IF to_regclass('public.aiprelude') IS NOT NULL THEN
        INSERT INTO ai.preludes (chat_id, text)
        SELECT chat, prelude FROM public.aiprelude
        ON CONFLICT DO NOTHING;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP SCHEMA ai CASCADE;
