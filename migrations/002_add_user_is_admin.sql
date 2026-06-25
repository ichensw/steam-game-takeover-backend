SET @add_is_admin_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_user` ADD COLUMN `is_admin` tinyint unsigned NOT NULL DEFAULT ''0'' COMMENT ''是否管理员：0否，1是''',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND COLUMN_NAME = 'is_admin'
);

PREPARE add_is_admin_stmt FROM @add_is_admin_sql;
EXECUTE add_is_admin_stmt;
DEALLOCATE PREPARE add_is_admin_stmt;
