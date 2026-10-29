-- Merge the accidentally-created user 710 into the original user 600.
--
-- This migration is deliberately limited to the known bad source record. Every
-- UPDATE and the DELETE below require the expected identity values, so another
-- user occupying either ID is left untouched. It is also safe to run after the
-- repair has already been performed: user 710 no longer matches and all
-- statements become no-ops.
--
-- The migration runner executes statements individually, so the ordering below
-- avoids the users.email and users.wallet_address unique-key conflicts while
-- preserving the source values.

-- migrate:up

UPDATE `account_securities` AS child
JOIN `users` AS source ON source.`id` = 710
JOIN `users` AS target ON target.`id` = 600
SET child.`user_id` = target.`id`
WHERE child.`user_id` = source.`id`
  AND source.`name` = 'parsa'
  AND source.`email` = 'parsabr80@gmail.com'
  AND source.`code` = 'hm-2000491'
  AND source.`wallet_address` = '0xf608017c284a4aa0489842cb042bb47708bafcb7';

UPDATE `buildings` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `building_entry_sessions` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `children_permissions` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `comment_reports` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `comments` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `debts` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `dynasties` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `family_members` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `feature_hourly_profits` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `first_orders` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `general_settings` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `interactions` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `kycs` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `level_user` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `limited_feature_purchases` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `locked_assets` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `messages` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `notes` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `orders` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `otps` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `payments` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `personal_infos` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `privacies` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `received_prizes` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `recieved_level_prizes` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `referral_order_histories` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `resets` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `sessions` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `settings` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `transactions` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `user_activities` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `user_challenge_prizes` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `user_logs` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `user_question_answers` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `user_statistics_setting` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `user_variables` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `wallets` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`user_id`=target.`id` WHERE child.`user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';

UPDATE `buy_feature_requests` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`buyer_id`=target.`id` WHERE child.`buyer_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `buy_feature_requests` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`seller_id`=target.`id` WHERE child.`seller_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `chats` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`from_user`=target.`id` WHERE child.`from_user`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `chats` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`to_user`=target.`id` WHERE child.`to_user`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `follows` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`follower_id`=target.`id` WHERE child.`follower_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `follows` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`following_id`=target.`id` WHERE child.`following_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `join_requests` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`from_user`=target.`id` WHERE child.`from_user`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `join_requests` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`to_user`=target.`id` WHERE child.`to_user`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `referrals` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`reference_id`=target.`id` WHERE child.`reference_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `referrals` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`referrer_id`=target.`id` WHERE child.`referrer_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `sell_feature_requests` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`seller_id`=target.`id` WHERE child.`seller_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `trades` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`buyer_id`=target.`id` WHERE child.`buyer_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `trades` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`seller_id`=target.`id` WHERE child.`seller_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `profile_limitations` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`limiter_user_id`=target.`id` WHERE child.`limiter_user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `profile_limitations` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`limited_user_id`=target.`id` WHERE child.`limited_user_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `users` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`referrer_id`=target.`id` WHERE child.`referrer_id`=source.`id` AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';

UPDATE `notifications` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`notifiable_id`=target.`id` WHERE child.`notifiable_id`=source.`id` AND child.`notifiable_type`='App\\Models\\User' AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `personal_access_tokens` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`tokenable_id`=target.`id` WHERE child.`tokenable_id`=source.`id` AND child.`tokenable_type`='App\\Models\\User' AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `model_has_roles` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`model_id`=target.`id` WHERE child.`model_id`=source.`id` AND child.`model_type`='App\\Models\\User' AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';
UPDATE `model_has_permissions` AS child JOIN `users` AS source ON source.`id`=710 JOIN `users` AS target ON target.`id`=600 SET child.`model_id`=target.`id` WHERE child.`model_id`=source.`id` AND child.`model_type`='App\\Models\\User' AND source.`name`='parsa' AND source.`email`='parsabr80@gmail.com' AND source.`code`='hm-2000491' AND source.`wallet_address`='0xf608017c284a4aa0489842cb042bb47708bafcb7';

UPDATE `users` AS target
JOIN `users` AS source ON source.`id` = 710
SET target.`name` = source.`name`,
    target.`email` = NULL,
    target.`wallet_address` = NULL,
    target.`nonce` = source.`nonce`,
    target.`phone` = source.`phone`,
    target.`phone_verified_at` = source.`phone_verified_at`,
    target.`email_verified_at` = source.`email_verified_at`,
    target.`ip` = source.`ip`,
    target.`password` = source.`password`,
    target.`referal_link` = source.`referal_link`,
    target.`referrer_id` = CASE WHEN source.`referrer_id` = 710 THEN 600 ELSE source.`referrer_id` END,
    target.`code` = source.`code`,
    target.`score` = source.`score`,
    target.`last_seen` = source.`last_seen`,
    target.`remember_token` = source.`remember_token`,
    target.`created_at` = source.`created_at`,
    target.`updated_at` = source.`updated_at`,
    target.`access_token` = source.`access_token`,
    target.`refresh_token` = source.`refresh_token`,
    target.`token_type` = source.`token_type`,
    target.`expires_in` = source.`expires_in`
WHERE target.`id` = 600
  AND source.`name` = 'parsa'
  AND source.`email` = 'parsabr80@gmail.com'
  AND source.`code` = 'hm-2000491'
  AND source.`wallet_address` = '0xf608017c284a4aa0489842cb042bb47708bafcb7';

DELETE FROM `users`
WHERE `id` = 710
  AND `name` = 'parsa'
  AND `email` = 'parsabr80@gmail.com'
  AND `code` = 'hm-2000491'
  AND `wallet_address` = '0xf608017c284a4aa0489842cb042bb47708bafcb7';

UPDATE `users`
SET `email` = 'parsabr80@gmail.com',
    `wallet_address` = '0xf608017c284a4aa0489842cb042bb47708bafcb7'
WHERE `id` = 600
  AND `email` IS NULL
  AND `wallet_address` IS NULL
  AND `name` = 'parsa'
  AND `code` = 'hm-2000491';

-- migrate:down
-- This is an intentional, irreversible data merge. The deleted source row and
-- its former ownership cannot be reconstructed without a database backup.
SELECT 1 AS irreversible_user_merge;
