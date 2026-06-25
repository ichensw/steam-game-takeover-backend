SET @add_kook_invite_url = (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover` ADD COLUMN `kook_invite_url` varchar(255) DEFAULT NULL COMMENT ''KOOK频道邀请链接'' AFTER `kook_channel_name`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover'
    AND COLUMN_NAME = 'kook_invite_url'
);
PREPARE stmt FROM @add_kook_invite_url;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
