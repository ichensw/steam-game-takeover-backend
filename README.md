# 兔兔窝游戏接龙后端

基于 `backend-api-design.md` 的 Go 接口后端实现，使用 Gin、GORM、MySQL 和 JWT。

## 已实现接口

- `POST /api/auth/wx-login`
- `GET /api/me/profile`
- `PUT /api/me/profile`
- `POST /api/uploads/image`
- `GET /api/takeovers`
- `GET /api/takeovers/:takeoverId`
- `POST /api/takeovers`
- `POST /api/takeovers/:takeoverId/join`
- `POST /api/takeovers/:takeoverId/leave`
- `GET /api/admin/takeovers/:takeoverId`
- `PUT /api/admin/takeovers/:takeoverId`
- `DELETE /api/admin/takeovers/:takeoverId`
- `GET /api/admin/users`
- `GET /api/health`

详细字段、参数、响应示例和错误码见 [`docs/api.md`](docs/api.md)。

## 启动

1. 创建 MySQL 数据库，例如：

```sql
CREATE DATABASE steam_takeover DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

2. 执行迁移 SQL：

```bash
mysql -uroot -p steam_takeover < migrations/001_init.sql
```

3. 配置环境变量，可参考 `.env.example`。

4. 启动服务：

```bash
go mod tidy
go run ./cmd/server
```

## 微信登录本地调试

真实环境需要配置：

- `WX_APP_ID`
- `WX_APP_SECRET`

本地联调时可以设置：

```text
WX_LOGIN_MOCK=true
```

此时 `POST /api/auth/wx-login` 的 `code` 会被转换为测试 openid：`mock_${code}`。不要在线上打开该配置。

## 认证说明

普通用户接口使用：

```http
Authorization: Bearer user-token
```

管理员接口使用普通用户 token，且用户 `is_admin = 1`：

```http
Authorization: Bearer user-token
```

普通用户 token 使用 `JWT_SECRET` 签名。
