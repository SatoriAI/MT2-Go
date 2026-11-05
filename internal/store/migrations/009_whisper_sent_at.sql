-- A whisper's time is when it was sent, which only the game knows: it is taken before the
-- line reaches its recipient, and the row is saved after. A default of now() would be the
-- time of saving, which puts a quick reply before the line it answers.

ALTER TABLE whispers ALTER COLUMN sent_at DROP DEFAULT;
