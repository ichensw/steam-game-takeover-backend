# 兔兔窝游戏接龙后端接口文档

本文档按当前 Go 后端实现整理，入口路由见 `internal/httpapi/router.go`。

## 基础约定

默认服务地址：

```text
http://47.102.200.211:8081
```

如果通过 Nginx 路径转发，小程序 API 示例：

```text
http://域名/miniprogram-api/api/...
```

Nginx 需把 `/miniprogram-api/` 转发到后端 `/`，例如：

```nginx
location /miniprogram-api/ {
    proxy_pass http://127.0.0.1:8081/;
}
```

### 统一响应

所有接口返回统一 JSON：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {}
}
```

失败示例：

```json
{
  "success": false,
  "code": "UNAUTHORIZED",
  "message": "登录状态已失效，请重新进入小程序",
  "data": null
}
```

### 鉴权

用户接口：

```http
Authorization: Bearer <user-token>
```

管理员接口使用普通用户 token，且用户 `is_admin = 1`：

```http
Authorization: Bearer <user-token>
```

### 常用枚举

性别 `gender`：

| 值 | 含义 |
| --- | --- |
| `1` | 男 |
| `2` | 女 |

接龙时间类型 `scheduleType`：

| 值 | 含义 | 日期要求 |
| --- | --- | --- |
| `1` | 指定日期 | `startDate` 必填，`endDate` 为空或等于 `startDate` |
| `2` | 每天固定 | `startDate`、`endDate` 会被后端置空 |
| `3` | 日期范围 | `startDate`、`endDate` 必填 |

时间格式：

```text
startDate/endDate: YYYY-MM-DD
playTime: HH:mm
```

## 数据结构

### User

```json
{
  "id": 1,
  "nickname": "Zzzz",
  "steamId": "364262801",
  "gender": 1,
  "avatarUrl": "https://example.com/avatar.jpg",
  "profileCompleted": true
}
```

### Takeover

```json
{
  "id": 1,
  "title": "进 klook 集合",
  "participantLimit": 16,
  "joinedCount": 1,
  "scheduleType": 1,
  "startDate": "2026-06-21",
  "endDate": null,
  "playTime": "21:42",
  "scheduleText": "今天 21:42",
  "description": "一起玩",
  "hasJoined": true,
  "previewMembers": [],
  "members": []
}
```

### Member

普通用户接口不返回 `openid`，管理员详情会返回。

```json
{
  "userId": 1,
  "openid": "o_xxx",
  "nickname": "Zzzz",
  "steamId": "364262801",
  "gender": 1,
  "avatarUrl": "https://example.com/avatar.jpg",
  "joinedAt": "2026-06-21 21:42:00"
}
```

## 公共接口

### 健康检查

```http
GET /api/health
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "ok",
  "data": {
    "status": "ok"
  }
}
```

## 登录与用户

### 微信小程序登录

```http
POST /api/auth/wx-login
Content-Type: application/json
```

请求：

```json
{
  "code": "wx.login 返回的 code"
}
```

说明：

- 后端用 `code` 调微信 `jscode2session` 换取 `openid`。
- 若用户不存在，自动创建。
- 返回用户 token 和资料状态。
- `publishTakeoverEnabled` 表示当前用户是否可看到“发布接龙”按钮：全局配置开启，或当前用户 SteamID 在发布白名单中。
- `WX_LOGIN_MOCK=true` 时，会把 code 转为测试 openid，仅用于本地调试。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "token": "user-token",
    "user": {
      "id": 1,
      "nickname": "",
      "steamId": "",
      "gender": null,
      "avatarUrl": "",
      "profileCompleted": false
    },
    "publishTakeoverEnabled": false
  }
}
```

常见错误：

| HTTP | code | 说明 |
| --- | --- | --- |
| `400` | `PARAM_INVALID` | code 为空 |
| `502` | `SYSTEM_ERROR` | 微信登录失败 |
| `500` | `SYSTEM_ERROR` | 创建用户或签发 token 失败 |

### 查询当前用户资料

```http
GET /api/me/profile
Authorization: Bearer <user-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "id": 1,
    "nickname": "Zzzz",
    "steamId": "364262801",
    "gender": 1,
    "avatarUrl": "https://example.com/avatar.jpg",
    "profileCompleted": true
  }
}
```

### 保存当前用户资料

```http
PUT /api/me/profile
Authorization: Bearer <user-token>
Content-Type: application/json
```

