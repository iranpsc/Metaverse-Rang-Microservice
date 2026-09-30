-- One row is a paid visit window. entered_at is when the fee was paid.
-- The window lasts 24 hours from entered_at. Exit sets exited_at; re-entry in the
-- same window clears exited_at and does not insert another row.
-- Coupon usage is COUNT(*) of rows with that coupon_id.

-- migrate:up
CREATE TABLE `building_entry_sessions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `feature_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `fee_psc_paid` decimal(18,2) NOT NULL DEFAULT 0.00,
  `fee_irr_paid` decimal(18,2) NOT NULL DEFAULT 0.00,
  `coupon_id` bigint(20) unsigned DEFAULT NULL,
  `entered_at` timestamp NOT NULL DEFAULT current_timestamp(),
  `exited_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `building_entry_sessions_active_index` (`feature_id`,`user_id`,`exited_at`),
  KEY `building_entry_sessions_coupon_id_index` (`coupon_id`),
  CONSTRAINT `building_entry_sessions_feature_id_foreign` FOREIGN KEY (`feature_id`) REFERENCES `features` (`id`),
  CONSTRAINT `building_entry_sessions_coupon_id_foreign` FOREIGN KEY (`coupon_id`) REFERENCES `building_entry_coupons` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- migrate:down
DROP TABLE IF EXISTS `building_entry_sessions`;
