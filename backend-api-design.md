# 兔兔窝游戏接龙后端接口与库表设计

## 1. 项目背景

兔兔窝游戏接龙是一个微信小程序，用于组织 Steam 游戏接龙队伍。

小程序进入首页时先静默调用 `wx.login()`，后端通过 `code` 换取 `openid` 并校验用户是否被拉黑。正常用户展示接龙列表；如果用户已被管理员拉黑，则不展示接龙列表，并提示“您已被管理员拉黑”。首页不展示微信登录样式。

由于微信小程序管控限制，前端不能静默获取用户微信号、微信昵称、微信头像。后端只通过 `wx.login` 的 `code` 换取用户 `openid`，作为用户唯一身份。昵称、Steam ID、性别、头像由用户手动填写，其中头像可以非必填，前端可根据性别展示默认头像。

## 2. 核心功能范围

- 微信小程序 openid 登录
- 启动时校验黑名单状态
- 用户补充资料
- 接龙列表查询
- 接龙详情查询
- 创建接龙
- 加入接龙
- 管理员登录
- 管理员编辑接龙
- 管理员删除接龙
- 管理员查看接龙成员
- 管理员拉黑用户
- 被拉黑用户禁止创建、加入等核心操作

## 3. 设计约定

### 3.1 命名规范

库表设计按照阿里编码规范风格：

- 表名、字段名使用小写字母和下划线。
- 表名不使用复数。
- 表名建议增加业务前缀，本项目使用 `ttw_`。
- 主键统一使用 `id`。
- 创建时间使用 `gmt_create`。
- 修改时间使用 `gmt_modified`。
- 是否字段使用 `is_xxx`。
- 避免使用 MySQL 保留字，例如 `desc`、`limit`、`range`、`order`、`user` 等。
- 字符集使用 `utf8mb4`。
- 存储引擎使用 `InnoDB`。

### 3.2 统一响应结构

