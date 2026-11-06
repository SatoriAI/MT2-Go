-- The taskbar: what each of a character's slots holds. A slot holds an item, not a cell,
-- so it follows the item as it moves and goes when the item does.
CREATE TABLE taskbar (
    character_id integer  NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    slot         smallint NOT NULL CHECK (slot BETWEEN 0 AND 35),
    item_id      bigint   NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    PRIMARY KEY (character_id, slot)
);

-- Deleting an item looks up the slots holding it, as it drops them.
CREATE INDEX taskbar_item_id ON taskbar (item_id);
