CREATE TABLE IF NOT EXISTS `ttw_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `openid` varchar(64) NOT NULL COMMENT '寰俊灏忕▼搴弌penid',
  `unionid` varchar(64) DEFAULT NULL COMMENT '寰俊unionid',
  `nickname` varchar(32) DEFAULT NULL COMMENT '鐢ㄦ埛鏄电О',
  `steam_id` varchar(64) DEFAULT NULL COMMENT 'Steam ID',
  `gender` tinyint unsigned DEFAULT NULL COMMENT '鎬у埆锛?鐢凤紝2濂?,
  `avatar_url` varchar(255) DEFAULT NULL COMMENT '鐢ㄦ埛澶村儚鍦板潃',
  `is_profile_completed` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '璧勬枡鏄惁瀹屽杽锛?鍚︼紝1鏄?,
  `is_admin` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '鏄惁绠＄悊鍛橈細0鍚︼紝1鏄?,
  `is_banned` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '鏄惁灏佺锛?鍚︼紝1鏄?,
  `ban_reason` varchar(255) DEFAULT NULL COMMENT '灏佺鍘熷洜',
  `banned_at` datetime DEFAULT NULL COMMENT '灏佺鏃堕棿',
  `banned_by_admin_id` bigint unsigned DEFAULT NULL COMMENT '灏佺绠＄悊鍛業D',
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '鏄惁鍒犻櫎锛?鍚︼紝1鏄?,
  `credit_score` int unsigned NOT NULL DEFAULT '100' COMMENT '淇¤獕鍒?,
  `last_login_time` datetime DEFAULT NULL COMMENT '鏈€鍚庣櫥褰曟椂闂?,
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  KEY `idx_openid` (`openid`),
  KEY `idx_steam_id` (`steam_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鐢ㄦ埛琛?;

