INSERT INTO `ttw_admin_role_menu` (`role`, `menu_keys`) VALUES
('super_admin', JSON_ARRAY('dashboard', 'takeovers', 'reports', 'users', 'admin-users', 'kook-channels', 'kook-roles', 'kook-members', 'kook-users', 'kook-voice-stats', 'feedbacks', 'announcements', 'settings', 'wechat-ai-memory'))
ON DUPLICATE KEY UPDATE `menu_keys` = CASE
  WHEN JSON_CONTAINS(`menu_keys`, JSON_QUOTE('wechat-ai-memory')) THEN `menu_keys`
  ELSE JSON_ARRAY_APPEND(`menu_keys`, '$', 'wechat-ai-memory')
END;
