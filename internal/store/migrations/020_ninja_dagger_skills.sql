-- The ninja's Dagger skills that the game has: Ambush, Fast Attack and Rolling Dagger.
ALTER TABLE skills DROP CONSTRAINT skills_skill_check,
    ADD CONSTRAINT skills_skill_check
        CHECK (skill IN ('finger_strike', 'dragon_swirl', 'enchanted_blade', 'enchanted_armour',
            'three_way_cut', 'sword_spin', 'berserk', 'sword_aura',
            'spirit_strike', 'bash', 'stump', 'strong_body', 'sword_strike',
            'ambush', 'fast_attack', 'rolling_dagger'));