请求：

```json
{
  "nickname": "Zzzz",
  "steamId": "364262801",
  "gender": 1,
  "avatarUrl": "https://example.com/avatar.jpg"
}
```

校验：

| 字段 | 要求 |
| --- | --- |
| `nickname` | 必填，2 到 12 字 |
| `steamId` | 必填，仅允许数字，最多 64 字符；会调用 Steam 信息接口校验好友码是否存在 |
| `gender` | 只能为 `1` 或 `2` |
| `avatarUrl` | 可为空，最多 255 字符 |

说明：

- `nickname` 会先走本地敏感词表，再走微信文本内容安全。
- Steam 好友码不存在时返回 `PARAM_INVALID`，提示“Steam好友码错误，请填写正确的好友码。”
- 检测未通过时返回 `PARAM_INVALID`，提示“内容包含不合规信息，请修改后再提交”。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "saved",
  "data": {
    "id": 1,
    "nickname": "Zzzz",
    "steamId": "364262801",
    "gender": 1,
    "avatarUrl": "https://example.com/avatar.jpg",
    "profileCompleted": true
  }
}
```

## 接龙接口

### 微信机器人查询账号

后台启动时会默认创建一个仅用于查询的机器人账户。该账户不会标记为资料完善，因此可以查询接龙列表、今日接龙和接龙详情，但不能创建或加入接龙。

#### 微信机器人登录

```http
POST /api/auth/bot-login
Content-Type: application/json
```

说明：
- 默认账户由环境变量 `BOT_QUERY_*` 控制，服务启动时自动创建或更新。
- 返回的是普通用户 token，可用于 `GET /api/takeovers` 和 `GET /api/takeovers/{takeoverId}`。
- 机器人项目可直接调用该接口获取 token，无需手工配置 SteamID。

响应：
```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "token": "user-token",
    "user": {
      "id": 1,
      "nickname": "WeChat Bot",
      "steamId": "wechat-bot-query",
      "gender": 1,
      "avatarUrl": "",
      "profileCompleted": false
    }
  }
}
```

微信机器人使用的查询接口：
- 今日接龙：`GET /api/takeovers?timeFilter=today&page=1&pageSize=10`
- 接龙列表：`GET /api/takeovers?timeFilter=all&page=1&pageSize=10`
- 接龙详情：`GET /api/takeovers/{takeoverId}`

### 查询接龙列表

```http
GET /api/takeovers
Authorization: Bearer <user-token>
```

查询参数：

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `keyword` | string | 空 | 按标题、介绍模糊搜索 |
| `timeFilter` | string | `all` | 时间筛选 |
| `startDate` | string | 空 | 自定义范围开始日期 |
| `endDate` | string | 空 | 自定义范围结束日期 |
| `page` | number | `1` | 页码，小于等于 0 时按默认值 |
| `pageSize` | number | `10` | 每页数量，最大 `50` |

`timeFilter`：

| 值 | 含义 |
| --- | --- |
| `all` | 全部 |
| `today` | 今天 |
| `tomorrow` | 明天 |
| `this_week` | 本周 |
| `daily` | 每天固定 |
| `date_range` | 日期范围类型 |
| `custom_range` | 自定义日期范围，必须同时传 `startDate` 和 `endDate` |

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "page": 1,
    "pageSize": 10,
    "total": 1,
    "list": [
      {
        "id": 1,
        "title": "进 klook 集合",
        "participantLimit": 16,
        "joinedCount": 1,
        "scheduleType": 1,
        "startDate": "2026-06-21",
        "endDate": null,
        "playTime": "21:42",
        "scheduleText": "今天 21:42",
        "description": "",
        "hasJoined": true,
        "previewMembers": [
          {
            "userId": 1,
            "nickname": "Zzzz",
            "steamId": "364262801",
            "gender": 1,
            "avatarUrl": "https://example.com/avatar.jpg",
            "joinedAt": "2026-06-21 21:42:00"
          }
        ]
      }
    ]
  }
}
```

说明：

- 需要传用户 token；返回当前用户的 `hasJoined`、`isCreator`、`canManage` 状态。
- 列表只返回前 5 个 `previewMembers`。
- `custom_range` 只填开始或结束日期会返回 `PARAM_INVALID`；前端应等两个日期都填写后再请求。

### 查询接龙详情

