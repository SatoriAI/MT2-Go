-- The warrior's Body skills that the game has: Three-Way Cut, Sword Spin, Berserk and
-- Sword Aura.
ALTER TABLE skills DROP CONSTRAINT skills_skill_check,
    ADD CONSTRAINT skills_skill_check
        CHECK (skill IN ('finger_strike', 'dragon_swirl', 'enchanted_blade', 'enchanted_armour',
            'three_way_cut', 'sword_spin', 'berserk', 'sword_aura'));
