-- The bonuses an item carries, at most seven, each by its slot: rolled as it dropped, they
-- stay with the item row through trades and upgrades and go with it. Each is stored by
-- name; percentages and hit points alike are whole numbers, a weapon's weak side of its
-- pair below zero.
CREATE TABLE item_bonuses (
    item_id bigint   NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    slot    smallint NOT NULL CHECK (slot BETWEEN 0 AND 6),
    kind    text     NOT NULL CHECK (kind IN ('max_hp', 'attack_speed', 'critical', 'pierce', 'against_animals',
                                              'average_damage', 'skill_damage', 'average_resist')),
    value   smallint NOT NULL CHECK (value <> 0),
    PRIMARY KEY (item_id, slot)
);
