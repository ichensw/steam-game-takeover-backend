SET @add_user_is_banned_sql := (
  SELECT IF(COUNT(*) = 0, 'ALTER TABLE `ttw_user` ADD COLUMN `is_banned` tinyint unsigned NOT NULL DEFAULT ''0'' COMMENT ''是否封禁：0否，1是''', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_user' AND COLUMN_NAME = 'is_banned'
);
PREPARE add_user_is_banned_stmt FROM @add_user_is_banned_sql;
EXECUTE add_user_is_banned_stmt;
DEALLOCATE PREPARE add_user_is_banned_stmt;

SET @add_user_ban_reason_sql := (
  SELECT IF(COUNT(*) = 0, 'ALTER TABLE `ttw_user` ADD COLUMN `ban_reason` varchar(255) DEFAULT NULL COMMENT ''封禁原因''', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_user' AND COLUMN_NAME = 'ban_reason'
);
PREPARE add_user_ban_reason_stmt FROM @add_user_ban_reason_sql;
EXECUTE add_user_ban_reason_stmt;
DEALLOCATE PREPARE add_user_ban_reason_stmt;

SET @add_user_banned_at_sql := (
  SELECT IF(COUNT(*) = 0, 'ALTER TABLE `ttw_user` ADD COLUMN `banned_at` datetime DEFAULT NULL COMMENT ''封禁时间''', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_user' AND COLUMN_NAME = 'banned_at'
);
PREPARE add_user_banned_at_stmt FROM @add_user_banned_at_sql;
EXECUTE add_user_banned_at_stmt;
DEALLOCATE PREPARE add_user_banned_at_stmt;

SET @add_user_banned_by_admin_id_sql := (
  SELECT IF(COUNT(*) = 0, 'ALTER TABLE `ttw_user` ADD COLUMN `banned_by_admin_id` bigint unsigned DEFAULT NULL COMMENT ''封禁管理员ID''', 'SELECT 1')
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ttw_user' AND COLUMN_NAME = 'banned_by_admin_id'
);
PREPARE add_user_banned_by_admin_id_stmt FROM @add_user_banned_by_admin_id_sql;
EXECUTE add_user_banned_by_admin_id_stmt;
DEALLOCATE PREPARE add_user_banned_by_admin_id_stmt;

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
