-- Create a least-privilege MySQL user for sitemap-generator-service (SELECT only).
-- migrate runs as root in docker-compose, so GRANT is allowed.

-- migrate:up
CREATE USER IF NOT EXISTS 'sitemap_service'@'%' IDENTIFIED BY 'sitemap_password';
GRANT SELECT ON `metarang_db`.* TO 'sitemap_service'@'%';
FLUSH PRIVILEGES;

-- migrate:down
REVOKE SELECT ON `metarang_db`.* FROM 'sitemap_service'@'%';
DROP USER IF EXISTS 'sitemap_service'@'%';
