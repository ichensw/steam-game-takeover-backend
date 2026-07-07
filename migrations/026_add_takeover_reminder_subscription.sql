CREATE TABLE IF NOT EXISTS `ttw_takeover_reminder_subscription` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `takeover_id` bigint unsigned NOT NULL COMMENT '接龙ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `openid` varchar(64) NOT NULL COMMENT '用户openid',
  `remind_at` datetime NOT NULL COMMENT '提醒时间',
  `play_at` datetime NOT NULL COMMENT '开局时间',
  `send_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '发送状态：1=待发送，2=已发送，3=发送失败',
  `send_error` varchar(255) DEFAULT NULL COMMENT '发送失败原因',
  `sent_at` datetime DEFAULT NULL COMMENT '发送时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_takeover_user_play_at` (`takeover_id`, `user_id`, `play_at`),
  KEY `idx_takeover_id` (`takeover_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_remind_state` (`send_state`, `remind_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接龙开局订阅提醒表';