```http
GET /api/takeovers/{takeoverId}
Authorization: Bearer <user-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "id": 1,
    "title": "进 klook 集合",
    "participantLimit": 16,
    "joinedCount": 1,
    "scheduleType": 1,
    "startDate": "2026-06-21",
    "endDate": null,
    "playTime": "21:42",
    "scheduleText": "今天 21:42",
    "description": "",
    "hasJoined": true,
    "members": [
      {
        "userId": 1,
        "nickname": "Zzzz",
        "steamId": "364262801",
        "gender": 1,
        "avatarUrl": "https://example.com/avatar.jpg",
        "joinedAt": "2026-06-21 21:42:00"
      }
    ]
  }
}
```

常见错误：

| HTTP | code | 说明 |
| --- | --- | --- |
| `400` | `PARAM_INVALID` | takeoverId 非法 |
| `404` | `TAKEOVER_NOT_FOUND` | 接龙不存在或已删除 |

### 创建接龙

```http
POST /api/takeovers
Authorization: Bearer <user-token>
Content-Type: application/json
```

请求：

```json
{
  "title": "进 klook 集合",
  "participantLimit": 16,
  "scheduleType": 1,
  "startDate": "2026-06-21",
  "endDate": null,
  "playTime": "21:42",
  "description": "一起玩"
}
```

校验：

| 字段 | 要求 |
| --- | --- |
| `title` | 必填，最多 30 字 |
| `participantLimit` | `2` 到 `99` |
| `scheduleType` | `1`、`2`、`3` |
| `playTime` | 必填，格式 `HH:mm` |
| `description` | 可为空，最多 500 字 |
| 指定日期 | `startDate` 必填，不能早于今天；如果是今天，`playTime` 必须晚于当前时间 |
| 每天固定 | 后端忽略 `startDate`、`endDate` |
| 日期范围 | `startDate`、`endDate` 必填，结束日期不能早于开始日期，结束日期不能早于今天 |

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "created",
  "data": {
    "id": 1,
    "hasJoined": true,
    "joinedCount": 1
  }
}
```

说明：

- 用户必须资料完整。
- 创建者会自动加入该接龙。
- 只有 `publish_takeover_enabled=true`，或当前用户 SteamID 在发布白名单内，才允许创建。
- `title` 和 `description` 会先走本地敏感词表，再走微信文本内容安全。
- 检测未通过时返回 `PARAM_INVALID`，提示“内容包含不合规信息，请修改后再提交”。

### 加入接龙

```http
POST /api/takeovers/{takeoverId}/join
Authorization: Bearer <user-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "joined",
  "data": {
    "hasJoined": true,
    "joinedCount": 2
  }
}
```

常见错误：

| HTTP | code | 说明 |
| --- | --- | --- |
| `403` | `PROFILE_INCOMPLETE` | 用户资料未完善 |
| `404` | `TAKEOVER_NOT_FOUND` | 接龙不存在 |
| `409` | `ALREADY_JOINED` | 已加入 |
| `409` | `TAKEOVER_FULL` | 人数已满 |

### 退出接龙

```http
POST /api/takeovers/{takeoverId}/leave
Authorization: Bearer <user-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "left",
  "data": {
    "hasJoined": false,
    "joinedCount": 1
  }
}
```

常见错误：

| HTTP | code | 说明 |
| --- | --- | --- |
| `404` | `TAKEOVER_NOT_FOUND` | 接龙不存在 |
| `409` | `PARAM_INVALID` | 当前用户未加入 |

## 上传接口

### 上传图片

```http
POST /api/uploads/image
Authorization: Bearer <user-token>
Content-Type: multipart/form-data
```

表单字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | file | 是 | 图片文件 |

限制：

- 大小：1 byte 到 5 MB。
- 类型：JPG、PNG、GIF、WebP。
- 依赖 OSS 环境变量：`OSS_ENDPOINT`、`OSS_BUCKET`、`OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`、`OSS_BASE_URL`。
- 图片上传到 OSS 前会先走微信图片内容安全；检测未通过时返回 `PARAM_INVALID`。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "uploaded",
  "data": {
    "url": "https://bucket.oss-cn-hangzhou.aliyuncs.com/miniapp/uploads/2026/06/1-1780000000000-abcdef.jpg",
    "objectKey": "miniapp/uploads/2026/06/1-1780000000000-abcdef.jpg"
  }
}
```

curl 示例：

```bash
curl -X POST "http://47.102.200.211:8081/api/uploads/image" \
  -H "Authorization: Bearer user-token" \
  -F "file=@avatar.jpg"
```

