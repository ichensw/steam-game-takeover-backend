# KOOK Channel Occupancy And Auto Sort Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve KOOK aggregate user usage duration, add true channel occupancy duration, and safely redistribute selected groups' voice channels by occupancy on a configurable schedule with confirmed manual movement.

**Architecture:** The backend adds two pure units: interval union for occupancy and a stable cross-group sort planner. A database-backed configuration, execution ledger, and lease coordinate a KOOK gateway that previews, applies, retries, and rolls back plans. The React admin consumes typed APIs through focused sort and move components while the existing `KookChannels` page remains the data owner.

**Tech Stack:** Go 1.22, Gin, GORM, MySQL, Resty, React 19, TypeScript, Ant Design 6, Vitest, Ant Design draggable Tree.

## Global Constraints

- Existing `durationSeconds` and `durationText` remain aggregate user usage duration and must not change meaning.
- Add `occupiedDurationSeconds` and `occupiedDurationText`; one channel minute counts once regardless of user count.
- The channel page shows usage duration, occupancy duration, and active users; it does not show session count.
- All schedule calculations use `Asia/Shanghai` and the previous complete natural day, Monday-to-Sunday week, or calendar month.
- Selected groups share one schedule and are ordered from KOOK live channel order at execution time.
- Voice channels are globally sorted by occupancy descending; ties preserve pre-run global order.
- Fill 15 voice channels into each selected group in order; the final selected group receives every overflow channel.
- Non-voice channels are never ranked or directly moved by automatic sorting.
- Monthly days absent from a month run on that month's last day; execution minute is always `00`.
- Dragging never mutates remotely until the administrator confirms the generated move.
- Automatic sorting, immediate execution, and manual movement share a database lease.
- Existing routes and API meanings remain backward compatible.

---

### Task 1: Add Channel Occupancy Duration Without Changing Usage Duration

**Files:**
- Create: `internal/httpapi/kook_voice_occupancy.go`
- Create: `internal/httpapi/kook_voice_occupancy_test.go`
- Modify: `internal/httpapi/kook_voice_stats.go`

**Interfaces:**
- Produces: `mergeKookVoiceIntervals(intervals []kookVoiceInterval, rangeStart, rangeEnd time.Time) map[kookVoiceChannelKey]int64`, keyed by guild and channel.
- Extends: `kookVoiceChannelUsageDTO` with `OccupiedDurationSeconds int64` and `OccupiedDurationText string` JSON fields.
- Preserves: existing SQL sum and `durationSeconds` semantics.

- [ ] Write table-driven failing tests with `kookVoiceInterval{GuildID, ChannelID, JoinedAt, ExitedAt}` for five fully overlapping one-minute sessions, partial overlap, touching ranges, disjoint ranges, clipping at both boundaries, and an active nil exit.

```go
func TestMergeKookVoiceIntervalsCountsChannelOccupancyOnce(t *testing.T) {
    start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
    end := start.Add(time.Hour)
    intervals := make([]kookVoiceInterval, 5)
    for i := range intervals {
        exited := start.Add(time.Minute)
        intervals[i] = kookVoiceInterval{GuildID: "guild-1", ChannelID: "voice-1", JoinedAt: start, ExitedAt: &exited}
    }
    got := mergeKookVoiceIntervals(intervals, start, end)
    key := kookVoiceChannelKey{GuildID: "guild-1", ChannelID: "voice-1"}
    if got[key] != 60 { t.Fatalf("occupied = %d, want 60", got[key]) }
}
```

- [ ] Run `go test ./internal/httpapi -run 'TestMergeKookVoiceIntervals' -count=1`; expect failure because the interval type and merger do not exist.
- [ ] Implement clipping, per-channel sorting, and interval union in `kook_voice_occupancy.go`; the pure merger treats nil exit as its supplied range end, and the query layer supplies `min(time.Now(), requestedEnd)` as that effective end.
- [ ] Extend `kookVoiceChannelUsageSummary` to query overlapping intervals for the configured guild, call the merger, and attach occupied duration fields while leaving the existing aggregate usage query unchanged.
- [ ] Add a regression assertion that five one-minute sessions retain `durationSeconds == 300` while `occupiedDurationSeconds == 60` at the DTO assembly boundary.
- [ ] Run `go test ./internal/httpapi -run 'TestMergeKookVoiceIntervals|TestKookVoiceUsage' -count=1` and then `go test -count=1 ./...`; expect all tests to pass.
- [ ] Commit with `git commit -m "feat: add KOOK channel occupancy duration"`.

