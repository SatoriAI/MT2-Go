-- Every whisper delivered, kept so a client can show a conversation after logging in
-- again. The text is UTF-8, decoded from the client's code page, which is single-byte:
-- the client sends at most 254 bytes, so at most 254 characters.

CREATE TABLE whispers (
    id        bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sent_at   timestamptz NOT NULL DEFAULT now(),
    sender    integer     NOT NULL REFERENCES characters (id),
    recipient integer     NOT NULL REFERENCES characters (id),
    text      text        NOT NULL CHECK (char_length(text) BETWEEN 1 AND 254)
);

-- A conversation is read between two characters, in order.
CREATE INDEX whispers_conversation ON whispers (sender, recipient, sent_at);
