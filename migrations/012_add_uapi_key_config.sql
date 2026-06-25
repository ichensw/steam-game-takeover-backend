INSERT INTO `ttw_app_config` (`config_key`, `config_value`)
VALUES ('uapi_key', '')
ON DUPLICATE KEY UPDATE `config_key` = `config_key`;