### Task 2: Implement Stable Group Distribution And Schedule Calculations

**Files:**
- Create: `internal/httpapi/kook_channel_sort_planner.go`
- Create: `internal/httpapi/kook_channel_sort_planner_test.go`

**Interfaces:**
- Consumes: `kookChannelDTO.ID`, `Type`, `ParentID`, and `KookSort`; `KookSort` is the KOOK API order while `Level` remains tree depth.
- Produces: `buildKookChannelSortPlan(channels []kookChannelDTO, selectedGroupIDs []string, usage map[string]kookChannelSortUsage) (kookChannelSortPlan, error)`.
- Produces: `previousKookChannelSortRange(scheduleType string, now time.Time, location *time.Location) (time.Time, time.Time, error)`.
- Produces: `nextKookChannelSortRun(now time.Time, schedule kookChannelSortSchedule, location *time.Location) (time.Time, error)`.

- [ ] Write failing planner tests using five ordered category DTOs and 78 voice channels. Assert groups 1-4 receive 15 each, group 5 receives 18, higher occupancy ranks first, equal occupancy retains original global order, and text channels never appear in `Moves`.

```go
type kookChannelSortUsage struct { UsageSeconds, OccupiedSeconds int64 }
type kookChannelMove struct {
    ChannelID, ChannelName, FromParentID, ToParentID string
    FromLevel, ToLevel int
    UsageSeconds, OccupiedSeconds int64
}
type kookChannelSortPlan struct {
    Groups []kookChannelSortGroup
    Moves []kookChannelMove
}
```

- [ ] Run `go test ./internal/httpapi -run 'TestBuildKookChannelSortPlan' -count=1`; expect undefined planner symbols.
- [ ] Implement live group ordering by `KookSort`, stable voice ordering by occupied duration, 15-channel distribution, final-group overflow, and change-only move generation.
- [ ] Write schedule tests for daily prior day, weekly prior Monday-Sunday, monthly prior calendar month, weekly weekday execution, and February fallback for configured day 31.
- [ ] Implement schedule validation and calculations with `Asia/Shanghai`; saving after the current period's due hour returns the next period, not an immediate catch-up.
- [ ] Run the focused planner and schedule tests, then `go test -count=1 ./...`; expect all tests to pass.
- [ ] Commit with `git commit -m "feat: plan KOOK channel sorting"`.

### Task 3: Add Sort Configuration, Run Ledger, Lease, And Admin APIs

**Files:**
- Create: `migrations/039_add_kook_channel_sort.sql`
- Modify: `internal/model/model.go`
- Create: `internal/httpapi/kook_channel_sort_store.go`
- Create: `internal/httpapi/kook_channel_sort_store_test.go`
- Create: `internal/httpapi/kook_channel_sort_migration_test.go`
- Create: `internal/httpapi/kook_channel_sort_handlers.go`
- Modify: `internal/httpapi/router.go`

**Interfaces:**
- Produces models `model.KookChannelSortConfig` and `model.KookChannelSortRun` matching migration columns.
- Produces `kookChannelSortConfigDTO` with `enabled`, `groupIds`, `scheduleType`, `weekday`, `monthday`, `hour`, and `nextRunAt`.
- Produces lease methods `acquireKookChannelSortLease(owner string, ttl time.Duration) (bool, error)`, `renewKookChannelSortLease`, and `releaseKookChannelSortLease`.

- [ ] Write `migrations/039_add_kook_channel_sort.sql` tests that assert both InnoDB tables, the singleton config seed, JSON/longtext snapshots, unique execution key, status fields, and lease fields exist.
- [ ] Create the migration and matching GORM models. Store group IDs and run snapshots as JSON text; do not use the 255-character app-config table.
- [ ] Write failing validation tests: enabled configuration requires at least one group, valid schedule type, weekday `1..7` only for weekly, monthday `1..31` only for monthly, and hour `0..23`.
- [ ] Implement config serialization, validation, singleton upsert, next-run calculation, recent-run summary, and atomic token-based lease acquisition using `UPDATE ... WHERE locked_until IS NULL OR locked_until < NOW()`.
- [ ] Add the config and run-history KOOK-admin routes completed by this task:

