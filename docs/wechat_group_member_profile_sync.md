# WeChat Group Member Profile Sync

## Goal

Cache richer WeChat group member profiles for the admin "微信群管理" member pages while keeping list queries fast. The member list should continue to read from local tables; Hook API calls should run in controlled sync jobs, not during normal page loads.

## Hook APIs

### `POST /api/get_group_member_contact`

Use this as the primary source for a single member's rich profile.

Request:

```json
{
  "wxid": "wxid_3e9mll0g0fad21",
  "roomId": "45220347292@chatroom"
}
```

Useful response fields from `contactList[0]`:

| Local meaning | Hook field |
| --- | --- |
| Member wxid | `userName.String`, `friendUserName` |
| Nickname | `nickName.String` |
| Remark | `remark.String` |
| WeChat ID | `alias` |
| Sex | `sex` |
| Region | `country`, `province`, `city` |
| Signature | `signature` |
| Avatar | `bigHeadImgUrl`, `smallHeadImgUrl`, `headImgMd5` |
| Chatroom | `newChatroomData.chatRoomUserName.String` |
| In-room flag | `isInChatRoom` |
| Raw status | `verifyFlag`, `contactType`, `deleteFlag`, `status` if present |

Notes:

- `alias` is the closest field to a human WeChat ID, but it may be empty.
- `userName` and `friendUserName` are usually wxid values, not human WeChat IDs.
- Store the raw profile JSON so future fields can be surfaced without another network call.

### `POST /api/get_room_members`

Use this for full group member discovery and low-cost batch cache warmup.

Request:

```json
{
  "room_id": "49866796771@chatroom"
}
```

Useful response fields:

| Local meaning | Hook field |
| --- | --- |
| Room ID | `chatroomUserName` |
| Owner wxid | `chatRoomOwner` |
| Member total | `allMemberCount`, `newChatroomData.memberCount` |
| Admin total | `adminCount` |
| Member wxids | `allMemberUserNameList[].String` |
| Member nickname/avatar | `newChatroomData.chatRoomMember[]` |
| Inviter | `newChatroomData.chatRoomMember[].inviterUserName` |
| Join trace XML | `newChatroomData.chatRoomMember[].addChatRoomSceneNewXml` |
| Member flag/status | `chatroomMemberFlag`, `status` |

### `POST /api/get_chatroom_detail_cache`

Use this when group details are needed frequently. It returns cached group detail plus `newChatroomData.chatRoomMember[]`, similar to `get_room_members`, and may include `displayName`.

### `POST /api/get_group_memeber_info`

Use this as the source for group-specific member information, especially the member's group display name. The endpoint name is intentionally spelled `memeber` to match the Hook API path.

Request:

```json
{
  "roomId": "49767299448@chatroom",
  "memeberId": "wxid_bktzp6cv7wxe12"
}
```

The request parameter is intentionally spelled `memeberId` to match the Hook API.

Example response:

```json
{
  "displayName": "可耐的甜橙-880783846",
  "nick": "橙子橙子",
  "roomId": "46348533444@chatroom"
}
```

Useful response fields:

| Local meaning | Hook field |
| --- | --- |
| Room ID | `roomId` |
| Member wxid | request `memeberId` |
| Group display name | `displayName` |
| Nickname echo | `nick` |

Notes:

- Group nickname display uses only `displayName`; do not use `nick` as a group nickname fallback.
- If the endpoint returns an empty object, skip updating group-specific fields for that member.
- The endpoint is fast enough for higher-concurrency batch sync, but it still runs in background jobs rather than page-load requests.

### `POST /api/get_member_nick`

Use this only as a lightweight fallback when full contact data is not needed. It can return nickname, avatar, member flag, inviter, and status, but not the richer contact fields such as `alias`, region, or signature.

### `/get_contact`

Do not use this in the default group member profile sync. Most group members are not bot contacts, so this endpoint does not reliably return useful data for the member list.

## Proposed Local Tables

### `wechat_group_member_profiles`

One row per `(room_id, member_wxid)`.

Recommended columns:

| Column | Purpose |
| --- | --- |
| `room_id` | Chatroom ID |
| `member_wxid` | Member wxid |
| `nickname` | WeChat nickname |
| `display_name` | Group display name from `get_group_memeber_info.displayName` |
| `remark` | Bot account remark for this contact |
| `alias` | Human WeChat ID when available |
| `sex` | Raw WeChat sex value |
| `country`, `province`, `city` | Region |
| `signature` | User signature |
| `big_head_img_url`, `small_head_img_url`, `head_img_md5` | Avatar metadata |
| `chatroom_member_flag` | Raw chatroom member flag |
| `status` | Raw status |
| `inviter_user_name` | Inviter wxid if known |
| `add_chatroom_scene_xml` | Join trace XML if known |
| `is_in_chat_room` | Current membership signal |
| `last_seen_message_at` | Latest message observed locally |
| `group_info_synced_at` | Last successful non-empty `get_group_memeber_info` sync |
| `profile_synced_at` | Last successful `get_group_member_contact` sync |
| `profile_sync_error` | Last sync error |
| `raw_profile_json` | Full raw response/contact JSON |
| `created_at`, `updated_at` | Local timestamps |

