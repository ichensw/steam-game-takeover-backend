INSERT INTO `ttw_takeover_member_activity` (`takeover_id`, `user_id`, `action`, `remark`, `gmt_create`)
SELECT m.`takeover_id`, m.`user_id`, 2, m.`remark`, m.`gmt_modified`
FROM `ttw_takeover_member` AS m
WHERE m.`member_state` = 2
  AND NOT EXISTS (
    SELECT 1
    FROM `ttw_takeover_member_activity` AS a
    WHERE a.`takeover_id` = m.`takeover_id`
      AND a.`user_id` = m.`user_id`
      AND a.`action` = 2
  );