## KOOK 接口

### 查询频道列表

```http
GET /api/kook/channels
Authorization: Bearer <user-token>
```

说明：

- 后端使用 `ttw_app_config.kook_bot_token` 和 `ttw_app_config.kook_guild_id` 调用 KOOK。
- 当前只返回文字频道，默认最多 50 个。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "list": [
      {
        "id": "1895580130534522",
        "name": "新人指导处",
        "topic": "欢迎新人",
        "parentId": "",
        "level": 2
      }
    ],
    "meta": {
      "page": 1,
      "pageTotal": 1,
      "pageSize": 50,
      "total": 30
    }
  }
}
```

## 管理员接口

除登录外，后台接口统一使用后台 token：

```http
Authorization: Bearer <admin-token>
```

### 管理员登录

```http
POST /api/admin/auth/login
Content-Type: application/json
```

请求：

```json
{
  "username": "admin",
  "password": "admin123"
}
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "logged in",
  "data": {
    "token": "admin-token",
    "expiresIn": 7200,
    "admin": {
      "id": 1,
      "username": "admin",
      "nickname": "超级管理员",
      "enabled": true
    }
  }
}
```

### 管理员退出登录

```http
POST /api/admin/auth/logout
Authorization: Bearer <admin-token>
```

说明：后端会撤销当前后台 token。

### 添加管理员

```http
POST /api/admin/admin-users
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "username": "csw",
  "password": "123456",
  "nickname": "管理员"
}
```

### 管理员列表

```http
GET /api/admin/admin-users?page=1&pageSize=20&keyword=&sortField=&sortOrder=
Authorization: Bearer <admin-token>
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `keyword` | string | 按用户名或昵称模糊搜索 |
| `sortField` | string | 排序字段：`id`、`username`、`nickname`、`enabled`、`lastLoginTime`、`createdAt`，默认 `createdAt` |
| `sortOrder` | string | `asc` 或 `desc`，默认 `desc` |
| `page` | number | 页码，默认 `1` |
| `pageSize` | number | 每页数量，默认 `20`，最大 `50` |

### 首页统计

```http
GET /api/admin/dashboard/summary
Authorization: Bearer <admin-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "takeoverTotal": 100,
    "userTotal": 50,
    "pendingReportTotal": 3
  }
}
```

### 系统设置查询

```http
GET /api/admin/settings
Authorization: Bearer <admin-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "publishTakeoverEnabled": false,
    "uapiKey": "uapi-xxx",
    "kookBotToken": "1/xxx",
    "kookGuildId": "3623183187289015"
  }
}
```

### 系统设置更新

```http
PUT /api/admin/settings
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "publishTakeoverEnabled": true,
  "uapiKey": "uapi-xxx",
  "kookBotToken": "1/xxx",
  "kookGuildId": "3623183187289015"
}
```

说明：

- 当前支持 `publishTakeoverEnabled`、`uapiKey`、`kookBotToken`、`kookGuildId`。
- 可只传需要更新的字段。
- `publishTakeoverEnabled` 对应 `ttw_app_config.config_key = publish_takeover_enabled`。
- `uapiKey` 对应 `ttw_app_config.config_key = uapi_key`，用于校验 Steam 好友码。
- `kookBotToken` 对应 `ttw_app_config.config_key = kook_bot_token`。
- `kookGuildId` 对应 `ttw_app_config.config_key = kook_guild_id`。

### 管理员查询接龙列表

```http
GET /api/admin/takeovers?page=1&pageSize=20&keyword=&status=&sortField=&sortOrder=
Authorization: Bearer <admin-token>
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `keyword` | string | 按标题、介绍搜索 |
| `status` | string | `normal` 或 `closed` |
| `sortField` | string | 排序字段：`id`、`title`、`participantLimit`、`scheduleType`、`startDate`、`endDate`、`playTime`、`status`、`createdAt`，默认 `createdAt` |
| `sortOrder` | string | `asc` 或 `desc`，默认 `desc` |
| `page` | number | 页码，默认 `1` |
| `pageSize` | number | 每页数量，默认 `20`，最大 `50` |

### 用户数量统计

```http
GET /api/admin/users/summary
Authorization: Bearer <admin-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "wxUserTotal": 42,
    "adminUserTotal": 2,
    "bannedUserTotal": 3,
    "totalUserTotal": 47
  }
}
```

字段说明：

- `wxUserTotal`：微信用户数量，不包含封禁用户。
- `adminUserTotal`：启用中的后台用户数量。
- `bannedUserTotal`：封禁用户数量。
- `totalUserTotal = wxUserTotal + adminUserTotal + bannedUserTotal`。

### 管理员查询接龙详情

```http
GET /api/admin/takeovers/{takeoverId}
Authorization: Bearer <admin-token>
```

响应与普通详情相同，但 `members` 会额外包含 `openid`。

### 管理员编辑接龙

```http
PUT /api/admin/takeovers/{takeoverId}
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求体与创建接龙一致。

