-- Silny przeciwko Orkom, which the valley's orcs made worth rolling, is a kind of bonus too.
ALTER TABLE item_bonuses DROP CONSTRAINT item_bonuses_kind_check,
    ADD CONSTRAINT item_bonuses_kind_check CHECK (kind IN ('max_hp', 'attack_speed', 'critical', 'pierce', 'against_animals',
                                                          'against_orcs', 'average_damage', 'skill_damage', 'average_resist'));
