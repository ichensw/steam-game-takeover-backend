SET @add_user_credit_score_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_user` ADD COLUMN `credit_score` int unsigned NOT NULL DEFAULT ''100'' COMMENT ''信誉分'' AFTER `is_deleted`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_user'
    AND COLUMN_NAME = 'credit_score'
);

PREPARE add_user_credit_score_stmt FROM @add_user_credit_score_sql;
EXECUTE add_user_credit_score_stmt;
DEALLOCATE PREPARE add_user_credit_score_stmt;

SET @add_report_penalty_score_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover_report` ADD COLUMN `penalty_score` int unsigned NOT NULL DEFAULT ''0'' COMMENT ''扣除分数'' AFTER `image_url`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover_report'
    AND COLUMN_NAME = 'penalty_score'
);

PREPARE add_report_penalty_score_stmt FROM @add_report_penalty_score_sql;
EXECUTE add_report_penalty_score_stmt;
DEALLOCATE PREPARE add_report_penalty_score_stmt;

SET @add_report_handle_note_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover_report` ADD COLUMN `handle_note` varchar(500) DEFAULT NULL COMMENT ''处理说明'' AFTER `penalty_score`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover_report'
    AND COLUMN_NAME = 'handle_note'
);

PREPARE add_report_handle_note_stmt FROM @add_report_handle_note_sql;
EXECUTE add_report_handle_note_stmt;
DEALLOCATE PREPARE add_report_handle_note_stmt;

SET @add_report_handled_by_admin_id_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover_report` ADD COLUMN `handled_by_admin_id` bigint unsigned DEFAULT NULL COMMENT ''处理管理员ID'' AFTER `handle_note`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover_report'
    AND COLUMN_NAME = 'handled_by_admin_id'
);

PREPARE add_report_handled_by_admin_id_stmt FROM @add_report_handled_by_admin_id_sql;
EXECUTE add_report_handled_by_admin_id_stmt;
DEALLOCATE PREPARE add_report_handled_by_admin_id_stmt;

SET @add_report_handled_at_sql := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE `ttw_takeover_report` ADD COLUMN `handled_at` datetime DEFAULT NULL COMMENT ''处理时间'' AFTER `handled_by_admin_id`',
    'SELECT 1'
  )
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ttw_takeover_report'
    AND COLUMN_NAME = 'handled_at'
);

PREPARE add_report_handled_at_stmt FROM @add_report_handled_at_sql;
EXECUTE add_report_handled_at_stmt;
DEALLOCATE PREPARE add_report_handled_at_stmt;
