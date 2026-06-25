INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES
  ('kook_bot_token', ''),
  ('kook_guild_id', '')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
