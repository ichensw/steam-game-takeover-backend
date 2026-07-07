CREATE TABLE IF NOT EXISTS `ttw_announcement` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `title` varchar(80) NOT NULL COMMENT '公告标题',
  `content` varchar(1000) NOT NULL COMMENT '公告内容',
  `image_url` varchar(255) DEFAULT NULL COMMENT '公告图片',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1=启用 2=停用',
  `start_time` datetime NOT NULL COMMENT '开始展示时间',
  `end_time` datetime DEFAULT NULL COMMENT '结束展示时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  KEY `idx_status_time` (`status`, `start_time`, `end_time`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站内公告表';

CREATE TABLE IF NOT EXISTS `ttw_user_announcement_read` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `announcement_id` bigint unsigned NOT NULL COMMENT '公告ID',
  `read_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '已读时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_announcement` (`user_id`, `announcement_id`),
  KEY `idx_announcement_id` (`announcement_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户公告已读表';
