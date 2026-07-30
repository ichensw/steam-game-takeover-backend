# Wxbot 配置契约与版本号实施计划

**Goal:** 让 `steam-game-takeover-backend`、`steam-game-takeover-web`、`wechat-hook-bot` 对同一份机器人配置强制一致。新增配置字段时，保存、下发、机器人应用、状态上报任一环节不一致都能被测试或错误状态暴露，而不是让机器人静默离线。

**Architecture:** 后端维护唯一配置契约和 schemaVersion，保存配置时 normalize + validate；机器人拉取配置时校验 schemaVersion 和字段类型，失败则保留旧配置并上报 lastConfigError；状态上报不再依赖完整 config JSON 能被双方同时解析。

## Current Touch Points

- 后端配置保存/归一化：`internal/wechatadmin/wxbot.go`
  - `normalizeWxbotConfig`
  - `normalizeWxbotConfigValue`
  - `normalizeWxbotBool`
  - `normalizeWxbotInt`
  - `normalizeWxbotStringList`
- 后端现有测试：`internal/wechatadmin/wxbot_test.go`
- 机器人配置定义：`../wechat-hook-bot/bot/config/settings.py`
- 机器人默认配置：`../wechat-hook-bot/bot/config/defaults.py`
- 机器人远程配置：`../wechat-hook-bot/bot/wxbot/control_center.py`
  - `_remote_config_specs`
  - `_filter_remote_config`
  - `WxbotControlService._apply_remote_config`
  - `WxbotControlClient.mark_config_applied`
- 机器人现有测试：`../wechat-hook-bot/tests/test_wxbot_control.py`
- Web 配置页面：`../steam-game-takeover-web/src/pages/WechatWxbotControl.tsx`

## Contract

Add one canonical schema file in the backend repo:

```text
docs/contracts/wxbot-config.schema.json
```

Add one vendored snapshot in the bot repo because the bot builds independently:

```text
../wechat-hook-bot/docs/contracts/wxbot-config.schema.json
```

The schema is the source of truth for remotely controlled sections. Start with `schemaVersion = 1`.

Remote config payload:

```json
{
  "schemaVersion": 1,
  "config": {}
}
```

Applied/status payload:

```json
{
  "botId": "wxbot-main",
  "status": "online",
  "configSchemaVersion": 1,
  "lastConfigError": ""
}
```

Do not include raw secrets in `lastConfigError`, logs, or status responses.

## Task 1: Add the Shared Schema and Compatibility Tests

**Files:**

- Create: `docs/contracts/wxbot-config.schema.json`
- Create: `../wechat-hook-bot/docs/contracts/wxbot-config.schema.json`
- Modify: `../wechat-hook-bot/tests/test_wxbot_control.py`
- Modify: `internal/wechatadmin/wxbot_test.go`

**Steps:**

- [ ] Define schema version 1 for remotely controlled sections only: `bot`, `monitor`, `webhook`, `database`, `logging`, `welcome`, `party_site`, `summary_reminder`, `ai`, `wxbot_control`, `oss`.
- [ ] Assert bot default config validates against the schema.
- [ ] Assert backend normalized default/sample config validates against the schema.
- [ ] Add a tiny hash check script or test command that compares the backend canonical schema and bot vendored snapshot before release.
- [ ] Assert every bot AI config field is accepted by backend normalization.
- [ ] Assert unknown remote-controlled fields fail validation instead of being silently stored.
- [ ] Assert wrong primitive types fail with a readable field path.

**Check:**

```bash
cd ../wechat-hook-bot && pytest -q tests/test_wxbot_control.py
cd ../steam-game-takeover-backend && go test -count=1 ./internal/wechatadmin
```

## Task 2: Make Backend Save/Normalize Version-aware

**Files:**

- Modify: `internal/wechatadmin/wxbot.go`
- Modify: `internal/wechatadmin/wxbot_test.go`

**Steps:**

- [ ] Accept both legacy raw config and new `{schemaVersion, config}` payload during migration.
- [ ] Normalize storage to include `schemaVersion: 1`.
- [ ] Reject unsupported future versions.
- [ ] Fill missing known fields from backend defaults before saving.
- [ ] Reject unknown fields in schema-owned sections.
- [ ] Return a clear validation error that names the bad path, such as `ai.reply_timeout_seconds must be number`.

**Lazy migration rule:** keep legacy payload support only at the save boundary. Internal storage and bot pull should use the versioned shape.

## Task 3: Make Bot Pull/Apply Fail Closed

**Files:**

- Modify: `../wechat-hook-bot/bot/wxbot/control_center.py`
- Modify: `../wechat-hook-bot/bot/config/settings.py`
- Modify: `../wechat-hook-bot/tests/test_wxbot_control.py`

**Steps:**

- [ ] Read `schemaVersion` before applying `config`.
- [ ] Reject unsupported versions and keep the last working runtime config.
- [ ] Validate/filter config before mutating `app_config`.
- [ ] Store the latest config error in memory for status reporting.
- [ ] Do not log or report secret values.
- [ ] Keep heartbeat/status reporting alive when config apply fails.

## Task 4: Decouple Status From Full Config Parsing

**Files:**

- Modify: `internal/wechatadmin/wxbot.go`
- Modify: `../wechat-hook-bot/bot/wxbot/control_center.py`
- Modify: related status tests in both repos.

**Steps:**

- [ ] Add `configSchemaVersion` and `lastConfigError` to bot status/applied-config payloads.
- [ ] Make backend status handling store/display config error independently from online/offline heartbeat.
- [ ] Ensure config apply failure reports a visible unhealthy config state, not only an offline state.
- [ ] Keep existing online calculation based on heartbeat timestamps.

## Task 5: Update Web Config Editor

**Files:**

- Modify: `../steam-game-takeover-web/src/pages/WechatWxbotControl.tsx`

**Steps:**

- [ ] Send the versioned payload shape on save.
- [ ] Show backend validation errors directly beside the config save action.
- [ ] Display `lastConfigError` in the bot status area when present.
- [ ] Avoid duplicating schema logic in the browser; the browser submits, backend validates.

## Task 6: Deployment Order

- [ ] Deploy backend first. It must accept both legacy and versioned save payloads.
- [ ] Deploy Web second. It starts sending versioned payloads and displaying config errors.
- [ ] Deploy/auto-update bot last. It starts enforcing schemaVersion.
- [ ] After all three are deployed, remove legacy raw-config save support in a later cleanup only if production has no old clients.

## Acceptance Criteria

- Adding a new `ai.*` field without updating the contract fails tests.
- Backend refuses to save wrongly typed config.
- Bot refuses incompatible config without losing heartbeat.
- Admin page can distinguish `在线但配置失败` from `离线`.
- Existing wxbot config save, pull, applied acknowledgement, and heartbeat still work.

## Skipped For Now

- No Go/Python code generation from schema in this version.
- No database migration that merges bot config with any other system config.
- No new Web page; reuse the existing wxbot control/config page.
