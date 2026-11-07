-- A helmet and shoes are worn too, one in each place, as the weapon and armour are.
ALTER TABLE items DROP CONSTRAINT items_worn_check,
    ADD CONSTRAINT items_worn_check CHECK (worn IN ('weapon', 'body', 'head', 'feet'));
