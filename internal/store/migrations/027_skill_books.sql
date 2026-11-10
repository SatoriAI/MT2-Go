-- Books raise a Master, from 20, to M10, and soul stones a Grand Master, from 30, to G10:
-- 39 is the top until Perfect. Each skill keeps when it may next be trained so, as a read
-- waits a while after the last; NULL until it is first trained.
ALTER TABLE skills DROP CONSTRAINT skills_level_check,
    ADD CONSTRAINT skills_level_check CHECK (level BETWEEN 1 AND 39),
    ADD COLUMN next_read timestamptz;
