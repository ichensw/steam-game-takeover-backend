ALTER TABLE `ttw_takeover_report`
  MODIFY COLUMN `image_url` varchar(512) DEFAULT NULL COMMENT 'report first image url';

SET @add_report_image_urls_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover_report` ADD COLUMN `image_urls` json DEFAULT NULL COMMENT ''report image urls'' AFTER `image_url`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover_report'
    AND COLUMN_NAME = 'image_urls'
);

PREPARE add_report_image_urls_stmt FROM @add_report_image_urls_sql;
EXECUTE add_report_image_urls_stmt;
DEALLOCATE PREPARE add_report_image_urls_stmt;
