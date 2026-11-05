-- What each character owns, and what is to be given to it or taken from it.
--
-- An item is one row: its kind by the client's number, how many of it, and the inventory
-- cell its top stands in, two pages of 45. A taller item covering cells below its top is
-- checked by the game, which knows each kind's size; the database keeps two items from
-- sharing a top cell. The server writes this table and nothing else does.
--
-- An order is the command line's way to change what a character owns while there is no
-- website: the server carries it out the next time the character is selected.

CREATE TABLE items (
    id           bigint   GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    character_id integer  NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    vnum         integer  NOT NULL CHECK (vnum > 0),
    count        smallint NOT NULL CHECK (count BETWEEN 1 AND 200),
    cell         smallint NOT NULL CHECK (cell BETWEEN 0 AND 89),
    UNIQUE (character_id, cell)
);

CREATE TABLE item_orders (
    id           bigint   GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    character_id integer  NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    kind         text     NOT NULL CHECK (kind IN ('give', 'take')),
    vnum         integer  CHECK (vnum > 0),
    count        smallint CHECK (count BETWEEN 1 AND 200),
    item_id      bigint,
    CHECK ((kind = 'give') = (vnum IS NOT NULL AND count IS NOT NULL AND item_id IS NULL)),
    CHECK ((kind = 'take') = (vnum IS NULL AND count IS NULL AND item_id IS NOT NULL))
);

CREATE INDEX item_orders_character_id ON item_orders (character_id);
