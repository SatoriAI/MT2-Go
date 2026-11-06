-- Yang: what a character has, at most a round cap under what the client's signed 32-bit
-- points hold, and never below nothing, so an overspend or overflow is refused here too.
ALTER TABLE characters
    ADD COLUMN gold integer NOT NULL DEFAULT 0 CHECK (gold BETWEEN 0 AND 2000000000);

-- An event multiplies experience, yang or both: each from 1 to 10, and at least one above
-- 1, or it is no event. The events already scheduled leave yang as it is.
ALTER TABLE events
    DROP CONSTRAINT events_exp_percent_check,
    ADD CHECK (exp_percent BETWEEN 100 AND 1000),
    ADD COLUMN yang_percent integer NOT NULL DEFAULT 100 CHECK (yang_percent BETWEEN 100 AND 1000),
    ADD CHECK (exp_percent > 100 OR yang_percent > 100);
