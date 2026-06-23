ALTER TABLE `ttw_takeover_report`
  MODIFY COLUMN `image_url` varchar(512) DEFAULT NULL COMMENT 'report first image url';

CREATE TABLE IF NOT EXISTS `ttw_takeover_report_image` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `report_id` bigint unsigned NOT NULL COMMENT 'report id',
  `image_url` varchar(512) NOT NULL COMMENT 'report image url',
  `sort_order` int unsigned NOT NULL DEFAULT '0' COMMENT 'sort order',
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT 'is deleted: 0 no, 1 yes',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created time',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'modified time',
  PRIMARY KEY (`id`),
  KEY `idx_report_id` (`report_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='takeover report image table';
