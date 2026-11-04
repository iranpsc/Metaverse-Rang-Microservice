-- Delete user 494 and records owned by that user.
--
-- The current database row is:
--   email: list.of.iranian.manufacturers@gmail.com
--   code:  hm-2000241
-- The identity predicates on the final user delete make this migration safe if
-- the numeric ID is ever reused before the migration is applied.

-- migrate:up

-- Delete polymorphic records before deleting the user.
DELETE FROM `activity_log`
WHERE `causer_id` = 494
  AND `causer_type` = 'App\\Models\\User';

DELETE FROM `notifications`
WHERE `notifiable_id` = 494
  AND `notifiable_type` = 'App\\Models\\User';

DELETE FROM `personal_access_tokens`
WHERE `tokenable_id` = 494
  AND `tokenable_type` = 'App\\Models\\User';

DELETE FROM `model_has_permissions`
WHERE `model_id` = 494
  AND `model_type` = 'App\\Models\\User';

DELETE FROM `model_has_roles`
WHERE `model_id` = 494
  AND `model_type` = 'App\\Models\\User';

-- Delete records with foreign keys or user-owned records.
DELETE FROM `comment_reports` WHERE `user_id` = 494;
DELETE FROM `interactions` WHERE `user_id` = 494;
DELETE FROM `account_securities` WHERE `user_id` = 494;
DELETE FROM `building_entry_sessions` WHERE `user_id` = 494;
DELETE FROM `buildings` WHERE `user_id` = 494;
DELETE FROM `bulk_message_logs` WHERE `user_id` = 494;
DELETE FROM `buy_feature_requests` WHERE `buyer_id` = 494 OR `seller_id` = 494;
DELETE FROM `chats` WHERE `from_user` = 494 OR `to_user` = 494;
DELETE FROM `children_permissions` WHERE `user_id` = 494;
DELETE FROM `comments` WHERE `user_id` = 494;
DELETE FROM `debts` WHERE `user_id` = 494;
DELETE FROM `dynasties` WHERE `user_id` = 494;
DELETE FROM `family_members` WHERE `user_id` = 494;
DELETE FROM `feature_hourly_profits` WHERE `user_id` = 494;
DELETE FROM `first_orders` WHERE `user_id` = 494;
DELETE FROM `follows` WHERE `follower_id` = 494 OR `following_id` = 494;
DELETE FROM `general_settings` WHERE `user_id` = 494;
DELETE FROM `join_requests` WHERE `from_user` = 494 OR `to_user` = 494;
DELETE FROM `kycs` WHERE `user_id` = 494;
DELETE FROM `level_user` WHERE `user_id` = 494;
DELETE FROM `limited_feature_purchases` WHERE `user_id` = 494;
DELETE FROM `locked_assets` WHERE `user_id` = 494;
DELETE FROM `messages` WHERE `user_id` = 494;
DELETE FROM `notes` WHERE `user_id` = 494;
DELETE FROM `orders` WHERE `user_id` = 494;
DELETE FROM `otps` WHERE `user_id` = 494;
DELETE FROM `payments` WHERE `user_id` = 494;
DELETE FROM `personal_infos` WHERE `user_id` = 494;
DELETE FROM `privacies` WHERE `user_id` = 494;
DELETE FROM `profile_limitations`
WHERE `limiter_user_id` = 494
   OR `limited_user_id` = 494;
DELETE FROM `received_prizes` WHERE `user_id` = 494;
DELETE FROM `recieved_level_prizes` WHERE `user_id` = 494;
DELETE FROM `referral_order_histories` WHERE `user_id` = 494;
DELETE FROM `referrals` WHERE `reference_id` = 494 OR `referrer_id` = 494;
DELETE FROM `reports` WHERE `user_id` = 494;
DELETE FROM `resets` WHERE `user_id` = 494;
DELETE FROM `sell_feature_requests` WHERE `seller_id` = 494;
DELETE FROM `sessions` WHERE `user_id` = 494;
DELETE FROM `settings` WHERE `user_id` = 494;
DELETE FROM `tickets` WHERE `user_id` = 494;
DELETE FROM `trades` WHERE `buyer_id` = 494 OR `seller_id` = 494;
DELETE FROM `transactions` WHERE `user_id` = 494;
DELETE FROM `user_activities` WHERE `user_id` = 494;
DELETE FROM `user_challenge_prizes` WHERE `user_id` = 494;
DELETE FROM `user_events` WHERE `user_id` = 494;
DELETE FROM `user_logs` WHERE `user_id` = 494;
DELETE FROM `user_question_answers` WHERE `user_id` = 494;
DELETE FROM `user_statistics_setting` WHERE `user_id` = 494;
DELETE FROM `user_variables` WHERE `user_id` = 494;
DELETE FROM `wallets` WHERE `user_id` = 494;

-- Preserve other users who referred this user.
UPDATE `users`
SET `referrer_id` = NULL
WHERE `referrer_id` = 494;

DELETE FROM `users`
WHERE `id` = 494
  AND `email` = 'list.of.iranian.manufacturers@gmail.com'
  AND `code` = 'hm-2000241';

-- migrate:down
-- This is an intentional, irreversible data deletion. Restore from a backup
-- if the deleted user or owned records need to be recovered.
SELECT 1 AS irreversible_user_deletion;
