-- What each character refuses from other players, chosen in the client's options and kept
-- across logins. Only whispers are refused so far; the rest wait for their mechanics.

ALTER TABLE characters
    ADD COLUMN blocks text[] NOT NULL DEFAULT '{}'
        CHECK (blocks <@ ARRAY['exchange', 'party', 'guild', 'whisper', 'friend', 'party_request']);
