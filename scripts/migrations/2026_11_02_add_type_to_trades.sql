-- Adds the discriminator used by the feature trade-history API.
-- Existing records are initialized with the schema default to preserve the
-- behavior of databases created from scripts/schema.sql.
-- migrate:up
ALTER TABLE `trades`
  ADD COLUMN IF NOT EXISTS `type` VARCHAR(191) NOT NULL DEFAULT 'land' AFTER `seller_id`;

-- migrate:down
ALTER TABLE `trades`
  DROP COLUMN IF EXISTS `type`;
