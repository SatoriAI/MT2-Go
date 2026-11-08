-- A shield is worn too, in a place of its own.
ALTER TABLE items DROP CONSTRAINT items_worn_check,
    ADD CONSTRAINT items_worn_check CHECK (worn IN ('weapon', 'body', 'head', 'feet', 'shield'));
