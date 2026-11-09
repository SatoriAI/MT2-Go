-- The ninja's Archery: Repetitive Shot, Arrow Shower, Fire Arrow, Feather Walk and Poison
-- Arrow.
ALTER TABLE skills DROP CONSTRAINT skills_skill_check,
    ADD CONSTRAINT skills_skill_check
        CHECK (skill IN ('finger_strike', 'dragon_swirl', 'enchanted_blade', 'enchanted_armour',
            'three_way_cut', 'sword_spin', 'berserk', 'sword_aura',
            'spirit_strike', 'bash', 'stump', 'strong_body', 'sword_strike',
            'ambush', 'fast_attack', 'rolling_dagger',
            'repetitive_shot', 'arrow_shower', 'fire_arrow', 'feather_walk', 'poison_arrow',
            'dark_strike', 'flame_strike', 'dark_protection', 'ghost_strike', 'dark_orb',
            'flying_talisman', 'shooting_dragon', 'dragon_roar',
            'lightning_throw', 'summon_lightning', 'lightning_claw',
            'blessing', 'reflection', 'dragons_help', 'heal', 'swiftness', 'attack_up'));
