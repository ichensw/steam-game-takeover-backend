# 兔兔窝游戏接龙后端

兔兔窝游戏接龙后端是一个面向微信小程序和后台管理系统的 Go REST API 服务。项目负责用户登录、资料维护、游戏接龙发布与报名、举报与信誉分、用户反馈、KOOK 频道集成、内容安全审核、OSS 图片上传和后台管理能力。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| Web 框架 | Gin |
| ORM | GORM |
| 数据库 | MySQL |
| 鉴权 | JWT、后台 token |
| 文件存储 | 阿里云 OSS |
| 第三方集成 | 微信小程序登录、微信内容安全、KOOK Bot、Steam Web API |
| 测试 | Go test |

## 核心功能

### 小程序端

- 微信登录和用户 token 签发。
- 用户资料保存，包含昵称、Steam 好友码、性别、头像和资料完整性校验。
- 游戏接龙列表、详情、创建、编辑、删除、加入、退出。
- 接龙成员备注。
- 接龙举报和反馈图片上传。
- 用户意见反馈提交。
- KOOK 频道列表、频道树和邀请链接能力。
- 图片上传到 OSS，上传前走内容安全检测。
- 微信内容安全和本地敏感词审核。

### 后台管理端

- 管理员登录、登出、管理员列表和新增管理员。
- 首页统计、用户数量统计。
- 接龙列表、详情、编辑和删除。
- 用户列表、用户详情、封禁、解封、信誉分恢复。
- 批量设置用户超管。
- 发布接龙全局开关和发布白名单。
- 举报审核列表、详情、同意、驳回和批量处理。
- 用户反馈列表、详情和状态更新。
- 系统设置查询和更新。

## 目录结构

```text
.
├── cmd/server                 # 服务入口
├── docs
│   ├── api.md                 # 接口文档 Markdown
│   └── index.html             # 接口文档页面
├── internal
│   ├── config                 # 环境变量配置
│   ├── database               # 数据库初始化
│   ├── httpapi                # 路由、handler、DTO、鉴权、第三方集成
│   └── model                  # GORM 数据模型
├── migrations                 # 数据库 SQL 迁移
├── scripts                    # 本地构建脚本
├── .env.example               # 环境变量模板
├── go.mod
└── README.md
```

## 快速开始

### 1. 准备数据库

```sql
CREATE DATABASE steam_takeover
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;
```

执行初始化 SQL：

```bash
mysql -uroot -p steam_takeover < migrations/001_init.sql
```

如果是已有数据库，按文件编号顺序执行增量 migration：

```bash
mysql -uroot -p steam_takeover < migrations/023_add_user_feedback.sql
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

按实际环境修改 `.env`。常用配置如下：

| 变量 | 说明 |
| --- | --- |
| `APP_ADDR` | 服务监听地址，例如 `:8081` |
| `DB_DSN` | MySQL DSN |
| `JWT_SECRET` | 小程序用户 token 签名密钥 |
| `USER_TOKEN_TTL_HOURS` | 小程序用户 token 有效期 |
| `ADMIN_PASSWORD` | 初始化后台管理员密码 |
| `ADMIN_TOKEN_SECRET` | 后台 token 签名密钥 |
| `ADMIN_TOKEN_TTL_HOURS` | 后台 token 有效期 |
| `WX_APP_ID` | 微信小程序 AppID |
| `WX_APP_SECRET` | 微信小程序 AppSecret |
| `WX_LOGIN_MOCK` | 本地登录 mock 开关，生产必须为 `false` |
| `CONTENT_SECURITY_ENABLED` | 微信内容安全开关 |
| `BOT_QUERY_ENABLED` | 微信机器人查询账号开关 |
| `OSS_*` | 阿里云 OSS 上传配置 |
| `WECHAT_BOT_ADMIN_URL` | 微信机器人后台内部 API 地址 |
| `WECHAT_BOT_GATEWAY_SHARED_SECRET` | 与微信机器人后台一致的服务间密钥 |
| `WECHAT_BOT_PROXY_TIMEOUT_SECONDS` | 普通机器人查询超时秒数 |
| `WECHAT_BOT_SUMMARY_TIMEOUT_SECONDS` | AI 总结请求超时秒数 |

### 3. 启动服务

```bash
go mod tidy
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:8081/api/health
```

预期响应：

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

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `go run ./cmd/server` | 本地启动服务 |
| `go test -count=1 ./...` | 运行全部测试 |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/steam-game-takeover-backend ./cmd/server` | 构建 Linux 生产二进制 |
| `./scripts/deploy_backend.sh deploy` | 构建、上传并发布线上后端 |
| `./scripts/deploy_backend.sh status` | 查看线上后端服务状态和健康检查 |
| `go test -count=1 ./internal/httpapi` | 只运行 HTTP API 测试 |

