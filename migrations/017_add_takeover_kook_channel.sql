SET @add_kook_channel_id = (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover` ADD COLUMN `kook_channel_id` varchar(64) DEFAULT NULL COMMENT ''KOOK语音频道ID'' AFTER `description`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover'
    AND COLUMN_NAME = 'kook_channel_id'
);
PREPARE stmt FROM @add_kook_channel_id;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @add_kook_channel_name = (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover` ADD COLUMN `kook_channel_name` varchar(128) DEFAULT NULL COMMENT ''KOOK语音频道名称'' AFTER `kook_channel_id`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover'
    AND COLUMN_NAME = 'kook_channel_name'
);
PREPARE stmt FROM @add_kook_channel_name;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
