# 队伍成员拉黑准入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 阻止用户加入任何当前成员已将其拉黑的接龙，同时不泄露限制者身份。

**Architecture:** 在 `JoinTakeover` 已持有的接龙行锁事务中，保留创建者拉黑校验，并新增一次成员表与拉黑表的存在性查询。该查询仅匹配 `member_state = joined` 的成员；命中后通过新的通用错误码返回固定提示词。

**Tech Stack:** Go 1.22, Gin, GORM, MySQL, Go testing

## Global Constraints

- 不新增数据库表、迁移、依赖或小程序页面改动。
- 拉黑为单向关系，仅当前已加入成员影响申请人的加入资格。
- 失败响应必须为 `当前接龙暂时无法加入，请稍后重试。`，不得暴露限制者身份或“拉黑”原因。
- 复用现有接龙行锁事务，避免成员状态与加入校验并发交错。

---

### Task 1: 加入校验与通用响应

**Files:**
- Modify: `internal/httpapi/takeover_member_control_handlers.go:14-19,133-142`
- Modify: `internal/httpapi/takeover_handlers.go:842-848,899-900`
- Modify: `internal/httpapi/response.go:18,66`
- Modify: `internal/httpapi/user_block_handlers_test.go:3-41`

**Interfaces:**
- Consumes: `model.MemberStateJoined`, `model.UserBlock`, `JoinTakeover` 已锁定的 `*gorm.DB` 事务。
- Produces: `isUserBlockedByActiveTakeoverMember(db, takeoverID, blockedUserID) (bool, error)`；`CodeTakeoverJoinUnavailable`。

- [ ] **Step 1: 写失败测试，固定成员拉黑查询与通用文案**

在 `internal/httpapi/user_block_handlers_test.go` 增加 `strings` 和 `gorm.io/gorm` 导入，并加入：

```go
func TestActiveTakeoverMemberBlockQueryOnlyMatchesJoinedMembers(t *testing.T) {
	db := newTakeoverExpirationDryRunDB(t)
	sqlText := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return activeTakeoverMemberBlockQuery(tx, 12, 34).Limit(1).Count(new(int64))
	})
	for _, clause := range []string{
		"FROM ttw_takeover_member AS m",
		"JOIN ttw_user_block AS b ON b.owner_user_id = m.user_id",
		"m.takeover_id = 12",
		"m.member_state = 1",
		"b.blocked_user_id = 34",
	} {
		if !strings.Contains(sqlText, clause) {
			t.Fatalf("member block SQL missing %q: %s", clause, sqlText)
		}
	}
}

func TestTakeoverJoinUnavailableMessageIsGeneric(t *testing.T) {
	if got := friendlyMessage(CodeTakeoverJoinUnavailable, ""); got != "当前接龙暂时无法加入，请稍后重试。" {
		t.Fatalf("friendlyMessage() = %q", got)
	}
}
```

- [ ] **Step 2: 运行测试，确认因缺少查询函数和错误码而失败**

Run: `go test ./internal/httpapi -run 'Test(ActiveTakeoverMemberBlockQueryOnlyMatchesJoinedMembers|TakeoverJoinUnavailableMessageIsGeneric)$'`

Expected: FAIL，提示 `activeTakeoverMemberBlockQuery` 和 `CodeTakeoverJoinUnavailable` 未定义。

- [ ] **Step 3: 写最小查询与错误码实现**

在 `internal/httpapi/takeover_member_control_handlers.go` 用 `errTakeoverJoinUnavailable` 替换 `errUserBlockedByCreator`，并在 `isUserBlockedBy` 后加入：

```go
func activeTakeoverMemberBlockQuery(db *gorm.DB, takeoverID, blockedUserID uint64) *gorm.DB {
	return db.Table("ttw_takeover_member AS m").
		Joins("JOIN ttw_user_block AS b ON b.owner_user_id = m.user_id").
		Where("m.takeover_id = ? AND m.member_state = ? AND b.blocked_user_id = ?", takeoverID, model.MemberStateJoined, blockedUserID)
}

func isUserBlockedByActiveTakeoverMember(db *gorm.DB, takeoverID, blockedUserID uint64) (bool, error) {
	var count int64
	err := activeTakeoverMemberBlockQuery(db, takeoverID, blockedUserID).Limit(1).Count(&count).Error
	return count > 0, err
}
```

在 `internal/httpapi/takeover_handlers.go` 的创建者校验后加入：

```go
if blocked, err := isUserBlockedByActiveTakeoverMember(tx, takeoverID, freshUser.ID); err != nil {
	return err
} else if blocked {
	return errTakeoverJoinUnavailable
}
```

并将创建者校验命中时也返回 `errTakeoverJoinUnavailable`。错误分支使用：

```go
case errors.Is(err, errTakeoverJoinUnavailable):
	fail(c, http.StatusForbidden, CodeTakeoverJoinUnavailable, errTakeoverJoinUnavailable.Error())
```

在 `internal/httpapi/response.go` 用以下常量和翻译替换 `CodeUserBlockedByCreator`：

```go
CodeTakeoverJoinUnavailable = "TAKEOVER_JOIN_UNAVAILABLE"
```

```go
CodeTakeoverJoinUnavailable: "当前接龙暂时无法加入，请稍后重试。",
```

- [ ] **Step 4: 运行针对性测试，确认查询和文案通过**

Run: `go test ./internal/httpapi -run 'Test(ActiveTakeoverMemberBlockQueryOnlyMatchesJoinedMembers|TakeoverJoinUnavailableMessageIsGeneric)$'`

Expected: PASS。

- [ ] **Step 5: 运行完整后端测试与静态检查**

Run: `gofmt -w internal/httpapi/takeover_member_control_handlers.go internal/httpapi/takeover_handlers.go internal/httpapi/response.go internal/httpapi/user_block_handlers_test.go && go test ./... && git diff --check`

Expected: PASS，且 `git diff --check` 无输出。

- [ ] **Step 6: 提交实现**

```bash
git add internal/httpapi/takeover_member_control_handlers.go internal/httpapi/takeover_handlers.go internal/httpapi/response.go internal/httpapi/user_block_handlers_test.go
git commit -m "feat: enforce member block on takeover joins"
```