## 接口文档

接口详情见：

- Markdown: [`docs/api.md`](docs/api.md)
- HTML 页面: [`docs/index.html`](docs/index.html)

线上文档通常通过 nginx 暴露为：

```text
https://www.rabbits.ink/api-docs/
```

API 统一前缀通常为：

```text
https://www.rabbits.ink/miniprogram-api
```

## 鉴权说明

小程序端接口使用用户 token：

```http
Authorization: Bearer <user-token>
```

后台管理接口使用后台 token：

```http
Authorization: Bearer <admin-token>
```

被封禁用户会被用户鉴权中间件拦截，不能访问需要小程序用户登录的 C 端接口。

## 数据库迁移

项目当前使用手写 SQL migration，不包含自动迁移执行器。

规则：

- 新库优先执行 `migrations/001_init.sql`。
- 已有库按编号顺序执行新增 SQL。
- 生产执行前先确认目标库和字段是否已经存在，避免重复 DDL。
- 表结构变更需要同时更新 `internal/model/model.go` 和 `migrations/001_init.sql`。

## 内容安全

服务端统一做内容安全，不依赖前端敏感词表：

- 昵称、接龙标题、接龙介绍等文本走本地敏感词和微信文本安全。
- 头像、举报图片、反馈图片等图片走微信图片安全。
- 审核记录保存在内容审核表，便于排查和小程序审核说明。
- 微信接口失败时按保守策略处理，不直接放行高风险内容。

## 第三方集成

### 微信

- `WX_APP_ID` 和 `WX_APP_SECRET` 用于小程序登录。
- `CONTENT_SECURITY_ENABLED=true` 时启用内容安全检测。
- 本地联调可临时设置 `WX_LOGIN_MOCK=true`，生产禁止开启。

### 阿里云 OSS

头像、举报图片、反馈图片等上传到 OSS。需要配置：

```text
OSS_ENDPOINT
OSS_BUCKET
OSS_ACCESS_KEY_ID
OSS_ACCESS_KEY_SECRET
OSS_BASE_URL
```

### KOOK

KOOK Bot 配置保存在系统设置中，用于查询频道和生成频道邀请链接：

- `kook_bot_token`
- `kook_guild_id`

### Steam

Steam 好友码相关校验使用 Steam Web API，配置项为：

```text
steam_web_api_key
```

### 微信机器人后台

后台管理员访问 `/api/admin/wechat-bot/*` 时，服务会先校验现有管理员 token 和角色菜单权限，再将白名单请求转发到 `WECHAT_BOT_ADMIN_URL`。浏览器不会接触服务间共享密钥。生产环境应让微信机器人后台只接受本服务的私有网络访问。

## 部署说明

查看线上状态：

```bash
./scripts/deploy_backend.sh status
```

服务器侧通常放置在：

```text
/opt/steam-game-takeover-backend
```

systemd 服务名：

```text
steam-game-takeover-backend.service
```

发布后端：

```bash
./scripts/deploy_backend.sh deploy
```

脚本会按顺序执行测试、Linux 构建、上传二进制、备份旧二进制、重启 systemd 服务、检查本机和公网健康接口。脚本不保存服务器密码、数据库密码或密钥，SSH/SCP/MySQL 认证走本机或服务器已有配置。

如需随发布执行增量 SQL，必须显式指定 migration，避免误把历史 migration 重跑：

```bash
MIGRATIONS="migrations/043_xxx.sql" ./scripts/deploy_backend.sh deploy
```

可按需覆盖服务器配置：

```bash
DEPLOY_HOST=47.102.200.211 APP_DIR=/opt/steam-game-takeover-backend ./scripts/deploy_backend.sh status
```

## 开发约定

- 保持接口响应使用统一结构：`success`、`code`、`message`、`data`。
- 新增接口同步更新 `docs/api.md`。
- 新增表同步补充 `migrations/001_init.sql` 和新的增量 migration。
- 新增业务校验至少补充一个最小测试。
- 不在代码、README 或提交记录中写入生产密码、token、私钥。

## 参考入口

- 服务入口：[cmd/server/main.go](cmd/server/main.go)
- 路由注册：[internal/httpapi/router.go](internal/httpapi/router.go)
- 数据模型：[internal/model/model.go](internal/model/model.go)
- 接口文档：[docs/api.md](docs/api.md)
