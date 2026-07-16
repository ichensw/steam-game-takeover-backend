CREATE TABLE IF NOT EXISTS `ttw_user_block` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `owner_user_id` bigint unsigned NOT NULL COMMENT '拉黑发起用户ID',
  `blocked_user_id` bigint unsigned NOT NULL COMMENT '被拉黑用户ID',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_owner_blocked` (`owner_user_id`, `blocked_user_id`),
  KEY `idx_owner_user_id` (`owner_user_id`),
  KEY `idx_blocked_user_id` (`blocked_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户拉黑关系表';
