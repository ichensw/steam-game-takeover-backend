SET @add_user_is_deleted_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_user` ADD COLUMN `is_deleted` tinyint unsigned NOT NULL DEFAULT ''0'' COMMENT ''是否删除：0否，1是'' AFTER `is_admin`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND COLUMN_NAME = 'is_deleted'
);

PREPARE add_user_is_deleted_stmt FROM @add_user_is_deleted_sql;
EXECUTE add_user_is_deleted_stmt;
DEALLOCATE PREPARE add_user_is_deleted_stmt;

SET @drop_user_openid_unique_sql := (
  SELECT IF(
    COUNT(*) > 0,
    'ALTER TABLE `ttw_user` DROP INDEX `uk_openid`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND INDEX_NAME = 'uk_openid'
);

PREPARE drop_user_openid_unique_stmt FROM @drop_user_openid_unique_sql;
EXECUTE drop_user_openid_unique_stmt;
DEALLOCATE PREPARE drop_user_openid_unique_stmt;

SET @add_user_openid_index_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'CREATE INDEX `idx_openid` ON `ttw_user` (`openid`)',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND INDEX_NAME = 'idx_openid'
);

PREPARE add_user_openid_index_stmt FROM @add_user_openid_index_sql;
EXECUTE add_user_openid_index_stmt;
DEALLOCATE PREPARE add_user_openid_index_stmt;
