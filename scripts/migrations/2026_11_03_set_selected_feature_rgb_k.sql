-- Set the requested RGB status for the five explicitly identified features.
-- This is a follow-up migration because the original reset migration may
-- already be recorded as applied in an environment.

-- migrate:up
UPDATE `feature_properties`
SET `rgb` = 'k',
    `updated_at` = NOW()
WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522');

-- migrate:down
-- Irreversible status correction; the prior per-feature RGB values are not
-- available to reconstruct safely.
SELECT 1 AS irreversible_selected_feature_rgb_update;