```go
kookAdmin.GET("/kook-channel-sort/config", h.AdminGetKookChannelSortConfig)
kookAdmin.PUT("/kook-channel-sort/config", h.AdminUpdateKookChannelSortConfig)
kookAdmin.GET("/kook-channel-sort/runs", h.AdminListKookChannelSortRuns)
```

- [ ] Run store, validation, router, and full backend tests; expect all to pass.
- [ ] Commit with `git commit -m "feat: persist KOOK channel sort configuration"`.

### Task 4: Execute, Retry, Roll Back, And Schedule KOOK Moves

**Files:**
- Create: `internal/httpapi/kook_channel_gateway.go`
- Create: `internal/httpapi/kook_channel_sort_executor.go`
- Create: `internal/httpapi/kook_channel_sort_executor_test.go`
- Create: `internal/httpapi/kook_channel_sort_worker.go`
- Create: `internal/httpapi/kook_channel_sort_worker_test.go`
- Modify: `internal/httpapi/kook_channel_sort_handlers.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Produces interface `kookChannelGateway` with `ListChannels(context.Context) ([]kookChannelDTO, error)` and `UpdateChannel(context.Context, kookChannelPosition) error`.
- Produces `executeKookChannelSort(ctx context.Context, trigger string) (model.KookChannelSortRun, error)`.
- Produces `StartKookChannelSortWorker(ctx context.Context)`.

- [ ] Build a fake gateway and write failing tests that verify preview makes zero update calls, successful execution sends only changed moves, 429/network failures retry at most twice, business errors do not retry, and a mid-plan failure rolls completed moves back in reverse order.
- [ ] Implement the gateway using the existing dynamic KOOK token, `fetchKookChannels`, and `POST /channel/update` with `channel_id`, `parent_id`, and `level`. Preserve the existing Gin proxy behavior for ordinary edits.
- [ ] Implement usage loading, pure plan invocation, run snapshot persistence, serial updates, retry classification/backoff, reverse rollback, and `failed` versus `rollback_failed` results.
- [ ] Implement manual movement request `{targetParentId, placement, anchorChannelId}` for `top`, `bottom`, `before`, and `after`; resolve the final level from KOOK live siblings and use the shared lease and gateway.
- [ ] Register `POST /admin/kook-channel-sort/preview`, `POST /admin/kook-channel-sort/run`, and `POST /admin/kook-channels/:channelId/move` under the existing KOOK-admin role group.
- [ ] Write worker tests with an injected clock/tick channel. Assert due jobs run once, disabled jobs do not run, unique execution keys suppress duplicates, and next-run time advances after success or failure.
- [ ] Implement a one-minute worker loop and start it from `cmd/server/main.go` after existing workers.
- [ ] Run executor, worker, handler, and full backend tests; run `go vet ./...`; expect clean output.
- [ ] Commit with `git commit -m "feat: execute scheduled KOOK channel sorting"`.

### Task 5: Add Typed Frontend APIs And Pure Move Helpers

**Files:**
- Modify: `../steam-game-takeover-web/src/api/admin.ts`
- Create: `../steam-game-takeover-web/src/utils/kookChannelMove.ts`
- Create: `../steam-game-takeover-web/src/utils/kookChannelMove.test.ts`

**Interfaces:**
- Extends `KookChannelUsage` with `occupiedDurationSeconds` and `occupiedDurationText` while retaining existing fields.
- Produces `KookChannelSortConfig`, `KookChannelSortPlan`, `KookChannelSortRun`, and API wrapper functions for all sort and move routes from Tasks 3 and 4.
- Produces `buildKookMoveCandidate(source, target, mode, disabled): KookChannelMoveRequest | null`, where mode is `inside`, `before`, or `after` from Ant Design Tree drop metadata.

- [ ] Write failing Vitest cases for dropping a channel onto a category, before/after a sibling based on pointer half, root-level category reorder, invalid category nesting, and filtered-list disablement.
- [ ] Run `npm test -- src/utils/kookChannelMove.test.ts`; expect failure because the helper does not exist.
- [ ] Implement typed API contracts and the pure move-candidate helper without adding a drag dependency.
- [ ] Keep `sessionCount` in the API type because `Dashboard.tsx` still consumes it; remove it only from the KOOK channel management column configuration in Task 7.
- [ ] Run focused and full frontend tests; expect all to pass.
- [ ] Commit in the frontend repository with `git commit -m "feat: add KOOK channel sorting APIs"`.

### Task 6: Build The Automatic Sort Configuration And Preview UI

**Files:**
- Create: `../steam-game-takeover-web/src/components/KookChannelSortDrawer.tsx`
- Modify: `../steam-game-takeover-web/src/pages/KookChannels.tsx`
- Modify: `../steam-game-takeover-web/src/styles.css`

**Interfaces:**
- Consumes typed API functions and current category rows.
- Emits `onCompleted()` after configuration save or immediate execution so the page reloads channels and usage.

- [ ] Add an “自动排序设置” toolbar button and a focused drawer with enabled switch, live category multi-select, schedule segmented control/select, conditional weekday/monthday fields, hour selector, next-run summary, and recent run table.
- [ ] Implement “预览排序” to render group order and move rows with current/target location plus both durations; preview must not call the run or move APIs.
- [ ] Implement “立即执行” with a confirmation modal containing range and move count, then refresh config, runs, channels, and usage on success.
- [ ] Add dense responsive styles using existing tokens; the drawer uses one column on narrow screens and tables scroll rather than overlap.
- [ ] Run `npm test` and `npm run build`; expect both to pass.
- [ ] Commit with `git commit -m "feat: configure KOOK channel auto sorting"`.

### Task 7: Add Confirmed Move Modal And Tree Dragging

**Files:**
- Create: `../steam-game-takeover-web/src/components/KookChannelMoveModal.tsx`
- Modify: `../steam-game-takeover-web/src/pages/KookChannels.tsx`
- Modify: `../steam-game-takeover-web/src/styles.css`

**Interfaces:**
- Consumes current flat channels and `moveKookChannel(channelId, request)`.
- Supports target group plus `top`, `bottom`, `before`, and `after` placement.
- Ant Design `Tree` drag creates the same request and confirmation content as the modal.

- [ ] Add a “移动” row action that opens a modal with a target group selector, placement selector, and context-dependent anchor channel selector.
- [ ] Replace the KOOK channel management `sessionCount` column definition/order/width with `occupiedDurationSeconds`, while retaining `durationSeconds` and `activeUserCount`.
- [ ] Add an “调整频道结构” drawer containing Ant Design `<Tree draggable>` built from the complete channel list. Ordinary channels may drop into categories or before/after siblings; categories may only reorder at root. Disable drag while a keyword filter is active.
- [ ] On tree drop, show original group, target group, and target position in `modal.confirm`; call no API and make no optimistic row mutation before confirmation.
- [ ] On confirmation, call the dedicated move route and reload live channel structure and both duration metrics; on cancellation clear drag state only.
- [ ] Add restrained drop-target styling and ensure action buttons and row text remain usable on mobile.
- [ ] Run `npm test` and `npm run build`; expect both to pass.
- [ ] Commit with `git commit -m "feat: move KOOK channels with confirmation"`.

### Task 8: End-To-End Verification And Production Release

**Files:**
- No tracked source changes expected.

**Interfaces:**
- Consumes tested backend and frontend commits.
- Produces migrated, deployed production services with the scheduler initially disabled until a preview and confirmed manual run succeed.

- [ ] Run `go test -count=1 ./...`, `go vet ./...`, frontend `npm test`, and frontend `npm run build`; expect clean success.
- [ ] Inspect both repositories with `git diff --check`, `git status --short --branch`, and recent commit logs; confirm no secrets, build output, or internal task artifacts are tracked.
- [ ] Push backend `main` and frontend `master`.
- [ ] Back up the current backend binary and frontend `dist`, then apply `migrations/039_add_kook_channel_sort.sql` before starting the new backend binary.
- [ ] Confirm the backend service is active and `/api/health` returns HTTP 200; deploy the verified frontend `dist`.
- [ ] Log into production, open KOOK channel management, verify both durations and confirm the default configuration is disabled.
- [ ] Do not select production groups, execute a move plan, or enable the schedule until the administrator supplies the intended group selection and schedule; deployment itself must not reorder live KOOK channels.
- [ ] Run `nginx -t`; do not modify or reload Nginx unless the unchanged `/miniprogram-api/api` route fails.
