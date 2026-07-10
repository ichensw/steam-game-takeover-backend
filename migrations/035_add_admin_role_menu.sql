CREATE TABLE IF NOT EXISTS `ttw_admin_role_menu` (
  `role` varchar(32) NOT NULL COMMENT '管理员角色',
  `menu_keys` json NOT NULL COMMENT '可见菜单key列表',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台角色菜单配置表';

INSERT INTO `ttw_admin_role_menu` (`role`, `menu_keys`) VALUES
('super_admin', JSON_ARRAY('dashboard', 'takeovers', 'reports', 'users', 'admin-users', 'kook-channels', 'kook-roles', 'kook-members', 'kook-users', 'feedbacks', 'announcements', 'settings')),
('kook_admin', JSON_ARRAY('dashboard', 'takeovers', 'reports', 'users', 'kook-channels', 'kook-roles', 'kook-members', 'kook-users', 'feedbacks', 'announcements', 'settings')),
('admin', JSON_ARRAY('dashboard', 'takeovers', 'reports', 'users', 'feedbacks', 'announcements', 'settings'))
ON DUPLICATE KEY UPDATE `role` = `role`;
