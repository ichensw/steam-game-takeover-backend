ALTER TABLE `wechat_group_member_profiles`
  ADD INDEX `idx_room_updated_at` (`room_id`, `updated_at`),
  ADD INDEX `idx_room_display_name` (`room_id`, `display_name`),
  ADD INDEX `idx_room_nickname` (`room_id`, `nickname`),
  ADD INDEX `idx_room_alias` (`room_id`, `alias`);
