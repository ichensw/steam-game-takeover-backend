CREATE TABLE IF NOT EXISTS `ttw_kook_voice_session` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `guild_id` varchar(64) NOT NULL COMMENT 'KOOK服务器ID',
  `channel_id` varchar(64) NOT NULL COMMENT 'KOOK语音频道ID',
  `kook_user_id` varchar(64) NOT NULL COMMENT 'KOOK用户ID',
  `joined_at` datetime NOT NULL COMMENT '进入频道时间',
  `exited_at` datetime DEFAULT NULL COMMENT '退出频道时间',
  `duration_seconds` int unsigned NOT NULL DEFAULT 0 COMMENT '已确认使用秒数',
  `status` varchar(16) NOT NULL DEFAULT 'active' COMMENT 'active/closed/abnormal',
  `source` varchar(16) NOT NULL DEFAULT 'event' COMMENT 'event/reconcile',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  KEY `idx_guild_channel_joined` (`guild_id`, `channel_id`, `joined_at`),
  KEY `idx_user_joined` (`kook_user_id`, `joined_at`),
  KEY `idx_exited_at` (`exited_at`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='KOOK语音频道使用会话表';

INSERT INTO `ttw_admin_role_menu` (`role`, `menu_keys`) VALUES
('super_admin', JSON_ARRAY('dashboard', 'takeovers', 'reports', 'users', 'admin-users', 'kook-channels', 'kook-roles', 'kook-members', 'kook-users', 'kook-voice-stats', 'feedbacks', 'announcements', 'settings')),
('kook_admin', JSON_ARRAY('dashboard', 'takeovers', 'reports', 'users', 'kook-channels', 'kook-roles', 'kook-members', 'kook-users', 'kook-voice-stats', 'feedbacks', 'announcements', 'settings'))
ON DUPLICATE KEY UPDATE `menu_keys` = CASE
  WHEN JSON_CONTAINS(`menu_keys`, JSON_QUOTE('kook-voice-stats')) THEN `menu_keys`
  ELSE JSON_ARRAY_APPEND(`menu_keys`, '$', 'kook-voice-stats')
END;