CREATE TABLE IF NOT EXISTS `ttw_admin_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `username` varchar(64) NOT NULL COMMENT '管理员用户名',
  `password_hash` varchar(255) NOT NULL COMMENT '密码哈希',
  `nickname` varchar(64) DEFAULT NULL COMMENT '管理员昵称',
  `avatar_url` varchar(255) DEFAULT NULL COMMENT '管理员头像地址',
  `role` varchar(32) NOT NULL DEFAULT 'admin' COMMENT '管理员角色',
  `enabled` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '是否启用：0否，1是',
  `last_login_time` datetime DEFAULT NULL COMMENT '最后登录时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台管理员表';

INSERT INTO `ttw_admin_user` (`username`, `password_hash`, `nickname`, `role`, `enabled`)
VALUES ('admin', '$2a$10$W3ZjxXFR./UByWLjZvs5z.OIrZcd30i9C2droloE7aTlPENPBRm7u', '超级管理员', 'super_admin', 1)
ON DUPLICATE KEY UPDATE `username` = `username`;

CREATE TABLE IF NOT EXISTS `ttw_admin_token` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `admin_user_id` bigint unsigned NOT NULL COMMENT '绠＄悊鍛業D',
  `token_id` varchar(64) NOT NULL COMMENT 'token鍞竴ID',
  `expires_at` datetime NOT NULL COMMENT '杩囨湡鏃堕棿',
  `is_revoked` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '鏄惁鎾ら攢锛?鍚︼紝1鏄?,
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_token_id` (`token_id`),
  KEY `idx_admin_user_id` (`admin_user_id`),
  KEY `idx_expires_at` (`expires_at`),
  KEY `idx_is_revoked` (`is_revoked`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鍚庡彴鐧诲綍token琛?;

CREATE TABLE IF NOT EXISTS `ttw_takeover` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `creator_user_id` bigint unsigned NOT NULL COMMENT '鍒涘缓浜虹敤鎴稩D',
  `title` varchar(50) NOT NULL COMMENT '鎺ラ緳鏍囬',
  `participant_limit` int unsigned NOT NULL COMMENT '浜烘暟涓婇檺',
  `schedule_type` tinyint unsigned NOT NULL COMMENT '鏃堕棿绫诲瀷锛?鎸囧畾鏃ユ湡锛?姣忓ぉ鍥哄畾锛?鏃ユ湡鑼冨洿',
  `start_date` date DEFAULT NULL COMMENT '寮€濮嬫棩鏈?,
  `end_date` date DEFAULT NULL COMMENT '缁撴潫鏃ユ湡',
  `play_time` time NOT NULL COMMENT '鍥哄畾鏃堕棿',
  `description` varchar(500) DEFAULT NULL COMMENT '鎺ラ緳浠嬬粛',
  `summary_name` varchar(64) DEFAULT NULL COMMENT '接龙汇总展示词',
  `summary_source` varchar(16) DEFAULT NULL COMMENT '汇总展示词来源: ai/manual/fallback',
  `summary_title_hash` varchar(64) DEFAULT NULL COMMENT '汇总提取内容哈希',
  `summary_error` varchar(255) DEFAULT NULL COMMENT '最近一次汇总提取错误',
  `summary_updated_at` datetime DEFAULT NULL COMMENT '汇总展示词更新时间',
  `takeover_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '鎺ラ緳鐘舵€侊細1姝ｅ父锛?宸插叧闂?,
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '鏄惁鍒犻櫎锛?鍚︼紝1鏄?,
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  KEY `idx_creator_user_id` (`creator_user_id`),
  KEY `idx_schedule` (`schedule_type`, `start_date`, `end_date`, `play_time`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鎺ラ緳琛?;

CREATE TABLE IF NOT EXISTS `ttw_takeover_member` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `takeover_id` bigint unsigned NOT NULL COMMENT '鎺ラ緳ID',
  `user_id` bigint unsigned NOT NULL COMMENT '鐢ㄦ埛ID',
  `member_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '鎴愬憳鐘舵€侊細1宸插姞鍏ワ紝2宸查€€鍑?,
  `remark` varchar(100) DEFAULT NULL COMMENT '鍔犲叆澶囨敞',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_takeover_user` (`takeover_id`, `user_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鎺ラ緳鎴愬憳琛?;

CREATE TABLE IF NOT EXISTS `ttw_takeover_member_activity` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `takeover_id` bigint unsigned NOT NULL COMMENT 'takeover id',
  `user_id` bigint unsigned NOT NULL COMMENT 'user id',
  `action` tinyint unsigned NOT NULL COMMENT '1 join, 2 leave',
  `remark` varchar(100) DEFAULT NULL COMMENT 'member remark',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created at',
  PRIMARY KEY (`id`),
  KEY `idx_takeover_id` (`takeover_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='takeover member activity';
CREATE TABLE IF NOT EXISTS `ttw_takeover_reminder_subscription` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `takeover_id` bigint unsigned NOT NULL COMMENT '鎺ラ緳ID',
  `user_id` bigint unsigned NOT NULL COMMENT '鐢ㄦ埛ID',
  `openid` varchar(64) NOT NULL COMMENT '鐢ㄦ埛openid',
  `remind_at` datetime NOT NULL COMMENT '鎻愰啋鏃堕棿',
  `play_at` datetime NOT NULL COMMENT '寮€灞€鏃堕棿',
  `send_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '鍙戦€佺姸鎬侊細1=寰呭彂閫侊紝2=宸插彂閫侊紝3=鍙戦€佸け璐?,
  `send_error` varchar(255) DEFAULT NULL COMMENT '鍙戦€佸け璐ュ師鍥?,
  `sent_at` datetime DEFAULT NULL COMMENT '鍙戦€佹椂闂?,
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_takeover_user_play_at` (`takeover_id`, `user_id`, `play_at`),
  KEY `idx_takeover_id` (`takeover_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_remind_state` (`send_state`, `remind_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鎺ラ緳寮€灞€璁㈤槄鎻愰啋琛?;

CREATE TABLE IF NOT EXISTS `ttw_takeover_report` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `takeover_id` bigint unsigned NOT NULL COMMENT '鎺ラ緳ID',
  `reporter_user_id` bigint unsigned NOT NULL COMMENT '涓炬姤浜虹敤鎴稩D',
  `reported_user_id` bigint unsigned NOT NULL COMMENT '琚妇鎶ヤ汉鐢ㄦ埛ID',
  `report_type` varchar(32) NOT NULL DEFAULT 'other' COMMENT 'report type',
  `report_content` varchar(500) NOT NULL COMMENT '涓炬姤鍐呭',
  `image_url` varchar(512) DEFAULT NULL COMMENT '涓炬姤鎴浘',
  `image_urls` json DEFAULT NULL COMMENT '涓炬姤鎴浘鏁扮粍',
  `penalty_score` int unsigned NOT NULL DEFAULT '0' COMMENT '鎵ｉ櫎鍒嗘暟',
  `handle_note` varchar(500) DEFAULT NULL COMMENT '澶勭悊璇存槑',
  `handled_by_admin_id` bigint unsigned DEFAULT NULL COMMENT '澶勭悊绠＄悊鍛業D',
  `handled_at` datetime DEFAULT NULL COMMENT '澶勭悊鏃堕棿',
  `report_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '鐘舵€侊細1寰呭鐞嗭紝2宸插鐞嗘湭鎵ｅ垎锛?宸插鐞嗗凡鎵ｅ垎',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  KEY `idx_takeover_id` (`takeover_id`),
  KEY `idx_reporter_user_id` (`reporter_user_id`),
  KEY `idx_reported_user_id` (`reported_user_id`),
  KEY `idx_report_type` (`report_type`),
  UNIQUE KEY `uk_takeover_report_pair` (`takeover_id`, `reporter_user_id`, `reported_user_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鎺ラ緳涓炬姤琛?;

CREATE TABLE IF NOT EXISTS `ttw_user_feedback` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `user_id` bigint unsigned NOT NULL COMMENT '鎻愪氦鍙嶉鐨勭敤鎴稩D',
  `feedback_type` varchar(32) NOT NULL COMMENT '鍙嶉绫诲瀷锛歴uggestion/problem/experience/other',
  `content` varchar(500) NOT NULL COMMENT '鍙嶉鍐呭',
  `contact` varchar(100) NOT NULL DEFAULT '' COMMENT '鑱旂郴鏂瑰紡锛岄€夊～',
  `images` json DEFAULT NULL COMMENT '鍙嶉鍥剧墖URL鏁扮粍',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '鐘舵€侊細1=寰呴噰绾?2=宸查噰绾?3=涓嶇悊鐫?,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_status_created_at` (`status`, `created_at`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鐢ㄦ埛鎰忚鍙嶉琛?;

CREATE TABLE IF NOT EXISTS `ttw_announcement` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `title` varchar(80) NOT NULL COMMENT '鍏憡鏍囬',
  `content` varchar(1000) NOT NULL COMMENT '鍏憡鍐呭',
  `image_url` varchar(255) DEFAULT NULL COMMENT '鍏憡鍥剧墖',
  `status` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '鐘舵€侊細1=鍚敤 2=鍋滅敤',
  `start_time` datetime NOT NULL COMMENT '寮€濮嬪睍绀烘椂闂?,
  `end_time` datetime DEFAULT NULL COMMENT '缁撴潫灞曠ず鏃堕棿',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`id`),
  KEY `idx_status_time` (`status`, `start_time`, `end_time`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='绔欏唴鍏憡琛?;

CREATE TABLE IF NOT EXISTS `ttw_user_announcement_read` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `user_id` bigint unsigned NOT NULL COMMENT '鐢ㄦ埛ID',
  `announcement_id` bigint unsigned NOT NULL COMMENT '鍏憡ID',
  `read_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '宸茶鏃堕棿',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_announcement` (`user_id`, `announcement_id`),
  KEY `idx_announcement_id` (`announcement_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='鐢ㄦ埛鍏憡宸茶琛?;

CREATE TABLE IF NOT EXISTS `ttw_admin_operate_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '涓婚敭',
  `operate_type` varchar(32) NOT NULL COMMENT '鎿嶄綔绫诲瀷',
  `target_type` varchar(32) NOT NULL COMMENT '鐩爣绫诲瀷锛歵akeover/user',
  `target_id` bigint unsigned NOT NULL COMMENT '鐩爣ID',
  `operate_content` varchar(1000) DEFAULT NULL COMMENT '鎿嶄綔鍐呭',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  PRIMARY KEY (`id`),
  KEY `idx_target` (`target_type`, `target_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='绠＄悊鍛樻搷浣滄棩蹇楄〃';

CREATE TABLE IF NOT EXISTS `ttw_app_config` (
  `config_key` varchar(64) NOT NULL COMMENT '閰嶇疆閿?,
  `config_value` varchar(255) NOT NULL COMMENT '閰嶇疆鍊?,
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '鍒涘缓鏃堕棿',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '淇敼鏃堕棿',
  PRIMARY KEY (`config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='搴旂敤閰嶇疆琛?;

INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES
  ('publish_takeover_enabled', 'false'),
  ('ai_extract_enabled', 'false'),
  ('ai_extract_api_key', ''),
  ('ai_extract_base_url', ''),
  ('ai_extract_model', '')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
