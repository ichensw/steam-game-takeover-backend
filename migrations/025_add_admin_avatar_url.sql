SET @add_admin_avatar_url_sql := (
  SELECT IF(COUNT(*) = 0, 'ALTER TABLE `ttw_admin_user` ADD COLUMN `avatar_url` varchar(255) DEFAULT NULL COMMENT ''管理员头像地址'' AFTER `nickname`', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_admin_user' AND COLUMN_NAME = 'avatar_url'
);
PREPARE add_admin_avatar_url_stmt FROM @add_admin_avatar_url_sql;
EXECUTE add_admin_avatar_url_stmt;
DEALLOCATE PREPARE add_admin_avatar_url_stmt;
