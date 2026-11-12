-- The two unique places, where the thief's gloves are worn, and how long an item worn for
-- a limited time has been worn, in seconds: it counts only while worn in the world, and
-- the item is gone once it reaches its kind's time.
ALTER TABLE items DROP CONSTRAINT items_worn_check,
    ADD CONSTRAINT items_worn_check CHECK (worn IN ('weapon', 'body', 'head', 'feet', 'shield', 'arrow',
                                                    'unique1', 'unique2')),
    ADD COLUMN worn_for integer NOT NULL DEFAULT 0 CHECK (worn_for >= 0);
