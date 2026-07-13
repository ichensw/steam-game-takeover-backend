CREATE TABLE IF NOT EXISTS `ttw_kook_channel_sort_config` (
  `id` tinyint unsigned NOT NULL COMMENT 'singleton id',
  `enabled` tinyint(1) NOT NULL DEFAULT 0 COMMENT 'automatic sorting enabled',
  `group_ids` json NOT NULL COMMENT 'selected KOOK category ids',
  `schedule_type` varchar(16) NOT NULL DEFAULT 'daily' COMMENT 'daily/weekly/monthly',
  `weekday` tinyint unsigned DEFAULT NULL COMMENT '1=Monday through 7=Sunday',
  `monthday` tinyint unsigned DEFAULT NULL COMMENT '1 through 31',
  `hour` tinyint unsigned NOT NULL DEFAULT 0 COMMENT 'Asia/Shanghai hour',
  `next_run_at` datetime DEFAULT NULL COMMENT 'next scheduled run',
  `lock_token` varchar(64) DEFAULT NULL COMMENT 'shared lease owner',
  `locked_until` datetime DEFAULT NULL COMMENT 'shared lease expiry',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created at',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated at',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='KOOK channel sort configuration';

INSERT INTO `ttw_kook_channel_sort_config`
  (`id`, `enabled`, `group_ids`, `schedule_type`, `hour`)
VALUES
  (1, 0, JSON_ARRAY(), 'daily', 0)
ON DUPLICATE KEY UPDATE `id` = `id`;

CREATE TABLE IF NOT EXISTS `ttw_kook_channel_sort_run` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `trigger` varchar(16) NOT NULL COMMENT 'scheduled/manual',
  `execution_key` varchar(128) DEFAULT NULL COMMENT 'scheduled period idempotency key',
  `range_start` datetime NOT NULL COMMENT 'usage range start',
  `range_end` datetime NOT NULL COMMENT 'usage range end',
  `group_snapshot` longtext NOT NULL COMMENT 'ordered group JSON snapshot',
  `plan_snapshot` longtext NOT NULL COMMENT 'complete move plan JSON snapshot',
  `status` varchar(32) NOT NULL COMMENT 'planning/running/succeeded/failed/rollback_failed',
  `planned_count` int unsigned NOT NULL DEFAULT 0 COMMENT 'planned move count',
  `moved_count` int unsigned NOT NULL DEFAULT 0 COMMENT 'completed move count',
  `error_message` text DEFAULT NULL COMMENT 'execution or rollback error',
  `started_at` datetime NOT NULL COMMENT 'execution start',
  `finished_at` datetime DEFAULT NULL COMMENT 'execution finish',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created at',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated at',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_execution_key` (`execution_key`),
  KEY `idx_status` (`status`),
  KEY `idx_started_at` (`started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='KOOK channel sort execution ledger';
