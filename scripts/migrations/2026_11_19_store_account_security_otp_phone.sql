-- migrate:up
ALTER TABLE otps ADD COLUMN phone VARCHAR(191) DEFAULT NULL AFTER code;

-- migrate:down
ALTER TABLE otps DROP COLUMN phone;
