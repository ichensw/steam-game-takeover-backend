SET @add_admin_role_sql := (
  SELECT IF(COUNT(*) = 0, 'ALTER TABLE `ttw_admin_user` ADD COLUMN `role` varchar(32) NOT NULL DEFAULT ''admin'' COMMENT ''管理员角色'' AFTER `avatar_url`', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_admin_user' AND COLUMN_NAME = 'role'
);
PREPARE add_admin_role_stmt FROM @add_admin_role_sql;
EXECUTE add_admin_role_stmt;
DEALLOCATE PREPARE add_admin_role_stmt;

UPDATE `ttw_admin_user`
SET `role` = 'super_admin'
WHERE `username` = 'admin';

UPDATE `ttw_admin_user`
SET `role` = 'admin'
WHERE `role` IS NULL OR `role` = '';
