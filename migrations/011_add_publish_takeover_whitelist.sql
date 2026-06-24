CREATE TABLE IF NOT EXISTS `ttw_publish_takeover_whitelist` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `steam_id` varchar(64) NOT NULL COMMENT 'Steam ID',
  `enabled` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '是否启用：0否，1是',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_steam_id` (`steam_id`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发布接龙白名单表';
