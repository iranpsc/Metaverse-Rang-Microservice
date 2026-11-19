-- Clear wallet addresses from all users.

-- migrate:up
UPDATE `users`
SET `wallet_address` = NULL;

-- migrate:down
-- Irreversible data migration: cleared wallet addresses cannot be restored.
