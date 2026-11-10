-- The aids an item leaves on a character, each held until something uses it up: Rada
-- Pustelnika's bonus on the next book read, and Zwój Egzorcyzmu's read past a wait.
ALTER TABLE characters
    ADD COLUMN aids text[] NOT NULL DEFAULT '{}'
        CHECK (aids <@ ARRAY['read_bonus', 'no_read_wait']);
