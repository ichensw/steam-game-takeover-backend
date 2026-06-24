CREATE TABLE IF NOT EXISTS `ttw_sensitive_word` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `word` varchar(128) NOT NULL COMMENT '敏感词',
  `enabled` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '是否启用：0否，1是',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_word` (`word`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='本地敏感词表';
