-- Sitemap management metadata and durable generation history.
-- migrate:up
CREATE TABLE IF NOT EXISTS sitemap_generation_runs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  trigger_type VARCHAR(64) NOT NULL,
  target VARCHAR(191) NOT NULL,
  language VARCHAR(16) NULL,
  status VARCHAR(32) NOT NULL,
  started_at DATETIME NULL,
  finished_at DATETIME NULL,
  duration_ms BIGINT NULL,
  url_count INT UNSIGNED NOT NULL DEFAULT 0,
  added_count INT UNSIGNED NOT NULL DEFAULT 0,
  removed_count INT UNSIGNED NOT NULL DEFAULT 0,
  error_message TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY sitemap_runs_status_idx (status),
  KEY sitemap_runs_target_idx (target, language)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sitemap_files (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  language VARCHAR(16) NOT NULL,
  filename VARCHAR(255) NOT NULL,
  file_type VARCHAR(16) NOT NULL,
  url_count INT UNSIGNED NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'available',
  updated_at DATETIME NULL,
  last_run_id BIGINT UNSIGNED NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY sitemap_files_path_unique (language, filename),
  KEY sitemap_files_type_idx (file_type, language)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sitemap_url_changes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_id BIGINT UNSIGNED NOT NULL,
  language VARCHAR(16) NOT NULL,
  target VARCHAR(191) NOT NULL,
  change_type VARCHAR(16) NOT NULL,
  url TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY sitemap_url_changes_run_idx (run_id),
  CONSTRAINT sitemap_url_changes_run_fk FOREIGN KEY (run_id) REFERENCES sitemap_generation_runs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

GRANT SELECT, INSERT, UPDATE, DELETE ON `metarang_db`.`sitemap_generation_runs` TO 'sitemap_service'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON `metarang_db`.`sitemap_files` TO 'sitemap_service'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON `metarang_db`.`sitemap_url_changes` TO 'sitemap_service'@'%';
FLUSH PRIVILEGES;

-- migrate:down
REVOKE SELECT, INSERT, UPDATE, DELETE ON `metarang_db`.`sitemap_url_changes` FROM 'sitemap_service'@'%';
REVOKE SELECT, INSERT, UPDATE, DELETE ON `metarang_db`.`sitemap_files` FROM 'sitemap_service'@'%';
REVOKE SELECT, INSERT, UPDATE, DELETE ON `metarang_db`.`sitemap_generation_runs` FROM 'sitemap_service'@'%';
DROP TABLE IF EXISTS sitemap_url_changes, sitemap_files, sitemap_generation_runs;
