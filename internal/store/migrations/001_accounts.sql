-- Lengths are in bytes, because the client's fixed name fields are. Numeric ranges match
-- the fields the client receives, so no stored value can fail to encode. What the game
-- names (kingdom, class, sex, skill group) is stored by name; the server turns names into
-- the client's numbers when it builds a packet. NULL means not chosen yet.

CREATE TABLE accounts (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text        NOT NULL CHECK (octet_length(name) BETWEEN 1 AND 30),
    password_hash bytea       NOT NULL,
    -- Picked on the first login; it never changes after.
    empire        text        CHECK (empire IN ('shinsoo', 'chunjo', 'jinno')),
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Names are unique regardless of case, so nobody can register Admin beside admin.
CREATE UNIQUE INDEX accounts_name_key ON accounts (lower(name));

CREATE TABLE characters (
    id           integer     GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id   bigint      NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    slot         smallint    NOT NULL CHECK (slot BETWEEN 0 AND 3),
    name         text        NOT NULL CHECK (octet_length(name) BETWEEN 1 AND 24),
    class        text        NOT NULL CHECK (class IN ('warrior', 'ninja', 'sura', 'shaman')),
    sex          text        NOT NULL CHECK (sex IN ('male', 'female')),
    level        smallint    NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 255),
    play_minutes integer     NOT NULL DEFAULT 0 CHECK (play_minutes >= 0),
    st           smallint    NOT NULL CHECK (st BETWEEN 0 AND 255),
    ht           smallint    NOT NULL CHECK (ht BETWEEN 0 AND 255),
    dx           smallint    NOT NULL CHECK (dx BETWEEN 0 AND 255),
    iq           smallint    NOT NULL CHECK (iq BETWEEN 0 AND 255),
    hair         integer     NOT NULL DEFAULT 0 CHECK (hair BETWEEN 0 AND 65535),
    map          integer     NOT NULL CHECK (map BETWEEN 1 AND 65535),
    x            integer     NOT NULL,
    y            integer     NOT NULL,
    -- Each class has two paths; a character follows one of its own class's.
    skill_group  text        CHECK (
        (class = 'warrior' AND skill_group IN ('body', 'mental'))
        OR (class = 'ninja' AND skill_group IN ('dagger', 'archery'))
        OR (class = 'sura' AND skill_group IN ('weaponry', 'black_magic'))
        OR (class = 'shaman' AND skill_group IN ('dragon', 'healing'))
    ),
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, slot)
);

CREATE UNIQUE INDEX characters_name_key ON characters (lower(name));
