-- Discount coupons an owner can issue for building entry.
-- code is unique across all features. Real usage is counted from sessions.

-- migrate:up
CREATE TABLE `building_entry_coupons` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `feature_id` bigint(20) unsigned NOT NULL,
  `code` varchar(64) NOT NULL,
  `discount_percentage` tinyint(3) unsigned NOT NULL,
  `max_usage_count` int(10) unsigned NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `building_entry_coupons_code_unique` (`code`),
  KEY `building_entry_coupons_feature_id_index` (`feature_id`),
  CONSTRAINT `building_entry_coupons_feature_id_foreign` FOREIGN KEY (`feature_id`) REFERENCES `features` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- migrate:down
DROP TABLE IF EXISTS `building_entry_coupons`;