建议后端接口统一返回：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "操作成功",
  "data": {}
}
```

失败示例：

```json
{
  "success": false,
  "code": "USER_BLOCKED",
  "message": "当前用户已被限制使用",
  "data": null
}
```

### 3.3 常用错误码

| 错误码 | 含义 |
| --- | --- |
| `SUCCESS` | 成功 |
| `PARAM_INVALID` | 参数错误 |
| `UNAUTHORIZED` | 用户未登录 |
| `PROFILE_INCOMPLETE` | 用户资料未完善 |
| `USER_BLOCKED` | 用户已被拉黑 |
| `TAKEOVER_NOT_FOUND` | 接龙不存在 |
| `TAKEOVER_FULL` | 接龙人数已满 |
| `ALREADY_JOINED` | 用户已加入该接龙 |
| `ADMIN_UNAUTHORIZED` | 管理员未登录或 token 无效 |
| `ADMIN_PASSWORD_INVALID` | 管理员密码错误 |
| `SYSTEM_ERROR` | 系统异常 |

## 4. 用户身份设计

### 4.1 小程序登录流程

1. 小程序启动或进入首页时，前端调用 `wx.login()` 获取 `code`。
2. 前端调用后端 `POST /api/auth/wx-login`。
3. 后端使用 `code` 请求微信接口，换取 `openid`。
4. 后端根据 `openid` 查询用户。
5. 如果用户不存在，则创建用户。
6. 后端返回业务 token、用户资料状态、是否被拉黑。
7. 如果 `blocked = true`，前端不请求或不展示接龙列表，只展示拉黑提示。
8. 如果 `blocked = false`，前端继续请求接龙列表。

注意：

- `openid` 只能由后端通过微信接口换取。
- 小程序不能拿到用户微信号 `wxid`。
- 小程序不能静默拿到微信昵称、微信头像。
- `appid`、`app_secret` 必须保存在后端，不能放在前端。
- 首页列表展示依赖登录后的黑名单校验结果，黑名单用户不允许看到接龙列表。

### 4.2 用户资料

用户需要手动填写：

- 昵称，必填
- Steam ID，必填
- 性别，必填，1 男，2 女
- 头像，非必填

如果用户未上传头像，前端根据性别展示默认头像：

```text
miniprogram/assets/avatar-male.jpg
miniprogram/assets/avatar-female.jpg
```

## 5. 数据库表设计

### 5.1 用户表：ttw_user

```sql
CREATE TABLE `ttw_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `openid` varchar(64) NOT NULL COMMENT '微信小程序openid',
  `unionid` varchar(64) DEFAULT NULL COMMENT '微信unionid，可能为空',
  `nickname` varchar(32) DEFAULT NULL COMMENT '用户昵称',
  `steam_id` varchar(64) DEFAULT NULL COMMENT 'Steam ID',
  `gender` tinyint unsigned DEFAULT NULL COMMENT '性别：1男，2女',
  `avatar_url` varchar(255) DEFAULT NULL COMMENT '用户头像地址，空则前端按性别展示默认头像',
  `is_profile_completed` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '资料是否已完善：0否，1是',
  `is_blocked` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否被拉黑：0否，1是',
  `last_login_time` datetime DEFAULT NULL COMMENT '最后登录时间',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_openid` (`openid`),
  KEY `idx_steam_id` (`steam_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

说明：

- `openid` 是用户在当前小程序内的唯一身份。
- `unionid` 不是必然存在，只有满足微信开放平台条件时才可能返回。
- `is_profile_completed` 用于判断登录后是否需要弹出补充资料弹窗。
- `is_blocked` 用于快速判断用户是否被限制操作。

### 5.2 接龙表：ttw_takeover

```sql
CREATE TABLE `ttw_takeover` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `creator_user_id` bigint unsigned NOT NULL COMMENT '创建人用户ID',
  `title` varchar(50) NOT NULL COMMENT '接龙标题',
  `participant_limit` int unsigned NOT NULL COMMENT '人数上限',
  `schedule_type` tinyint unsigned NOT NULL COMMENT '时间类型：1指定日期，2每天固定，3日期范围',
  `start_date` date DEFAULT NULL COMMENT '开始日期',
  `end_date` date DEFAULT NULL COMMENT '结束日期',
  `play_time` time NOT NULL COMMENT '固定时间',
  `description` varchar(500) DEFAULT NULL COMMENT '接龙介绍',
  `takeover_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '接龙状态：1正常，2已关闭',
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否删除：0否，1是',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  KEY `idx_creator_user_id` (`creator_user_id`),
  KEY `idx_schedule` (`schedule_type`, `start_date`, `end_date`, `play_time`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接龙表';
```

时间类型说明：

| schedule_type | 含义 | start_date | end_date | play_time |
| --- | --- | --- | --- | --- |
| 1 | 指定日期 | 必填 | 可为空或等于 start_date | 必填 |
| 2 | 每天固定 | 可为空 | 可为空 | 必填 |
| 3 | 日期范围 | 必填 | 必填 | 必填 |

列表展示时，后端可以返回原始日期，前端负责展示成 `今天`、`明天`、`MM/DD` 等。也可以由后端额外返回格式化字段。

### 5.3 接龙成员表：ttw_takeover_member

```sql
CREATE TABLE `ttw_takeover_member` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `takeover_id` bigint unsigned NOT NULL COMMENT '接龙ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `member_state` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '成员状态：1已加入，2已退出',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_takeover_user` (`takeover_id`, `user_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接龙成员表';
```

说明：

- `uk_takeover_user` 防止同一个用户重复加入同一个接龙。
- 退出后可以将 `member_state` 改为 2。
- 如果之后需要再次加入，可以更新回 1。

### 5.4 用户拉黑表：ttw_block_user

```sql
CREATE TABLE `ttw_block_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint unsigned NOT NULL COMMENT '被拉黑用户ID',
  `openid` varchar(64) NOT NULL COMMENT '被拉黑用户openid',
  `nickname_snapshot` varchar(32) DEFAULT NULL COMMENT '拉黑时昵称快照',
  `steam_id_snapshot` varchar(64) DEFAULT NULL COMMENT '拉黑时Steam ID快照',
  `block_reason` varchar(255) DEFAULT NULL COMMENT '拉黑原因',
  `is_deleted` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否解除拉黑：0否，1是',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `gmt_modified` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`),
  KEY `idx_openid` (`openid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户拉黑表';
```

说明：

- 拉黑真实依据应该是 `user_id` 或 `openid`，不能只用 Steam ID。
- Steam ID 和昵称只作为展示快照。
- 拉黑时建议同步更新 `ttw_user.is_blocked = 1`。
- 解除拉黑时建议同步更新 `ttw_user.is_blocked = 0`。

### 5.5 管理员操作日志表：ttw_admin_operate_log

```sql
CREATE TABLE `ttw_admin_operate_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `operate_type` varchar(32) NOT NULL COMMENT '操作类型',
  `target_type` varchar(32) NOT NULL COMMENT '目标类型：takeover/user',
  `target_id` bigint unsigned NOT NULL COMMENT '目标ID',
  `operate_content` varchar(1000) DEFAULT NULL COMMENT '操作内容',
  `gmt_create` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_target` (`target_type`, `target_id`),
  KEY `idx_gmt_create` (`gmt_create`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员操作日志表';
```

操作类型建议：

| operate_type | 含义 |
| --- | --- |
| `ADMIN_LOGIN` | 管理员登录 |
| `TAKEOVER_UPDATE` | 编辑接龙 |
| `TAKEOVER_DELETE` | 删除接龙 |
| `USER_BLOCK` | 拉黑用户 |
| `USER_UNBLOCK` | 解除拉黑 |

## 6. 接口设计

### 6.1 微信登录

```http
POST /api/auth/wx-login
```

请求：

```json
{
  "code": "wx.login返回的code"
}
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "操作成功",
  "data": {
    "token": "user-token",
    "user": {
      "id": 10001,
      "nickname": "兔兔",
      "steamId": "7656119xxxxxxxxxx",
      "gender": 2,
      "avatarUrl": "",
      "profileCompleted": true,
      "blocked": false
    }
  }
}
```

后端处理：

- 使用 `code` 调用微信 `jscode2session`。
- 获取 `openid`。
- 根据 `openid` 查询或创建用户。
- 更新 `last_login_time`。
- 返回业务 token。

### 6.2 查询当前用户资料

```http
GET /api/me/profile
Authorization: Bearer user-token
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "操作成功",
  "data": {
    "id": 10001,
    "nickname": "兔兔",
    "steamId": "7656119xxxxxxxxxx",
    "gender": 2,
    "avatarUrl": "",
    "profileCompleted": true,
    "blocked": false
  }
}
```

### 6.3 保存当前用户资料

```http
PUT /api/me/profile
Authorization: Bearer user-token
```

请求：

```json
{
  "nickname": "兔兔",
  "steamId": "7656119xxxxxxxxxx",
  "gender": 2,
  "avatarUrl": ""
}
```

校验：

- `nickname` 必填，建议最长 32。
- `steamId` 必填，建议最长 64。
- `gender` 必填，只允许 1 或 2。
- `avatarUrl` 非必填。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "保存成功",
  "data": {
    "profileCompleted": true
  }
}
```

### 6.4 接龙列表查询

```http
GET /api/takeovers
Authorization: Bearer user-token
```

查询参数：

| 参数 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `keyword` | string | 否 | 搜索标题和介绍 |
| `timeFilter` | string | 否 | 时间筛选 |
| `startDate` | string | 否 | 日期范围开始，格式 YYYY-MM-DD |
| `endDate` | string | 否 | 日期范围结束，格式 YYYY-MM-DD |
| `page` | number | 否 | 页码，默认 1 |
| `pageSize` | number | 否 | 每页数量，默认 10 |

`timeFilter` 建议值：

| 值 | 含义 |
| --- | --- |
| `all` | 全部 |
| `today` | 今天 |
| `tomorrow` | 明天 |
| `this_week` | 本周 |
| `daily` | 每天固定 |
| `date_range` | 日期范围 |
| `custom_range` | 自定义日期范围 |

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "操作成功",
  "data": {
    "page": 1,
    "pageSize": 10,
    "total": 20,
    "list": [
      {
        "id": 20001,
        "title": "今晚双人成行",
        "participantLimit": 4,
        "joinedCount": 2,
        "scheduleType": 1,
        "startDate": "2026-06-20",
        "endDate": null,
        "playTime": "20:30",
        "scheduleText": "今天 20:30",
        "description": "轻松娱乐，缺两个队友",
        "hasJoined": true,
        "previewMembers": [
          {
            "userId": 10001,
            "nickname": "兔兔",
            "steamId": "7656119xxxxxxxxxx",
            "gender": 2,
            "avatarUrl": ""
          }
        ]
      }
    ]
  }
}
```

说明：

- `Authorization` 必填。首页启动时已经静默登录，列表必须在黑名单校验通过后再请求。
- 如果当前用户已被拉黑，后端返回 `USER_BLOCKED`，前端不展示接龙列表，并提示“您已被管理员拉黑”。
- 后端直接返回 `hasJoined`，方便前端展示 `加入接龙` 或 `查看接龙`。
- `previewMembers` 用于列表头像展示，建议返回前 3 到 5 个成员。

### 6.5 接龙详情查询

```http
GET /api/takeovers/{takeoverId}
Authorization: Bearer user-token
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "操作成功",
  "data": {
    "id": 20001,
    "title": "今晚双人成行",
    "participantLimit": 4,
    "joinedCount": 2,
    "scheduleType": 1,
    "startDate": "2026-06-20",
    "endDate": null,
    "playTime": "20:30",
    "scheduleText": "今天 20:30",
    "description": "轻松娱乐，缺两个队友",
    "hasJoined": false,
    "members": [
      {
        "userId": 10001,
        "openid": "openid只建议管理员模式返回",
        "nickname": "兔兔",
        "steamId": "7656119xxxxxxxxxx",
        "gender": 2,
        "avatarUrl": "",
        "joinedAt": "2026-06-20 18:30:00"
      }
    ]
  }
}
```

注意：

- 普通用户详情里不建议返回其他用户的 `openid`。
- 管理员模式下可以返回 `openid`，用于拉黑确认。

### 6.6 创建接龙

```http
POST /api/takeovers
Authorization: Bearer user-token
```

请求：

```json
{
  "title": "今晚双人成行",
  "participantLimit": 4,
  "scheduleType": 1,
  "startDate": "2026-06-20",
  "endDate": null,
  "playTime": "20:30",
  "description": "轻松娱乐，缺两个队友"
}
```

校验：

- 用户必须登录。
- 用户资料必须完善。
- 用户不能被拉黑。
- `title` 必填，最长 50。
- `participantLimit` 必填，建议 2 到 99。
- `scheduleType` 必填，只允许 1、2、3。
- `playTime` 必填。
- 指定日期不能早于今天。
- 日期范围的 `endDate` 不能早于 `startDate`。
- `description` 最长 500。

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "创建成功",
  "data": {
    "id": 20001
  }
}
```

### 6.7 编辑接龙

```http
PUT /api/admin/takeovers/{takeoverId}
Authorization: Bearer admin-token
```

请求：

```json
{
  "title": "今晚双人成行",
  "participantLimit": 4,
  "scheduleType": 3,
  "startDate": "2026-06-20",
  "endDate": "2026-06-22",
  "playTime": "20:30",
  "description": "轻松娱乐，缺两个队友"
}
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "保存成功",
  "data": null
}
```

### 6.8 删除接龙

```http
DELETE /api/admin/takeovers/{takeoverId}
Authorization: Bearer admin-token
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "删除成功",
  "data": null
}
```

建议软删除：

```sql
UPDATE ttw_takeover
SET is_deleted = 1
WHERE id = ?;
```

### 6.9 加入接龙

```http
POST /api/takeovers/{takeoverId}/join
Authorization: Bearer user-token
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "加入成功",
  "data": {
    "hasJoined": true,
    "joinedCount": 3
  }
}
```

校验：

- 用户必须登录。
- 用户资料必须完善。
- 用户不能被拉黑。
- 接龙必须存在且未删除。
- 接龙人数不能超过 `participant_limit`。
- 用户不能重复加入。

并发建议：

- 加入时需要在事务内处理。
- 查询当前有效成员数量时需要防并发超员。
- 可以使用事务和行锁，也可以维护冗余 `joined_count` 字段。

当前设计没有在 `ttw_takeover` 冗余 `joined_count`，实现简单但统计会多一次查询。若后续访问量变大，可以增加 `joined_count`。

### 6.10 管理员登录

```http
POST /api/admin/login
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
  "message": "登录成功",
  "data": {
    "adminToken": "admin-token",
    "expiresIn": 7200
  }
}
```

说明：

- 管理员密码不能写在小程序前端。
- 建议放在后端环境变量中。
- 管理员 token 建议短期有效，例如 2 小时。

环境变量示例：

```text
ADMIN_PASSWORD=your-admin-password
ADMIN_TOKEN_SECRET=your-random-secret
```

### 6.11 管理员拉黑用户

```http
POST /api/admin/users/{userId}/block
Authorization: Bearer admin-token
```

请求：

```json
{
  "reason": "恶意占位"
}
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "拉黑成功",
  "data": null
}
```

后端处理：

- 查询用户信息。
- 写入或更新 `ttw_block_user`。
- 更新 `ttw_user.is_blocked = 1`。
- 记录管理员操作日志。

### 6.12 管理员解除拉黑

```http
POST /api/admin/users/{userId}/unblock
Authorization: Bearer admin-token
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "解除成功",
  "data": null
}
```

### 6.13 管理员查询拉黑列表

```http
GET /api/admin/blocked-users
Authorization: Bearer admin-token
```

响应：

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "操作成功",
  "data": {
    "list": [
      {
        "userId": 10001,
        "openid": "o_xxxxxxxxxxxxx",
        "nickname": "兔兔",
        "steamId": "7656119xxxxxxxxxx",
        "reason": "恶意占位",
        "blockedAt": "2026-06-20 18:30:00"
      }
    ]
  }
}
```

