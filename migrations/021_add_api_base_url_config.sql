INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES ('api_base_url', 'https://debun.xyz/miniprogram-api')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
