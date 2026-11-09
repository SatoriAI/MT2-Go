-- Arrows are worn too, in a place of their own, for the bow to shoot.
ALTER TABLE items DROP CONSTRAINT items_worn_check,
    ADD CONSTRAINT items_worn_check CHECK (worn IN ('weapon', 'body', 'head', 'feet', 'shield', 'arrow'));
