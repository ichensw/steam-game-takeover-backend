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

管理员接口：

```http
Authorization: Bearer <admin-token>
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
  "profileCompleted": true,
  "blocked": false
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
      "profileCompleted": false,
      "blocked": false
    }
  }
}
```

常见错误：

| HTTP | code | 说明 |
| --- | --- | --- |
| `400` | `PARAM_INVALID` | code 为空 |
| `502` | `SYSTEM_ERROR` | 微信登录失败 |
| `500` | `SYSTEM_ERROR` | 创建用户或签发 token 失败 |

### Web 调试登录

```http
POST /api/auth/web-login
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

说明：

- 主要用于 Web/调试场景。
- `steamId` 必填，最长 64 字符。
- 如果只传 `steamId`，后端会按 SteamID 查询或创建一个 `web_` openid 用户。
- 如果传完整资料，会保存昵称、性别、头像并标记资料已完善。

响应同微信登录：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "token": "user-token",
    "user": {}
  }
}
```

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
    "profileCompleted": true,
    "blocked": false
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
| `nickname` | 必填，最多 32 字 |
| `steamId` | 必填，最多 64 字符 |
| `gender` | 只能为 `1` 或 `2` |
| `avatarUrl` | 可为空，最多 255 字符 |

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
    "profileCompleted": true,
    "blocked": false
  }
}
```

## 接龙接口

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

- 被拉黑用户返回 `USER_BLOCKED`。
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
| `403` | `USER_BLOCKED` | 当前用户被拉黑 |

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
| `title` | 必填，最多 50 字 |
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
- 被拉黑用户不能创建。

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
| `403` | `USER_BLOCKED` | 用户被拉黑 |
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

## 管理员接口

### 管理员登录

```http
POST /api/admin/login
Content-Type: application/json
```

请求：

```json
{
  "password": "管理员密码"
}
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "logged in",
  "data": {
    "adminToken": "admin-token",
    "expiresIn": 7200
  }
}
```

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

### 管理员拉黑用户

```http
POST /api/admin/users/{userId}/block
Authorization: Bearer <admin-token>
Content-Type: application/json
```

请求：

```json
{
  "reason": "恶意占位"
}
```

说明：

- `reason` 可为空，最多 255 字。
- 会写入或更新 `ttw_block_user`。
- 会设置用户 `is_blocked = true`。
- 会把该用户所有已加入成员状态改为已退出。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "blocked",
  "data": null
}
```

### 管理员解除拉黑

```http
POST /api/admin/users/{userId}/unblock
Authorization: Bearer <admin-token>
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "unblocked",
  "data": null
}
```

### 管理员查询拉黑列表

```http
GET /api/admin/blocked-users
Authorization: Bearer <admin-token>
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `keyword` | string | 按昵称快照或 SteamID 快照模糊搜索 |

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": {
    "list": [
      {
        "userId": 1,
        "openid": "o_xxx",
        "nickname": "Zzzz",
        "steamId": "364262801",
        "reason": "恶意占位",
        "blockedAt": "2026-06-21 21:42:00"
      }
    ]
  }
}
```

## 错误码

| code | 说明 |
| --- | --- |
| `SUCCESS` | 成功 |
| `PARAM_INVALID` | 参数不正确 |
| `UNAUTHORIZED` | 用户登录状态无效 |
| `PROFILE_INCOMPLETE` | 用户资料未完善 |
| `USER_BLOCKED` | 用户被拉黑 |
| `TAKEOVER_NOT_FOUND` | 接龙不存在或已删除 |
| `TAKEOVER_FULL` | 接龙人数已满 |
| `ALREADY_JOINED` | 用户已经加入 |
| `ADMIN_UNAUTHORIZED` | 管理员登录状态无效 |
| `ADMIN_PASSWORD_INVALID` | 管理员密码错误 |
| `SYSTEM_ERROR` | 系统异常 |

## 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `APP_ADDR` | 服务监听地址 | `:8081` |
| `DB_DSN` | MySQL DSN | 本地示例 DSN |
| `JWT_SECRET` | 用户 token 签名密钥 | `change-me-user-token-secret` |
| `USER_TOKEN_TTL_HOURS` | 用户 token 有效期小时 | `720` |
| `ADMIN_PASSWORD` | 管理员密码 | 空 |
| `ADMIN_TOKEN_SECRET` | 管理员 token 签名密钥 | `change-me-admin-token-secret` |
| `ADMIN_TOKEN_TTL_HOURS` | 管理员 token 有效期小时 | `2` |
| `WX_APP_ID` | 微信小程序 AppID | 空 |
| `WX_APP_SECRET` | 微信小程序 AppSecret | 空 |
| `WX_LOGIN_MOCK` | 是否启用微信登录 mock | `false` |
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
