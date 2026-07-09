SET @add_can_view_all_takeovers_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_user` ADD COLUMN `can_view_all_takeovers` tinyint unsigned NOT NULL DEFAULT ''0'' COMMENT ''是否可查看全部接龙：0否，1是'' AFTER `is_admin`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND COLUMN_NAME = 'can_view_all_takeovers'
);

PREPARE add_can_view_all_takeovers_stmt FROM @add_can_view_all_takeovers_sql;
EXECUTE add_can_view_all_takeovers_stmt;
DEALLOCATE PREPARE add_can_view_all_takeovers_stmt;
