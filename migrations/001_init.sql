CREATE TABLE IF NOT EXISTS `ttw_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `openid` varchar(64) NOT NULL COMMENT '微信小程序openid',
  `unionid` varchar(64) DEFAULT NULL COMMENT '微信unionid',
  `nickname` varchar(32) DEFAULT NULL COMMENT '用户昵称',
  `steam_id` varchar(64) DEFAULT NULL COMMENT 'Steam ID',
  `gender` tinyint unsigned DEFAULT NULL COMMENT '性别：1男，2女',
  `avatar_url` varchar(255) DEFAULT NULL COMMENT '用户头像地址',
  `is_profile_completed` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '资料是否完善：0否，1是',
  `is_blocked` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否被拉黑：0否，1是',
  `last_login_time` datetime DEFAULT NULL COMMENT '最后登录时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_openid` (`openid`),
  KEY `idx_steam_id` (`steam_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

CREATE TABLE IF NOT EXISTS `ttw_takeover` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `creator_user_id` bigint unsigned NOT NULL COMMENT '创建人用户ID',
  `title` varchar(50) NOT NULL COMMENT '接龙标题',
  `participant_limit` int unsigned NOT NULL COMMENT '人数上限',
  `schedule_type` tinyint unsigned NOT NULL COMMENT '时间类型：1指定日期，2每天固定，3日期范围',
  `start_date` date DEFAULT NULL COMMENT '开始日期',
  `end_date` date DEFAULT NULL COMMENT '结束日期',
  `play_time` time NOT NULL COMMENT '固定时间',
  `description` varchar(500) DEFAULT NULL COMMENT '接龙介绍',
  `takeover_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '接龙状态：1正常，2已关闭',
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否删除：0否，1是',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  KEY `idx_creator_user_id` (`creator_user_id`),
  KEY `idx_schedule` (`schedule_type`, `start_date`, `end_date`, `play_time`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接龙表';

CREATE TABLE IF NOT EXISTS `ttw_takeover_member` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `takeover_id` bigint unsigned NOT NULL COMMENT '接龙ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `member_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '成员状态：1已加入，2已退出',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_takeover_user` (`takeover_id`, `user_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接龙成员表';

CREATE TABLE IF NOT EXISTS `ttw_block_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint unsigned NOT NULL COMMENT '被拉黑用户ID',
  `openid` varchar(64) NOT NULL COMMENT '被拉黑用户openid',
  `nickname_snapshot` varchar(32) DEFAULT NULL COMMENT '拉黑时昵称快照',
  `steam_id_snapshot` varchar(64) DEFAULT NULL COMMENT '拉黑时Steam ID快照',
  `block_reason` varchar(255) DEFAULT NULL COMMENT '拉黑原因',
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否解除拉黑：0否，1是',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`),
  KEY `idx_openid` (`openid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户拉黑表';

CREATE TABLE IF NOT EXISTS `ttw_admin_operate_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `operate_type` varchar(32) NOT NULL COMMENT '操作类型',
  `target_type` varchar(32) NOT NULL COMMENT '目标类型：takeover/user',
  `target_id` bigint unsigned NOT NULL COMMENT '目标ID',
  `operate_content` varchar(1000) DEFAULT NULL COMMENT '操作内容',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_target` (`target_type`, `target_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员操作日志表';
