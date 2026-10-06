-- Range scans for the presence sweeper (last_seen between 7 and 2 minutes ago).

-- migrate:up
ALTER TABLE `users`
  ADD INDEX `users_last_seen_index` (`last_seen`);

-- migrate:down
ALTER TABLE `users`
  DROP INDEX `users_last_seen_index`;
