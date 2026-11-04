-- How far each character has come: experience towards the next level, and the stat points
-- its levels brought that it has not spent yet. A level needs at most 2,029,047,116, which
-- fits an integer.

ALTER TABLE characters
    ADD COLUMN exp         integer  NOT NULL DEFAULT 0 CHECK (exp >= 0),
    ADD COLUMN stat_points smallint NOT NULL DEFAULT 0 CHECK (stat_points >= 0);
