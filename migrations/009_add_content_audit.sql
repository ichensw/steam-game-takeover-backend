CREATE TABLE IF NOT EXISTS `ttw_content_audit` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `openid` varchar(64) NOT NULL COMMENT '微信openid',
  `content_type` varchar(32) NOT NULL COMMENT '内容类型',
  `target_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '业务ID',
  `scene` tinyint unsigned NOT NULL COMMENT '微信检测场景',
  `status` varchar(16) NOT NULL COMMENT '检测结果',
  `wx_result` json DEFAULT NULL COMMENT '微信原始结果',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_openid` (`openid`),
  KEY `idx_content` (`content_type`, `target_id`),
  KEY `idx_status` (`status`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='内容安全审核记录表';
