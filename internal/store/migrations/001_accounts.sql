CREATE TABLE accounts (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text        NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 30),
    password_hash bytea       NOT NULL,
    -- 0 until the player picks a kingdom on their first login.
    empire        smallint    NOT NULL DEFAULT 0 CHECK (empire BETWEEN 0 AND 3),
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE characters (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id   bigint      NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    slot         smallint    NOT NULL CHECK (slot BETWEEN 0 AND 3),
    name         text        NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 24),
    job          smallint    NOT NULL,
    level        smallint    NOT NULL DEFAULT 1,
    play_minutes integer     NOT NULL DEFAULT 0,
    st           smallint    NOT NULL,
    ht           smallint    NOT NULL,
    dx           smallint    NOT NULL,
    iq           smallint    NOT NULL,
    body_part    integer     NOT NULL DEFAULT 0,
    hair_part    integer     NOT NULL DEFAULT 0,
    x            integer     NOT NULL,
    y            integer     NOT NULL,
    skill_group  smallint    NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, slot)
);
