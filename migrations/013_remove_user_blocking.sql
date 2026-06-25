DROP TABLE IF EXISTS `ttw_block_user`;

SET @drop_is_blocked_sql := (
  SELECT IF(
    COUNT(*) = 1,
    'ALTER TABLE `ttw_user` DROP COLUMN `is_blocked`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND COLUMN_NAME = 'is_blocked'
);
PREPARE drop_is_blocked_stmt FROM @drop_is_blocked_sql;
EXECUTE drop_is_blocked_stmt;
DEALLOCATE PREPARE drop_is_blocked_stmt;
