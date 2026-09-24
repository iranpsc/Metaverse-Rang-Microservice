-- Unique constraint for one interaction per user/likeable pair.
-- Deduplicate existing rows (keep MAX(id)) before adding the unique key.
-- Apply to existing databases that were created from an older schema.sql.

-- migrate:up
DELETE i FROM `interactions` i
INNER JOIN (
  SELECT `user_id`, `likeable_type`, `likeable_id`, MAX(`id`) AS `max_id`
  FROM `interactions`
  GROUP BY `user_id`, `likeable_type`, `likeable_id`
  HAVING COUNT(*) > 1
) d ON i.`user_id` = d.`user_id`
  AND i.`likeable_type` = d.`likeable_type`
  AND i.`likeable_id` = d.`likeable_id`
  AND i.`id` < d.`max_id`;

ALTER TABLE `interactions`
  ADD UNIQUE KEY `interactions_user_likeable_unique` (`user_id`, `likeable_type`, `likeable_id`);

-- migrate:down
ALTER TABLE `interactions`
  DROP INDEX `interactions_user_likeable_unique`;
