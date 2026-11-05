-- The hit points and mana a character had when it was last saved in play. Both are NULL
-- until then, and such a character enters with all of them. A character is never saved
-- with no hit points, since that is dead; a dead one is saved as a restart in town leaves
-- it.

ALTER TABLE characters
    ADD COLUMN hp integer CHECK (hp > 0),
    ADD COLUMN sp integer CHECK (sp >= 0),
    ADD CHECK ((hp IS NULL) = (sp IS NULL));