## 7. 前后端流程对应

### 7.1 小程序启动登录

1. 用户进入小程序首页。
2. 前端调用 `wx.login()` 获取 `code`。
3. 前端调用 `POST /api/auth/wx-login`。
4. 后端通过微信接口换取 `openid`。
5. 后端查询或创建用户，并判断 `is_blocked`。
6. 后端返回 `token`、`profileCompleted`、`blocked`。
7. 前端保存 `token` 和用户状态。

### 7.2 黑名单拦截

1. 如果登录接口返回 `blocked = true`，前端不请求接龙列表。
2. 页面展示空状态提示：“您已被管理员拉黑”。
3. 前端隐藏创建接龙、加入接龙、查看接龙等核心操作入口。
4. 如果用户后续请求业务接口，后端仍然需要再次校验黑名单状态，不能只依赖前端拦截。

### 7.3 正常用户加载接龙列表

1. 如果登录接口返回 `blocked = false`，前端调用 `GET /api/takeovers`。
2. 请求头携带 `Authorization: Bearer user-token`。
3. 后端返回接龙列表、当前用户是否已加入 `hasJoined`、接龙成员头像预览等信息。
4. 前端根据 `hasJoined` 展示 `查看接龙` 或 `加入接龙`。

### 7.4 已完善资料用户再次进入

