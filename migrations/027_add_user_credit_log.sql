CREATE TABLE IF NOT EXISTS `ttw_user_credit_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `score_delta` int NOT NULL COMMENT '分数变化',
  `score_before` int unsigned NOT NULL COMMENT '变化前分数',
  `score_after` int unsigned NOT NULL COMMENT '变化后分数',
  `reason_type` varchar(32) NOT NULL COMMENT '原因类型',
  `reason` varchar(255) DEFAULT NULL COMMENT '原因说明',
  `operator_admin_id` bigint unsigned DEFAULT NULL COMMENT '操作管理员ID',
  `related_report_id` bigint unsigned DEFAULT NULL COMMENT '关联举报ID',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户信誉分流水表';
