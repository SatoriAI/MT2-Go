-- The skills a character has learnt, by name, each at its level from 1 to 20: the most
-- points raise a skill to. Skill points are not stored: they follow from the level and
-- what is learnt.
CREATE TABLE skills (
    character_id integer  NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    skill        text     NOT NULL CHECK (skill IN ('dragon_swirl', 'enchanted_blade')),
    level        smallint NOT NULL CHECK (level BETWEEN 1 AND 20),
    PRIMARY KEY (character_id, skill)
);