1. 用户进入小程序后仍然先走启动登录。
2. 后端返回 `profileCompleted = true`。
3. 前端不弹出补资料弹窗，直接展示接龙列表。
4. 用户查看、加入、创建时复用当前 token。

### 7.5 创建接龙

1. 用户点击创建接龙。
2. 前端确认用户已登录、资料已完善。
3. 前端提交创建表单。
4. 后端校验日期、人数、拉黑状态。
5. 创建成功后前端刷新列表。

### 7.6 查看接龙详情

1. 前端请求详情接口。
2. 后端返回成员列表和 `hasJoined`。
3. 前端根据 `hasJoined` 展示：
   - 已加入：展示已加入状态。
   - 未加入：展示加入接龙按钮。

### 7.7 管理员操作

1. 管理员点击左下角悬浮入口。
2. 前端弹出密码输入框。
3. 前端调用 `POST /api/admin/login`。
4. 后端校验密码并返回 admin token。
5. 前端进入管理员模式。
6. 管理员可编辑接龙、删除接龙、拉黑成员。

## 8. 日期筛选建议

列表筛选当前建议支持：

- 全部
- 今天
- 明天
- 本周
- 每天固定
- 日期范围
- 自定义日期范围

筛选规则建议：

### 8.1 今天

命中以下接龙：

