CREATE TABLE IF NOT EXISTS `ttw_user_feedback` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint unsigned NOT NULL COMMENT '提交反馈的用户ID',
  `feedback_type` varchar(32) NOT NULL COMMENT '反馈类型：suggestion/problem/experience/other',
  `content` varchar(500) NOT NULL COMMENT '反馈内容',
  `contact` varchar(100) NOT NULL DEFAULT '' COMMENT '联系方式，选填',
  `images` json DEFAULT NULL COMMENT '反馈图片URL数组',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1=待采纳 2=已采纳 3=不理睬',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_status_created_at` (`status`, `created_at`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户意见反馈表';
