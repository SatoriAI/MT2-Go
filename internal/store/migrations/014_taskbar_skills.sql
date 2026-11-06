-- A taskbar slot holds an item or a skill learnt, never both. A skill's slot goes with the
-- skill, as changing skill group forgets every skill.
ALTER TABLE taskbar
    ALTER COLUMN item_id DROP NOT NULL,
    ADD COLUMN skill text,
    ADD CHECK ((item_id IS NULL) <> (skill IS NULL)),
    ADD FOREIGN KEY (character_id, skill) REFERENCES skills (character_id, skill) ON DELETE CASCADE;
