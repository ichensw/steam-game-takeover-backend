# 每日接龙自动结束 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理员可配置每日接龙从创建起的有效天数，后端默认 10 天并自动将到期记录标记为已结束。

**Architecture:** 配置继续存放在 `ttw_app_config`。独立 worker 启动时和每小时读取有效天数，用单条幂等 Gorm 更新关闭超期每日接龙；前端在现有系统设置表单中维护该整数。

**Tech Stack:** Go 1.22、Gin、Gorm/MySQL、React 19、TypeScript、Ant Design 6、Vitest、Vite。

## Global Constraints

- 有效期从 `gmt_create` 计算，修改接龙不会延期。
- 默认值为 10，合法范围为 1 至 365 个自然日。
- 只更新 `schedule_type = ScheduleDaily`、`is_deleted = false`、`takeover_state = TakeoverStateNormal` 的记录。
- 到期后只设置 `takeover_state = TakeoverStateClosed`，不修改 `is_deleted`，不删除关联记录。
- worker 启动立即执行，之后每小时执行，错误只记日志并在下一轮重试。
- 不新增依赖、数据库表、迁移或 Nginx 路由。

---

### Task 1: 配置读取与管理接口

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/httpapi/app_config.go`
- Modify: `internal/httpapi/app_config_test.go`

**Interfaces:**
- Produces: `AppConfigDailyTakeoverExpirationDays = "daily_takeover_expiration_days"`。
- Produces: `func (h *Handler) dailyTakeoverExpirationDays() int`，始终返回 1 至 365，默认 10。
- Produces: 管理设置 JSON 字段 `dailyTakeoverExpirationDays`。

- [ ] **Step 1: 写失败测试**

在 `app_config_test.go` 增加纯解析测试：

```go
func TestParseDailyTakeoverExpirationDays(t *testing.T) {
	for _, tt := range []struct { raw string; want int }{
		{"", 10}, {"abc", 10}, {"0", 10}, {"366", 10}, {"1", 1}, {"365", 365},
	} {
		if got := parseDailyTakeoverExpirationDays(tt.raw); got != tt.want {
			t.Fatalf("parseDailyTakeoverExpirationDays(%q) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: 确认 RED**

运行：`go test -count=1 ./internal/httpapi -run 'TestParseDailyTakeoverExpirationDays'`

预期：因 `parseDailyTakeoverExpirationDays` 不存在而编译失败。

- [ ] **Step 3: 实现配置与接口校验**

增加常量、解析函数和设置字段。请求字段使用 `*int`；当值不在 1 至 365 时返回 `400 PARAM_INVALID` 和 `dailyTakeoverExpirationDays must be between 1 and 365`。合法值用 `strconv.Itoa` 保存，GET 和 PUT 响应均调用 `dailyTakeoverExpirationDays()`。

- [ ] **Step 4: 验证 GREEN**

运行：`go test -count=1 ./internal/httpapi`

预期：全部通过。

- [ ] **Step 5: 提交**

```bash
git add internal/model/model.go internal/httpapi/app_config.go internal/httpapi/app_config_test.go
git commit -m "feat: configure daily takeover expiration"
```

### Task 2: 每日接龙自动结束 worker

**Files:**
- Create: `internal/httpapi/daily_takeover_expiration.go`
- Create: `internal/httpapi/daily_takeover_expiration_test.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: `h.dailyTakeoverExpirationDays()`。
- Produces: `func closeExpiredDailyTakeovers(db *gorm.DB, now time.Time, days int) (int64, error)`。
- Produces: `func (h *Handler) StartDailyTakeoverExpirationWorker(ctx context.Context)`。

- [ ] **Step 1: 写失败 SQL 行为测试**

使用仓库现有 MySQL Gorm DryRun 模式，固定 `now` 和 `days`，断言生成 SQL 包含：

```go
Where("schedule_type = ? AND is_deleted = ? AND takeover_state = ? AND gmt_create <= ?",
	model.ScheduleDaily, false, model.TakeoverStateNormal, cutoff).
Update("takeover_state", model.TakeoverStateClosed)
```

并断言 cutoff 等于 `now.AddDate(0, 0, -10)`。

- [ ] **Step 2: 确认 RED**

运行：`go test -count=1 ./internal/httpapi -run 'TestCloseExpiredDailyTakeovers'`

预期：因清理函数不存在而编译失败。

- [ ] **Step 3: 实现单次清理和 worker**

`closeExpiredDailyTakeovers` 使用单条批量更新并返回 `RowsAffected`。worker 立即执行 `run()`，然后使用 `time.NewTicker(time.Hour)`；每轮重新读取配置。错误日志格式为 `close expired daily takeovers: %v`，成功更新时记录 `closed %d expired daily takeovers using %d days`。

- [ ] **Step 4: 接入服务启动**

在 `cmd/server/main.go` 的提醒 worker 后增加：

```go
handler.StartDailyTakeoverExpirationWorker(context.Background())
```

- [ ] **Step 5: 验证 GREEN**

运行：`go test -count=1 ./... && go vet ./...`

预期：全部通过且无 vet 错误。

- [ ] **Step 6: 提交**

```bash
git add cmd/server/main.go internal/httpapi/daily_takeover_expiration.go internal/httpapi/daily_takeover_expiration_test.go
git commit -m "feat: close expired daily takeovers"
```

### Task 3: 系统设置前端

**Files:**
- Create: `src/utils/settings.ts`
- Create: `src/utils/settings.test.ts`
- Modify: `src/pages/Settings.tsx`

**Interfaces:**
- Produces: `SettingsValues.dailyTakeoverExpirationDays?: number`。
- Produces: `normalizeSettings(values).dailyTakeoverExpirationDays: number`，缺失或非法时为 10。

- [ ] **Step 1: 写失败前端测试**

```ts
it('defaults daily takeover expiration to ten days', () => {
  expect(normalizeSettings({}).dailyTakeoverExpirationDays).toBe(10);
});

it('keeps valid integer expiration days', () => {
  expect(normalizeSettings({ dailyTakeoverExpirationDays: 30 }).dailyTakeoverExpirationDays).toBe(30);
});
```

- [ ] **Step 2: 确认 RED**

运行：`npm test -- src/utils/settings.test.ts`

预期：因 `src/utils/settings.ts` 不存在而失败。

- [ ] **Step 3: 实现规范化和表单控件**

从 `Settings.tsx` 移出 `SettingsValues`、`sensitiveKeys` 和 `normalizeSettings`。规范化使用整数和范围校验：

```ts
const days = Number(values.dailyTakeoverExpirationDays);
dailyTakeoverExpirationDays: Number.isInteger(days) && days >= 1 && days <= 365 ? days : 10
```

设置页导入 `InputNumber`，增加 `name="dailyTakeoverExpirationDays"`、`min={1}`、`max={365}`、`step={1}`、`precision={0}` 的表单项和已确认文案。

- [ ] **Step 4: 验证 GREEN 和构建**

运行：`npm test && npm run build`

预期：全部测试通过，TypeScript 和 Vite 构建成功。

- [ ] **Step 5: 提交**

```bash
git add src/pages/Settings.tsx src/utils/settings.ts src/utils/settings.test.ts
git commit -m "feat: configure daily takeover expiration in admin"
```

### Task 4: 审查、生产发布与验证

**Files:**
- No source files expected.

**Interfaces:**
- Consumes: 后端配置/worker 和前端设置表单。
- Produces: 生产环境自动结束行为。

- [ ] **Step 1: 全量质量门**

后端运行 `go test -count=1 ./... && go vet ./...`；前端运行 `npm test && npm run build && npm audit --omit=dev`。两个仓库运行 `git diff --check`，检查工作区和提交中无密钥。

- [ ] **Step 2: 浏览器验证**

在隔离浏览器检查桌面 1440px 和移动 320px：字段显示默认或服务端值、只能输入 1 至 365 的整数、保存请求包含 `dailyTakeoverExpirationDays`、无新增 console error/warning。

- [ ] **Step 3: 备份和部署**

构建 Linux x86_64 后端和前端 tar 包。服务器备份 `/opt/steam-game-takeover-backend/steam-game-takeover-backend` 与 `/opt/steam-game-takeover-web/dist`，原子替换后重启后端。运行 `nginx -t`，路径未变化时不修改 Nginx。

- [ ] **Step 4: 生产验证**

验证健康接口 200、两项服务 active、设置接口返回新字段。查询以下三类数量：超期且仍进行中的每日接龙应为 0；已结束的超期每日接龙应存在或与发布前待处理数一致；`is_deleted` 不因任务变化。检查发布后日志无清理错误。

- [ ] **Step 5: 推送远端**

后端推送 `main`，前端推送 `master`，再次确认本地分支与远端同步且工作区干净。
