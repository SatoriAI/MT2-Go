-- A trade swaps items between two characters at once: an item may go to a cell that one
-- the other side gives up stands in until the end of the transaction, so a cell is kept
-- by one item at a time only once the transaction ends.
ALTER TABLE items DROP CONSTRAINT items_character_id_cell_key,
    ADD UNIQUE (character_id, cell) DEFERRABLE INITIALLY DEFERRED;

-- Every trade done, for scam reports and the hunt for duplicated items: who traded, the
-- yang each gave, and each item that changed hands. The log names characters and items by
-- id without a reference, so it outlives a character or an item deleted.
CREATE TABLE trades (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    done_at     timestamptz NOT NULL DEFAULT now(),
    first_id    integer     NOT NULL,
    second_id   integer     NOT NULL,
    first_gold  integer     NOT NULL CHECK (first_gold >= 0),
    second_gold integer     NOT NULL CHECK (second_gold >= 0)
);

CREATE TABLE traded_items (
    trade_id bigint   NOT NULL REFERENCES trades (id) ON DELETE CASCADE,
    item_id  bigint   NOT NULL,
    vnum     integer  NOT NULL,
    count    smallint NOT NULL,
    from_id  integer  NOT NULL
);
