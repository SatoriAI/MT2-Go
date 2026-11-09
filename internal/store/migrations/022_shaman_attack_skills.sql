-- The shaman's attacks: Flying Talisman, Shooting Dragon and Dragon Roar of Dragon, and
-- Lightning Throw, Summon Lightning and Lightning Claw of Healing.
ALTER TABLE skills DROP CONSTRAINT skills_skill_check,
    ADD CONSTRAINT skills_skill_check
        CHECK (skill IN ('finger_strike', 'dragon_swirl', 'enchanted_blade', 'enchanted_armour',
            'three_way_cut', 'sword_spin', 'berserk', 'sword_aura',
            'spirit_strike', 'bash', 'stump', 'strong_body', 'sword_strike',
            'ambush', 'fast_attack', 'rolling_dagger',
            'dark_strike', 'flame_strike', 'dark_protection', 'ghost_strike', 'dark_orb',
            'flying_talisman', 'shooting_dragon', 'dragon_roar',
            'lightning_throw', 'summon_lightning', 'lightning_claw'));
