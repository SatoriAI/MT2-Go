-- The stats take the names the game gives them. A character's look is stored; its map is
-- not, because positions are world coordinates and the map follows from them.

ALTER TABLE characters RENAME COLUMN st TO strength;
ALTER TABLE characters RENAME COLUMN ht TO vitality;
ALTER TABLE characters RENAME COLUMN dx TO dexterity;
ALTER TABLE characters RENAME COLUMN iq TO intelligence;

ALTER TABLE characters
    ADD COLUMN shape smallint NOT NULL DEFAULT 0 CHECK (shape IN (0, 1)),
    DROP COLUMN map,
    -- The rule every name follows: 2 to 12 ASCII letters or digits.
    ADD CONSTRAINT characters_name_rule CHECK (name ~ '^[A-Za-z0-9]{2,12}$');
