ALTER TABLE `ttw_takeover_member`
  ADD COLUMN `remark` varchar(100) DEFAULT NULL COMMENT '加入备注' AFTER `member_state`;
