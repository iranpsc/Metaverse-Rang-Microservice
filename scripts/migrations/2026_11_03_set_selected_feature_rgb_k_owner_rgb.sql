-- Follow-up to the selected-feature reset. Keep this separate because the
-- original reset migration may already be recorded as applied in an
-- environment.

-- migrate:up
UPDATE `feature_properties`
SET `rgb` = 'k',
    `owner` = 'rgb',
    `updated_at` = NOW()
WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522');

-- migrate:down
-- Irreversible correction; previous values are not reconstructed safely.
SELECT 1 AS irreversible_selected_feature_rgb_owner_update;
