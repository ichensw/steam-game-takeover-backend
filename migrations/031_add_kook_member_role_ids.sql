ALTER TABLE `ttw_kook_member`
  ADD COLUMN `role_ids` json DEFAULT NULL COMMENT 'KOOK role ids' AFTER `is_bot`;
