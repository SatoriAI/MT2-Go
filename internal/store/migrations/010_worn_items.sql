-- An item is either in an inventory cell or worn, never both: a worn item has no cell.
-- One item is worn in each place, checked at the end of a transaction, so putting on an
-- item where another is worn can move the new one in before the old one out.

ALTER TABLE items
    ALTER COLUMN cell DROP NOT NULL,
    ADD COLUMN worn text CHECK (worn IN ('weapon', 'body')),
    ADD CHECK ((cell IS NULL) <> (worn IS NULL)),
    ADD UNIQUE (character_id, worn) DEFERRABLE INITIALLY DEFERRED;
