CREATE TABLE IF NOT EXISTS `ttw_app_config` (
  `config_key` varchar(64) NOT NULL COMMENT '配置键',
  `config_value` longtext NOT NULL COMMENT '配置值',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用配置表';

INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES ('publish_takeover_enabled', 'false')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
