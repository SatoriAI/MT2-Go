-- The sura's Black Magic skills that the game has: Dark Strike, Flame Strike, Dark
-- Protection, Ghost Strike and Dark Orb.
ALTER TABLE skills DROP CONSTRAINT skills_skill_check,
    ADD CONSTRAINT skills_skill_check
        CHECK (skill IN ('finger_strike', 'dragon_swirl', 'enchanted_blade', 'enchanted_armour',
            'three_way_cut', 'sword_spin', 'berserk', 'sword_aura',
            'spirit_strike', 'bash', 'stump', 'strong_body', 'sword_strike',
            'ambush', 'fast_attack', 'rolling_dagger',
            'dark_strike', 'flame_strike', 'dark_protection', 'ghost_strike', 'dark_orb'));
