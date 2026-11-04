-- Scheduled events: spans of time in which kills give more experience. The multiplier is
-- in percent, 200 doubling. One event runs at a time, so the ranges may not overlap; a
-- range of timestamps needs no extension for the exclusion.

CREATE TABLE events (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
    exp_percent integer     NOT NULL CHECK (exp_percent BETWEEN 101 AND 1000),
    starts_at   timestamptz NOT NULL,
    ends_at     timestamptz NOT NULL,
    CHECK (ends_at > starts_at),
    EXCLUDE USING gist (tstzrange(starts_at, ends_at) WITH &&)
);
