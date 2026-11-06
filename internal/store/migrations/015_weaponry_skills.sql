-- The rest of the sura's Weaponry that the game has: Finger Strike and Enchanted Armour.
ALTER TABLE skills DROP CONSTRAINT skills_skill_check,
    ADD CONSTRAINT skills_skill_check
        CHECK (skill IN ('finger_strike', 'dragon_swirl', 'enchanted_blade', 'enchanted_armour'));