差异：

- 不校验日期是否早于今天。
- 人数上限不能小于当前已加入人数。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "saved",
  "data": null
}
```

### 管理员删除接龙

```http
DELETE /api/admin/takeovers/{takeoverId}
Authorization: Bearer <admin-token>
```

说明：软删除，设置 `is_deleted = true`。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "deleted",
  "data": null
}
```

### 管理员查询用户

```http
GET /api/admin/users
Authorization: Bearer <admin-token>
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `keyword` | string | 按昵称、SteamID 或 openid 模糊搜索 |
| `status` | string | `normal` 或 `banned` |
| `sortField` | string | 排序字段：`id`、`nickname`、`steamId`、`isBanned`、`creditScore`、`lastLoginTime`、`createdAt`，默认 `createdAt` |
| `sortOrder` | string | `asc` 或 `desc`，默认 `desc` |
| `page` | number | 页码，默认 `1` |
| `pageSize` | number | 每页数量，默认 `20`，最大 `50` |

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "page": 1,
    "pageSize": 20,
    "total": 1,
    "list": [
      {
        "id": 1,
        "nickname": "Zzzz",
        "steamId": "364262801",
        "gender": 1,
        "avatarUrl": "https://example.com/avatar.jpg",
        "profileCompleted": true,
        "isAdmin": false,
        "publishWhitelisted": true,
        "creditScore": 100,
        "creditStatus": "normal"
      }
    ]
  }
}
```

### 管理员查询用户详情

```http
GET /api/admin/users/{userId}
Authorization: Bearer <admin-token>
```

### 批量设置用户超管

```http
POST /api/admin/users/admin/batch
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "userIds": [1, 2, 3],
  "isAdmin": true
}
```

说明：

- 批量更新 `ttw_user.is_admin`。
- `isAdmin=true` 设置为超管，`false` 取消超管。
- 已删除用户不会被更新。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "saved",
  "data": {
    "count": 3
  }
}
```

### 封禁用户

```http
POST /api/admin/users/{userId}/ban
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "reason": "违规行为"
}
```

说明：不踢出已加入接龙，只禁止后续访问 C 端接口。

### 解封用户

```http
POST /api/admin/users/{userId}/unban
Authorization: Bearer <admin-token>
```

### 恢复用户信誉分

```http
POST /api/admin/users/{userId}/credit
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "delta": 10,
  "toScore": 100
}
```

### 审核列表

```http
GET /api/admin/reports?page=1&pageSize=20&state=&keyword=&startDate=&endDate=
Authorization: Bearer <admin-token>
```

查询参数：

| 参数 | 说明 |
| --- | --- |
| `state` | 可选：`pending`、`approved`、`rejected`，默认 `pending` |
| `keyword` | 举报内容模糊查询 |
| `startDate` | 举报开始日期，格式 `YYYY-MM-DD` |
| `endDate` | 举报结束日期，格式 `YYYY-MM-DD`，不能早于 `startDate` |

### 审核详情

```http
GET /api/admin/reports/{reportId}
Authorization: Bearer <admin-token>
```

### 审核同意

```http
POST /api/admin/reports/{reportId}/approve
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "content": "情况属实",
  "penaltyScore": 10
}
```

说明：`content` 非必填，`penaltyScore` 必填，会扣除被举报用户信誉分。

### 审核驳回

```http
POST /api/admin/reports/{reportId}/reject
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "reason": "证据不足"
}
```

### 批量添加发布白名单

```http
POST /api/admin/publish-whitelist/batch
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "steamIds": ["123", "456", "789"]
}
```

## 错误码

| code | 说明 |
| --- | --- |
| `SUCCESS` | 成功 |
| `PARAM_INVALID` | 参数不正确 |
| `UNAUTHORIZED` | 用户登录状态无效 |
| `PROFILE_INCOMPLETE` | 用户资料未完善 |
| `TAKEOVER_NOT_FOUND` | 接龙不存在或已删除 |
| `TAKEOVER_FULL` | 接龙人数已满 |
| `ALREADY_JOINED` | 用户已经加入 |
| `ADMIN_UNAUTHORIZED` | 当前账号暂无管理员权限 |
| `SYSTEM_ERROR` | 系统异常 |

## 运营配置表

### 发布接龙全局开关

发布接龙全局开关仍使用 `ttw_app_config`：

```sql
UPDATE ttw_app_config
SET config_value = 'true'
WHERE config_key = 'publish_takeover_enabled';
```

规则：

- `publish_takeover_enabled=true`：所有用户可看到发布按钮，也可创建接龙。
- `publish_takeover_enabled=false`：只有 SteamID 在 `ttw_publish_takeover_whitelist` 且 `enabled=1` 的用户可看到发布按钮，也可创建接龙。

### 发布接龙白名单

白名单按 SteamID 维护：

```sql
INSERT INTO ttw_publish_takeover_whitelist (steam_id, enabled)
VALUES ('76561198000000000', 1)
ON DUPLICATE KEY UPDATE enabled = 1;

