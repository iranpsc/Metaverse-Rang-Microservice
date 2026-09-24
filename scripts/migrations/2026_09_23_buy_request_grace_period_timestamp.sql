-- buy_feature_requests.requested_grace_period was a Laravel varchar(191).
-- Go writes a datetime and scans sql.NullTime; VARCHAR comes back as []byte and
-- list queries silently dropped those rows. Store a real timestamp instead.

-- migrate:up
UPDATE `buy_feature_requests`
SET `requested_grace_period` = NULL
WHERE `requested_grace_period` IS NOT NULL
  AND `requested_grace_period` NOT REGEXP '^[0-9]{4}-[0-9]{2}-[0-9]{2}';

ALTER TABLE `buy_feature_requests`
  MODIFY `requested_grace_period` timestamp NULL DEFAULT NULL;

-- migrate:down
ALTER TABLE `buy_feature_requests`
  MODIFY `requested_grace_period` varchar(191) DEFAULT NULL;
