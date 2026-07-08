CREATE TABLE IF NOT EXISTS `ttw_kook_member` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `guild_id` varchar(64) NOT NULL COMMENT 'KOOK服务器ID',
  `kook_user_id` varchar(64) NOT NULL COMMENT 'KOOK用户ID',
  `username` varchar(64) DEFAULT NULL COMMENT 'KOOK用户名',
  `nickname` varchar(64) DEFAULT NULL COMMENT 'KOOK服务器昵称',
  `identify_num` varchar(16) DEFAULT NULL COMMENT 'KOOK用户名认证数字',
  `avatar_url` varchar(255) DEFAULT NULL COMMENT '头像地址',
  `is_bot` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否机器人',
  `member_status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '成员状态：1在服 2已退出',
  `joined_at` datetime DEFAULT NULL COMMENT '加入服务器时间',
  `exited_at` datetime DEFAULT NULL COMMENT '退出服务器时间',
  `is_blacklisted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否已加入KOOK黑名单',
  `blacklist_reason` varchar(255) DEFAULT NULL COMMENT '拉黑原因',
  `blacklisted_at` datetime DEFAULT NULL COMMENT '拉黑时间',
  `remark` varchar(255) DEFAULT NULL COMMENT '后台备注',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_guild_user` (`guild_id`, `kook_user_id`),
  KEY `idx_kook_user_id` (`kook_user_id`),
  KEY `idx_member_status` (`member_status`),
  KEY `idx_is_blacklisted` (`is_blacklisted`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='KOOK成员表';

INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES ('kook_verify_token', '')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
