SET @convert_admin_permissions_sql := (
  SELECT IF(COUNT(*) = 1, 'UPDATE `ttw_admin_user` SET `role` = ''kook_admin'' WHERE `role` <> ''super_admin'' AND JSON_CONTAINS(COALESCE(`permissions`, JSON_ARRAY()), JSON_QUOTE(''kook:manage''))', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_admin_user' AND COLUMN_NAME = 'permissions'
);
PREPARE convert_admin_permissions_stmt FROM @convert_admin_permissions_sql;
EXECUTE convert_admin_permissions_stmt;
DEALLOCATE PREPARE convert_admin_permissions_stmt;

UPDATE `ttw_admin_user`
SET `role` = 'admin'
WHERE `role` NOT IN ('super_admin', 'kook_admin', 'admin') OR `role` IS NULL OR `role` = '';

SET @drop_admin_permissions_sql := (
  SELECT IF(COUNT(*) = 1, 'ALTER TABLE `ttw_admin_user` DROP COLUMN `permissions`', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_admin_user' AND COLUMN_NAME = 'permissions'
);
PREPARE drop_admin_permissions_stmt FROM @drop_admin_permissions_sql;
EXECUTE drop_admin_permissions_stmt;
DEALLOCATE PREPARE drop_admin_permissions_stmt;
