-- Indexed lookup for "is the searcher following these users?" on /api/search/users.
-- Covers follower_id equality plus following_id IN (...), at most a handful of ids.

-- migrate:up
ALTER TABLE `follows`
  ADD INDEX `follows_follower_following_index` (`follower_id`, `following_id`);

-- migrate:down
ALTER TABLE `follows`
  DROP INDEX `follows_follower_following_index`;