Indexes:

- Unique: `(room_id, member_wxid)`
- Query: `(room_id, profile_synced_at)`
- Query: `(room_id, last_seen_message_at)`
- Optional search: `(alias)`, `(nickname)`, `(display_name)`

### `wechat_group_member_profile_sync_state`

One row per room.

Recommended columns:

| Column | Purpose |
| --- | --- |
| `room_id` | Chatroom ID |
| `status` | `idle`, `running`, `failed`, `succeeded` |
| `sync_type` | `full`, `incremental` |
| `cursor_member_wxid` | Full-sync cursor |
| `last_full_synced_at` | Last completed full sync |
| `last_incremental_synced_at` | Last completed incremental sync |
| `processed_count` | Current or last run processed count |
| `failed_count` | Current or last run failed count |
| `last_error` | Last job-level error |
| `locked_until` | Lease to prevent duplicate workers |
| `updated_at` | Local update timestamp |

## Sync Timing

### First full sync

Trigger when:

- Admin clicks "同步群成员资料".
- A room is enabled for member profile enrichment for the first time.
- Local profile coverage is below an agreed threshold.

Flow:

1. Call `get_room_members` or `get_chatroom_detail_cache` to discover current member wxids and low-cost nickname/avatar data.
2. Upsert discovered members into `wechat_group_member_profiles`.
3. Call `get_group_memeber_info` with higher concurrency to fetch group display names. Store `displayName`; if it is empty or absent, skip updating `display_name`.
4. Enqueue per-member `get_group_member_contact` jobs for richer public/contact profile fields.
5. Do not call `/get_contact` in the normal sync path.
6. Run profile sync with controlled concurrency and persist cursor/errors so the run can resume.

### Event-driven incremental sync

Enqueue a member sync when:

- A member joins a group.
- A member changes group nickname.
- A member sends a message but local profile is missing.
- Admin manually refreshes a member or current page.

### Scheduled incremental sync

Run periodically, for example every 30 minutes:

- Scan recently active members, such as members with messages in the last 24 hours.
- Sync only if `profile_synced_at` is empty or older than 24 hours.
- Limit each run, for example 50-100 members per bot, to avoid Hook API pressure.

### Low-frequency full calibration

Run daily or weekly for important groups:

- Refresh stale profile fields such as avatar, alias, signature, and region.
- Reconcile current membership.
- Mark missing or departed members as not in room when supported by API evidence.

Suggested TTLs:

| Profile type | TTL |
| --- | --- |
| Missing profile | immediate sync |
| Group display name | immediate sync on nickname-change event, otherwise 1 day |
| Recently active member | 24 hours |
| Ordinary cached member | 7 days |
| Group member roster | 1 day |
| Failed member sync | retry after 10-30 minutes, max 3 quick retries |

## Admin API Shape

Proposed backend admin endpoints:

```text
POST /api/admin/wechat-bot/groups/manage/:roomId/member-profiles/sync
GET  /api/admin/wechat-bot/groups/manage/:roomId/member-profiles/sync
POST /api/admin/wechat-bot/groups/manage/:roomId/members/:memberWxid/profile/refresh
```

`POST .../sync` body:

```json
{
  "mode": "full"
}
```

or:

```json
{
  "mode": "incremental"
}
```

The existing member list endpoint should join local profile cache fields and never call Hook API inline.

## Debug Plan For `get_group_member_contact`

Before implementation, manually verify the Hook API against a small known group.

1. Pick one test room and 2-3 member wxids.
2. Call `get_group_member_contact` for each member.
3. Call `get_group_memeber_info` for the same members using `roomId` and `memeberId`.
4. Save sanitized JSON samples outside source control or under a redacted fixture name.
5. Confirm which fields are stable:
   - `alias`
   - `nickName.String`
   - `remark.String`
   - group display name from `get_group_memeber_info`
   - `sex`
   - `country`, `province`, `city`
   - `signature`
   - avatar URLs
   - `isInChatRoom`
6. Test cases:
   - normal member
   - member without WeChat ID alias
   - member not in contacts
   - member already left group if available
   - invalid wxid or room id
   - `get_group_memeber_info` empty-object response
7. Record actual error shape for:
   - Hook API timeout
   - nonzero `baseResponse.ret`
   - empty `contactList`

Do not log full signatures, remarks, avatar URLs, or raw profiles in production logs. Log only room ID, member wxid, ret code, elapsed time, and a short error message.

## Open Questions

- Does `get_group_member_contact` require the member to be a contact/friend, or does group membership suffice?
- Is `alias` consistently present for users who set a WeChat ID?
- Does `isInChatRoom` accurately reflect current membership?
- What is the safe request rate for `get_group_member_contact` on the production bot?
- Should full sync be opt-in per group to avoid privacy and load surprises?
