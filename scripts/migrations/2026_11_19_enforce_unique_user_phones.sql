-- migrate:up
-- Keep each phone on the account with the lowest user ID.
UPDATE users AS duplicate_user
JOIN (
    SELECT phone, MIN(id) AS first_user_id
    FROM users
    WHERE phone IS NOT NULL
    GROUP BY phone
    HAVING COUNT(*) > 1
) AS duplicate_phones
    ON duplicate_user.phone = duplicate_phones.phone
    AND duplicate_user.id <> duplicate_phones.first_user_id
SET duplicate_user.phone = NULL,
    duplicate_user.phone_verified_at = NULL;

ALTER TABLE users ADD UNIQUE KEY users_phone_unique (phone);

-- migrate:down
-- Dropping the index does not restore cleared phone numbers or verification timestamps.
ALTER TABLE users DROP INDEX users_phone_unique;