- `schedule_type = 1` 且 `start_date = 今天`
- `schedule_type = 2`
- `schedule_type = 3` 且今天在 `start_date` 到 `end_date` 之间

### 8.2 明天

命中以下接龙：

- `schedule_type = 1` 且 `start_date = 明天`
- `schedule_type = 2`
- `schedule_type = 3` 且明天在 `start_date` 到 `end_date` 之间

### 8.3 本周

命中以下接龙：

- 指定日期在本周内
- 每天固定
- 日期范围与本周有交集

### 8.4 每天固定

命中：

- `schedule_type = 2`

### 8.5 日期范围

命中：

- `schedule_type = 3`

### 8.6 自定义日期范围

命中：

- 指定日期落在自定义范围内
- 每天固定
- 接龙日期范围与自定义范围有交集

## 9. 安全注意事项

- 管理员密码不能放在小程序前端。
- 微信 `app_secret` 不能放在小程序前端。
- `openid` 不建议暴露给普通用户。
- 普通用户查看接龙成员时，只返回昵称、Steam ID、头像、性别即可。
- 管理员模式才返回 `openid` 或允许基于 `userId` 拉黑。
- 被拉黑用户应该禁止创建接龙、加入接龙。
- 后端接口必须校验权限，不能只依赖前端隐藏按钮。
- 删除建议软删除，不建议直接物理删除。

## 10. 后续可扩展项

当前版本可以先不做，但库表和接口可以预留：

- 退出接龙
- 关闭接龙
- 接龙成员上限变更时校验当前人数
- 接龙创建人自主管理自己的接龙
- 管理员操作日志查询
- 上传用户头像到对象存储
- Steam ID 格式校验
- 接龙加入通知
- 黑名单解除功能
