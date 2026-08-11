ALTER TABLE `ttw_user`
  ADD COLUMN `points_units` bigint unsigned NOT NULL DEFAULT '0' COMMENT '积分，单位为0.1分' AFTER `credit_score`,
  ADD KEY `idx_points_units` (`points_units`);

ALTER TABLE `ttw_takeover`
  ADD COLUMN `points_settled_at` datetime DEFAULT NULL COMMENT '积分结算时间' AFTER `takeover_state`,
  ADD KEY `idx_points_settlement` (`schedule_type`, `is_deleted`, `points_settled_at`, `start_date`, `play_time`);

CREATE TABLE IF NOT EXISTS `ttw_user_point_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `point_delta_units` bigint NOT NULL COMMENT '积分变化，单位为0.1分',
  `point_before_units` bigint unsigned NOT NULL COMMENT '变化前积分，单位为0.1分',
  `point_after_units` bigint unsigned NOT NULL COMMENT '变化后积分，单位为0.1分',
  `reason_type` varchar(32) NOT NULL COMMENT '原因类型',
  `reason` varchar(255) DEFAULT NULL COMMENT '原因说明',
  `takeover_id` bigint unsigned DEFAULT NULL COMMENT '关联接龙ID',
  `operator_admin_id` bigint unsigned DEFAULT NULL COMMENT '操作管理员ID',
  `related_report_id` bigint unsigned DEFAULT NULL COMMENT '关联举报ID',
  `business_key` varchar(128) DEFAULT NULL COMMENT '幂等业务键',
  `effective_at` datetime NOT NULL COMMENT '积分归属时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_key` (`business_key`),
  KEY `idx_user_id` (`user_id`, `id`),
  KEY `idx_takeover_id` (`takeover_id`),
  KEY `idx_effective_at` (`effective_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户积分流水表';
