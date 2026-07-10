ALTER TABLE `ttw_takeover`
  ADD COLUMN `summary_name` varchar(64) DEFAULT NULL COMMENT '接龙汇总展示词' AFTER `kook_invite_url`,
  ADD COLUMN `summary_source` varchar(16) DEFAULT NULL COMMENT '汇总展示词来源: ai/manual/fallback' AFTER `summary_name`,
  ADD COLUMN `summary_title_hash` varchar(64) DEFAULT NULL COMMENT '汇总提取内容哈希' AFTER `summary_source`,
  ADD COLUMN `summary_error` varchar(255) DEFAULT NULL COMMENT '最近一次汇总提取错误' AFTER `summary_title_hash`,
  ADD COLUMN `summary_updated_at` datetime DEFAULT NULL COMMENT '汇总展示词更新时间' AFTER `summary_error`;

INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES
  ('ai_extract_enabled', 'false'),
  ('ai_extract_api_key', ''),
  ('ai_extract_base_url', ''),
  ('ai_extract_model', '')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
