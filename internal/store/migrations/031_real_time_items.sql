-- When an item that lasts a stretch of real time from its making ends, worn or not, as the
-- third hand's thirty days; NULL for every other item.
ALTER TABLE items ADD COLUMN ends_at timestamptz;
