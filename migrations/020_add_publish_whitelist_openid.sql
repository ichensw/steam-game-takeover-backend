SET @add_publish_whitelist_openid = (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_publish_takeover_whitelist` ADD COLUMN `openid` varchar(64) DEFAULT NULL COMMENT ''微信openid'' AFTER `id`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_publish_takeover_whitelist'
    AND COLUMN_NAME = 'openid'
);
PREPARE stmt FROM @add_publish_whitelist_openid;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @modify_publish_whitelist_steam_nullable = (
  SELECT IF(
    IS_NULLABLE = 'NO',
    'ALTER TABLE `ttw_publish_takeover_whitelist` MODIFY COLUMN `steam_id` varchar(64) DEFAULT NULL COMMENT ''Steam ID''',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_publish_takeover_whitelist'
    AND COLUMN_NAME = 'steam_id'
);
PREPARE stmt FROM @modify_publish_whitelist_steam_nullable;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

UPDATE `ttw_publish_takeover_whitelist` w
JOIN `ttw_user` u
  ON u.`steam_id` = w.`steam_id`
 AND u.`is_deleted` = 0
SET w.`openid` = u.`openid`
WHERE w.`openid` IS NULL
  AND w.`steam_id` IS NOT NULL
  AND w.`steam_id` <> '';

SET @add_publish_whitelist_openid_unique = (
  SELECT IF(
    COUNT(*) = 0,
    'CREATE UNIQUE INDEX `uk_openid` ON `ttw_publish_takeover_whitelist` (`openid`)',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_publish_takeover_whitelist'
    AND INDEX_NAME = 'uk_openid'
);
PREPARE stmt FROM @add_publish_whitelist_openid_unique;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
