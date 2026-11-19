-- migrate:up
-- Resolve any existing duplicate non-NULL phone values before applying this index.
-- Do not automatically choose which account keeps a number.
ALTER TABLE users ADD UNIQUE KEY users_phone_unique (phone);

-- migrate:down
ALTER TABLE users DROP INDEX users_phone_unique;
