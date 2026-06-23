SET @add_report_pair_unique_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover_report` ADD UNIQUE KEY `uk_takeover_report_pair` (`takeover_id`, `reporter_user_id`, `reported_user_id`)',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover_report'
    AND INDEX_NAME = 'uk_takeover_report_pair'
);

PREPARE add_report_pair_unique_stmt FROM @add_report_pair_unique_sql;
EXECUTE add_report_pair_unique_stmt;
DEALLOCATE PREPARE add_report_pair_unique_stmt;
