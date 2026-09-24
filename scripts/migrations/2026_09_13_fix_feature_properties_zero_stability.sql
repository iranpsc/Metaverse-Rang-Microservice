-- Backfill feature_properties.stability when it is 0.
-- Business rule: stability = area * density.
-- Applied automatically by the Compose migrate job on next stack start/restart.

-- migrate:up
UPDATE `feature_properties`
SET
  `stability` = `area` * `density`,
  `updated_at` = NOW()
WHERE `stability` = 0
  AND (`area` * `density`) <> 0;

-- migrate:down
-- Irreversible data backfill: previous zero values are not restored.
