-- Reset five explicitly identified features to pristine system inventory.
--
-- The property IDs are the scope authority.  We deliberately resolve the
-- target owner by its stable code and assert that it remains user ID 1: map
-- statistics still use ID 1 while marketplace purchases use this code.
--
-- This is intentionally a one-way data repair. Geometry is retained because it
-- defines the land itself. Feature images and trades are reset as requested;
-- image objects in storage-service are not reachable from this SQL migration
-- and must be garbage-collected by that service if desired.
--
-- Safety preflight: pending buy requests / locked assets must first be
-- cancelled and refunded through commercial-service. Paid building-entry
-- sessions are retained rather than erased. Hourly-profit rows are deleted as
-- part of this explicitly destructive reset, including accrued amounts.
-- The temporary guard table is left in place only when a preflight assertion
-- fails, making the failed migration safe to rerun after reconciliation.

-- migrate:up

CREATE TABLE IF NOT EXISTS `_feature_reset_guard_20261103` (
  `assertion` TINYINT NOT NULL,
  CONSTRAINT `feature_reset_guard_20261103_assertion` CHECK (`assertion` = 1)
) ENGINE=InnoDB;

INSERT INTO `_feature_reset_guard_20261103` (`assertion`)
SELECT CASE
  WHEN
    (SELECT COUNT(*)
       FROM `feature_properties`
      WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')) = 5
    AND
    (SELECT COUNT(DISTINCT `feature_id`)
       FROM `feature_properties`
      WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')) = 5
    AND NOT EXISTS (
      SELECT 1
      FROM `feature_properties`
      WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
        AND `karbari` NOT IN ('m', 't', 'a')
    )
    AND EXISTS (
      SELECT 1 FROM `users` WHERE `id` = 1 AND `code` = 'hm-2000000'
    )
    AND NOT EXISTS (
      SELECT 1
      FROM `buy_feature_requests`
      WHERE `feature_id` IN (
        SELECT `feature_id` FROM `feature_properties`
        WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
      )
        AND `status` = 0
        AND `deleted_at` IS NULL
    )
    AND NOT EXISTS (
      SELECT 1
      FROM `locked_assets`
      WHERE `feature_id` IN (
        SELECT `feature_id` FROM `feature_properties`
        WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
      )
    )
    AND NOT EXISTS (
      SELECT 1
      FROM `building_entry_sessions`
      WHERE `feature_id` IN (
        SELECT `feature_id` FROM `feature_properties`
        WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
      )
        AND (`fee_psc_paid` <> 0 OR `fee_irr_paid` <> 0)
    )
  THEN 1
  ELSE 0
END;

-- Delete reset-owned dependent state before changing the feature owner.
DELETE FROM `building_entry_sessions`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE FROM `building_entry_coupons`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE FROM `building_entry_configs`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE FROM `buildings`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE FROM `feature_hourly_profits`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

-- Remove the database image attachments. The underlying storage-service files
-- are intentionally not deleted here because SQL has no authority to remove
-- external objects.
DELETE FROM `images`
WHERE `imageable_type` = 'App\\Models\\Feature'
  AND `imageable_id` IN (
    SELECT `feature_id` FROM `feature_properties`
    WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
  );

-- A trade has dependent commission and polymorphic transaction ledger rows.
-- Delete them first so no record continues to reference a removed trade.
DELETE c
FROM `comissions` AS c
INNER JOIN `trades` AS t ON t.`id` = c.`trade_id`
WHERE t.`feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE tx
FROM `transactions` AS tx
INNER JOIN `trades` AS t
  ON t.`id` = tx.`payable_id`
 AND tx.`payable_type` = 'App\\Models\\Trade'
WHERE t.`feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE FROM `trades`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

-- `limited_feature_purchases` has no feature_id in the deployed schema: it
-- records a user's purchase count for an entire campaign. Do not delete it
-- here, because there is no safe way to attribute a row to one of these five
-- features without altering the user's unrelated campaign eligibility.

DELETE FROM `locked_features`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE FROM `sell_feature_requests`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

DELETE FROM `buy_feature_requests`
WHERE `feature_id` IN (
  SELECT `feature_id` FROM `feature_properties`
  WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522')
);

UPDATE `feature_properties`
SET
  `rgb` = CASE `karbari`
    WHEN 'm' THEN 'c'
    WHEN 't' THEN 'j'
    WHEN 'a' THEN 'q'
  END,
  `owner` = '',
  `label` = '',
  `price_psc` = '0',
  `price_irr` = '0',
  `minimum_price_percentage` = 100,
  `updated_at` = NOW()
WHERE `id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522');

UPDATE `features` AS f
INNER JOIN `feature_properties` AS fp ON fp.`feature_id` = f.`id`
INNER JOIN `users` AS system_user
  ON system_user.`id` = 1 AND system_user.`code` = 'hm-2000000'
SET f.`owner_id` = system_user.`id`,
    f.`updated_at` = NOW()
WHERE fp.`id` IN ('ar-1674', 'ar-1675', 'ar-1676', 'ar-1600', 'ar-1522');

DROP TABLE `_feature_reset_guard_20261103`;

-- migrate:down
-- Irreversible repair: former ownership and deleted reset-owned state cannot
-- be reconstructed safely. Geometry and external image-file history were
-- intentionally preserved.
SELECT 1 AS irreversible_selected_feature_reset;
