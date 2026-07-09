ALTER TABLE `ttw_takeover_report`
  ADD COLUMN `report_type` varchar(32) NOT NULL DEFAULT 'other' COMMENT 'report type' AFTER `reported_user_id`,
  ADD KEY `idx_report_type` (`report_type`);

CREATE TABLE IF NOT EXISTS `ttw_takeover_member_activity` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `takeover_id` bigint unsigned NOT NULL COMMENT 'takeover id',
  `user_id` bigint unsigned NOT NULL COMMENT 'user id',
  `action` tinyint unsigned NOT NULL COMMENT '1 join, 2 leave',
  `remark` varchar(100) DEFAULT NULL COMMENT 'member remark',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created at',
  PRIMARY KEY (`id`),
  KEY `idx_takeover_id` (`takeover_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='takeover member activity';

INSERT INTO `ttw_takeover_member_activity` (`takeover_id`, `user_id`, `action`, `remark`, `gmt_create`)
SELECT `takeover_id`, `user_id`, 1, `remark`, `gmt_create`
FROM `ttw_takeover_member`;
