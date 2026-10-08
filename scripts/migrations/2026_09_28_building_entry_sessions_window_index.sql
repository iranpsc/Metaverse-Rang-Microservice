-- Lookup index for the 24-hour paid visit window (feature, user, entered_at).

-- migrate:up
ALTER TABLE `building_entry_sessions`
  ADD INDEX `building_entry_sessions_window_index` (`feature_id`, `user_id`, `entered_at`);

-- migrate:down
ALTER TABLE `building_entry_sessions`
  DROP INDEX `building_entry_sessions_window_index`;