UPDATE ttw_publish_takeover_whitelist
SET enabled = 0
WHERE steam_id = '76561198000000000';

SELECT id, steam_id, enabled, gmt_create
FROM ttw_publish_takeover_whitelist
ORDER BY id DESC;
```

### 本地敏感词

本地敏感词用于微信内容安全前的兜底拦截，命中后直接拒绝并写入 `ttw_content_audit`：

```sql
INSERT INTO ttw_sensitive_word (word, enabled)
VALUES ('加我VX', 1)
ON DUPLICATE KEY UPDATE enabled = 1;

UPDATE ttw_sensitive_word
SET enabled = 0
WHERE word = '加我VX';

SELECT id, word, enabled, gmt_create
FROM ttw_sensitive_word
ORDER BY id DESC;
```

### 内容安全审核记录

昵称、接龙标题/介绍、图片上传会写入 `ttw_content_audit`：

```sql
SELECT id, user_id, openid, content_type, target_id, scene, status, wx_result, gmt_create
FROM ttw_content_audit
ORDER BY id DESC
LIMIT 20;
```

说明：

- `content_type=profile`：用户昵称。
- `content_type=takeover`：接龙标题/介绍。
- `content_type=image`：上传图片。
- `status=pass/review/risky/error`。
- 举报内容不做敏感检测。

## 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `APP_ADDR` | 服务监听地址 | `:8081` |
| `DB_DSN` | MySQL DSN | 本地示例 DSN |
| `JWT_SECRET` | 用户 token 签名密钥 | `change-me-user-token-secret` |
| `USER_TOKEN_TTL_HOURS` | 用户 token 有效期小时 | `720` |
| `WX_APP_ID` | 微信小程序 AppID | 空 |
| `WX_APP_SECRET` | 微信小程序 AppSecret | 空 |
| `WX_LOGIN_MOCK` | 是否启用微信登录 mock | `false` |
| `CONTENT_SECURITY_ENABLED` | 是否启用内容安全检测；`WX_LOGIN_MOCK=true` 时默认关闭 | `true` |
| `OSS_ENDPOINT` | OSS endpoint | 空 |
| `OSS_BUCKET` | OSS bucket | 空 |
| `OSS_ACCESS_KEY_ID` | OSS AccessKey ID | 空 |
| `OSS_ACCESS_KEY_SECRET` | OSS AccessKey Secret | 空 |
| `OSS_BASE_URL` | OSS 访问基础 URL | 空 |

## 调试提示

PowerShell 测列表接口：

```powershell
Invoke-WebRequest -Uri "http://47.102.200.211:8081/api/takeovers" `
  -Headers @{ Authorization = "Bearer user-token" }
```

PowerShell 测微信登录：

```powershell
Invoke-WebRequest -Uri "http://47.102.200.211:8081/api/auth/wx-login" `
  -Method POST `
  -ContentType "application/json" `
  -Body '{"code":"test_code"}'
```

Windows PowerShell 5.1 上传文件建议使用 `curl.exe`：

```powershell
curl.exe -X POST "http://47.102.200.211:8081/api/uploads/image" `
  -H "Authorization: Bearer user-token" `
  -F "file=@C:\path\avatar.jpg"
```
