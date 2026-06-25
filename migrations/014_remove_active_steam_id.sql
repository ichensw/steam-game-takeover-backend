SET @drop_active_steam_id_index_sql := (
  SELECT IF(
    COUNT(*) = 1,
    'ALTER TABLE `ttw_user` DROP INDEX `uk_active_steam_id`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND INDEX_NAME = 'uk_active_steam_id'
);
PREPARE drop_active_steam_id_index_stmt FROM @drop_active_steam_id_index_sql;
EXECUTE drop_active_steam_id_index_stmt;
DEALLOCATE PREPARE drop_active_steam_id_index_stmt;

SET @drop_active_steam_id_column_sql := (
  SELECT IF(
    COUNT(*) = 1,
    'ALTER TABLE `ttw_user` DROP COLUMN `active_steam_id`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND COLUMN_NAME = 'active_steam_id'
);
PREPARE drop_active_steam_id_column_stmt FROM @drop_active_steam_id_column_sql;
EXECUTE drop_active_steam_id_column_stmt;
DEALLOCATE PREPARE drop_active_steam_id_column_stmt;
