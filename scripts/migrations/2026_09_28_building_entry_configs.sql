-- Entry fee and level-scope settings for a completed building on a feature.
-- One row per feature. Guests pay fee_psc and fee_irr; the owner does not.

-- migrate:up
CREATE TABLE `building_entry_configs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `feature_id` bigint(20) unsigned NOT NULL,
  `fee_psc` decimal(18,2) NOT NULL DEFAULT 0.00,
  `fee_irr` decimal(18,2) NOT NULL DEFAULT 0.00,
  `about` varchar(1000) NOT NULL DEFAULT '',
  `level_scope_type` enum('exact','and_upper','and_lower') DEFAULT NULL,
  `level_slug` varchar(255) DEFAULT NULL,
  `is_active` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `building_entry_configs_feature_id_unique` (`feature_id`),
  CONSTRAINT `building_entry_configs_feature_id_foreign` FOREIGN KEY (`feature_id`) REFERENCES `features` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- migrate:down
DROP TABLE IF EXISTS `building_entry_configs`;
