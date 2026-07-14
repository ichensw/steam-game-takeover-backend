ALTER TABLE `ttw_kook_voice_session`
  ADD KEY `idx_voice_joined_exited` (`joined_at`, `exited_at`),
  ADD KEY `idx_voice_channel_joined_exited` (`channel_id`, `joined_at`, `exited_at`),
  ADD KEY `idx_voice_user_joined_exited` (`kook_user_id`, `joined_at`, `exited_at`);
