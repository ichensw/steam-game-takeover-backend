CREATE TABLE IF NOT EXISTS `ttw_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `openid` varchar(64) NOT NULL COMMENT '微信小程序openid',
  `unionid` varchar(64) DEFAULT NULL COMMENT '微信unionid',
  `nickname` varchar(32) DEFAULT NULL COMMENT '用户昵称',
  `steam_id` varchar(64) DEFAULT NULL COMMENT 'Steam ID',
  `gender` tinyint unsigned DEFAULT NULL COMMENT '性别：1男，2女',
  `avatar_url` varchar(255) DEFAULT NULL COMMENT '用户头像地址',
  `is_profile_completed` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '资料是否完善：0否，1是',
  `is_admin` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否管理员：0否，1是',
  `is_banned` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否封禁：0否，1是',
  `ban_reason` varchar(255) DEFAULT NULL COMMENT '封禁原因',
  `banned_at` datetime DEFAULT NULL COMMENT '封禁时间',
  `banned_by_admin_id` bigint unsigned DEFAULT NULL COMMENT '封禁管理员ID',
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否删除：0否，1是',
  `credit_score` int unsigned NOT NULL DEFAULT '100' COMMENT '信誉分',
  `last_login_time` datetime DEFAULT NULL COMMENT '最后登录时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  KEY `idx_openid` (`openid`),
  KEY `idx_steam_id` (`steam_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

CREATE TABLE IF NOT EXISTS `ttw_admin_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `username` varchar(64) NOT NULL COMMENT '管理员用户名',
  `password_hash` varchar(255) NOT NULL COMMENT '密码哈希',
  `nickname` varchar(64) DEFAULT NULL COMMENT '管理员昵称',
  `enabled` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '是否启用：0否，1是',
  `last_login_time` datetime DEFAULT NULL COMMENT '最后登录时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台管理员表';

INSERT INTO `ttw_admin_user` (`username`, `password_hash`, `nickname`, `enabled`)
VALUES ('admin', '$2a$10$W3ZjxXFR./UByWLjZvs5z.OIrZcd30i9C2droloE7aTlPENPBRm7u', '超级管理员', 1)
ON DUPLICATE KEY UPDATE `username` = `username`;

CREATE TABLE IF NOT EXISTS `ttw_admin_token` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `admin_user_id` bigint unsigned NOT NULL COMMENT '管理员ID',
  `token_id` varchar(64) NOT NULL COMMENT 'token唯一ID',
  `expires_at` datetime NOT NULL COMMENT '过期时间',
  `is_revoked` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否撤销：0否，1是',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_token_id` (`token_id`),
  KEY `idx_admin_user_id` (`admin_user_id`),
  KEY `idx_expires_at` (`expires_at`),
  KEY `idx_is_revoked` (`is_revoked`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台登录token表';

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
  `remark` varchar(100) DEFAULT NULL COMMENT '加入备注',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_takeover_user` (`takeover_id`, `user_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接龙成员表';

CREATE TABLE IF NOT EXISTS `ttw_takeover_report` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `takeover_id` bigint unsigned NOT NULL COMMENT '接龙ID',
  `reporter_user_id` bigint unsigned NOT NULL COMMENT '举报人用户ID',
  `reported_user_id` bigint unsigned NOT NULL COMMENT '被举报人用户ID',
  `report_content` varchar(500) NOT NULL COMMENT '举报内容',
  `image_url` varchar(512) DEFAULT NULL COMMENT '举报截图',
  `image_urls` json DEFAULT NULL COMMENT '举报截图数组',
  `penalty_score` int unsigned NOT NULL DEFAULT '0' COMMENT '扣除分数',
  `handle_note` varchar(500) DEFAULT NULL COMMENT '处理说明',
  `handled_by_admin_id` bigint unsigned DEFAULT NULL COMMENT '处理管理员ID',
  `handled_at` datetime DEFAULT NULL COMMENT '处理时间',
  `report_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '状态：1待处理，2已处理未扣分，3已处理已扣分',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  KEY `idx_takeover_id` (`takeover_id`),
  KEY `idx_reporter_user_id` (`reporter_user_id`),
  KEY `idx_reported_user_id` (`reported_user_id`),
  UNIQUE KEY `uk_takeover_report_pair` (`takeover_id`, `reporter_user_id`, `reported_user_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接龙举报表';

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

CREATE TABLE IF NOT EXISTS `ttw_app_config` (
  `config_key` varchar(64) NOT NULL COMMENT '配置键',
  `config_value` varchar(255) NOT NULL COMMENT '配置值',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用配置表';

INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES ('publish_takeover_enabled', 'false')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
