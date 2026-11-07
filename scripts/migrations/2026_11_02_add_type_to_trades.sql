-- Adds the discriminator used by the feature trade-history API.
-- Existing records are initialized with the schema default to preserve the
-- behavior of databases created from scripts/schema.sql.
-- migrate:up
SET @has_trade_type := (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'trades'
    AND column_name = 'type'
);
SET @trade_type_ddl := IF(
  @has_trade_type = 0,
  'ALTER TABLE `trades` ADD COLUMN `type` VARCHAR(191) NOT NULL DEFAULT ''land'' AFTER `seller_id`',
  'SELECT 1'
);
PREPARE add_trade_type FROM @trade_type_ddl;
EXECUTE add_trade_type;
DEALLOCATE PREPARE add_trade_type;

-- migrate:down
ALTER TABLE `trades`
  DROP COLUMN `type`;
