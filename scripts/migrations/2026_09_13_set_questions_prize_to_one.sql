-- Set challenge question prize to 1 (credited as red asset on correct answer).
-- Applied automatically by the Compose migrate job on next stack start/restart.

-- migrate:up
UPDATE `questions`
SET
  `prize` = 1,
  `updated_at` = NOW();

-- migrate:down
-- Irreversible data backfill: previous prize values are not restored.
