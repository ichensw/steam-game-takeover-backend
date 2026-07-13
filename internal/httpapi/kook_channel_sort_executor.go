package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"steam-game-takeover-backend/internal/model"
)

const kookChannelSortLeaseTTL = 5 * time.Minute

var kookChannelSortSleep = time.Sleep

type kookChannelSortApplyResult struct {
	MovedCount               int
	Err                      error
	RollbackFailedChannelIDs []string
}

type kookChannelSortPreviewDTO struct {
	Range     map[string]string         `json:"range"`
	Groups    []kookChannelSortGroupDTO `json:"groups"`
	Moves     []kookChannelSortMoveDTO  `json:"moves"`
	MoveCount int                       `json:"moveCount"`
}

type kookChannelSortGroupDTO struct {
	GroupID      string `json:"groupId"`
	GroupName    string `json:"groupName"`
	Order        int    `json:"order"`
	ChannelCount int    `json:"channelCount"`
}

type kookChannelSortMoveDTO struct {
	ChannelID               string `json:"channelId"`
	ChannelName             string `json:"channelName"`
	FromParentID            string `json:"fromParentId"`
	FromParentName          string `json:"fromParentName"`
	ToParentID              string `json:"toParentId"`
	ToParentName            string `json:"toParentName"`
	FromLevel               int    `json:"fromLevel"`
	ToLevel                 int    `json:"toLevel"`
	UsageSeconds            int64  `json:"usageSeconds"`
	UsageText               string `json:"usageText"`
	OccupiedDurationSeconds int64  `json:"occupiedDurationSeconds"`
	OccupiedDurationText    string `json:"occupiedDurationText"`
}

func (h *Handler) planKookChannelSort(ctx context.Context, gateway kookChannelGateway) (kookChannelSortPlan, time.Time, time.Time, error) {
	config, err := h.getKookChannelSortConfig()
	if err != nil {
		return kookChannelSortPlan{}, time.Time{}, time.Time{}, err
	}
	location, err := loadKookChannelSortLocation()
	if err != nil {
		return kookChannelSortPlan{}, time.Time{}, time.Time{}, err
	}
	start, end, err := previousKookChannelSortRange(config.ScheduleType, time.Now(), location)
	if err != nil {
		return kookChannelSortPlan{}, time.Time{}, time.Time{}, err
	}
	channels, err := gateway.ListChannels(ctx)
	if err != nil {
		return kookChannelSortPlan{}, time.Time{}, time.Time{}, err
	}
	usageRows, err := h.kookVoiceChannelUsageSummary(start, end)
	if err != nil {
		return kookChannelSortPlan{}, time.Time{}, time.Time{}, err
	}
	usage := make(map[string]kookChannelSortUsage, len(usageRows))
	for _, row := range usageRows {
		usage[row.ChannelID] = kookChannelSortUsage{UsageSeconds: row.DurationSeconds, OccupiedSeconds: row.OccupiedDurationSeconds}
	}
	plan, err := buildKookChannelSortPlan(channels, config.GroupIDs, usage)
	return plan, start, end, err
}

func previewKookChannelSort(ctx context.Context, gateway kookChannelGateway, groupIDs []string, usage map[string]kookChannelSortUsage) (kookChannelSortPlan, error) {
	channels, err := gateway.ListChannels(ctx)
	if err != nil {
		return kookChannelSortPlan{}, err
	}
	return buildKookChannelSortPlan(channels, groupIDs, usage)
}

func kookChannelSortPreview(plan kookChannelSortPlan, start, end time.Time) kookChannelSortPreviewDTO {
	names := make(map[string]string, len(plan.Groups))
	groups := make([]kookChannelSortGroupDTO, 0, len(plan.Groups))
	for i, group := range plan.Groups {
		names[group.ID] = group.Name
		groups = append(groups, kookChannelSortGroupDTO{GroupID: group.ID, GroupName: group.Name, Order: i + 1, ChannelCount: len(group.Channels)})
	}
	moves := make([]kookChannelSortMoveDTO, 0, len(plan.Moves))
	for _, move := range plan.Moves {
		moves = append(moves, kookChannelSortMoveDTO{
			ChannelID: move.ChannelID, ChannelName: move.ChannelName,
			FromParentID: move.FromParentID, FromParentName: names[move.FromParentID],
			ToParentID: move.ToParentID, ToParentName: names[move.ToParentID],
			FromLevel: move.FromLevel, ToLevel: move.ToLevel,
			UsageSeconds: move.UsageSeconds, UsageText: durationText(move.UsageSeconds),
			OccupiedDurationSeconds: move.OccupiedSeconds, OccupiedDurationText: durationText(move.OccupiedSeconds),
		})
	}
	return kookChannelSortPreviewDTO{
		Range:  map[string]string{"startTime": start.Format("2006-01-02 15:04:05"), "endTime": end.Format("2006-01-02 15:04:05")},
		Groups: groups, Moves: moves, MoveCount: len(moves),
	}
}

func updateKookChannelWithRetry(ctx context.Context, gateway kookChannelGateway, position kookChannelPosition, wait func(context.Context, time.Duration) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = gateway.UpdateChannel(ctx, position); err == nil {
			return nil
		}
		var gatewayErr *kookChannelGatewayError
		if errors.As(err, &gatewayErr) && gatewayErr.StatusCode != 429 && gatewayErr.StatusCode < 500 {
			return err
		}
		if attempt == 2 {
			return err
		}
		if wait != nil {
			if waitErr := wait(ctx, time.Duration(attempt+1)*100*time.Millisecond); waitErr != nil {
				return waitErr
			}
		} else {
			kookChannelSortSleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	return err
}

func applyKookChannelSortPlan(ctx context.Context, gateway kookChannelGateway, plan kookChannelSortPlan, wait func(context.Context, time.Duration) error, renew func() error) kookChannelSortApplyResult {
	completed := make([]kookChannelMove, 0, len(plan.Moves))
	for _, move := range plan.Moves {
		if move.FromParentID == move.ToParentID && move.FromLevel == move.ToLevel {
			continue
		}
		if err := updateKookChannelWithRetry(ctx, gateway, kookChannelPosition{ChannelID: move.ChannelID, ParentID: move.ToParentID, Level: move.ToLevel}, wait); err != nil {
			result := kookChannelSortApplyResult{MovedCount: len(completed), Err: err}
			for i := len(completed) - 1; i >= 0; i-- {
				item := completed[i]
				if rollbackErr := updateKookChannelWithRetry(ctx, gateway, kookChannelPosition{ChannelID: item.ChannelID, ParentID: item.FromParentID, Level: item.FromLevel}, wait); rollbackErr != nil {
					result.RollbackFailedChannelIDs = append(result.RollbackFailedChannelIDs, item.ChannelID)
				}
			}
			return result
		}
		completed = append(completed, move)
		if renew != nil {
			if err := renew(); err != nil {
				result := kookChannelSortApplyResult{MovedCount: len(completed), Err: err}
				for i := len(completed) - 1; i >= 0; i-- {
					item := completed[i]
					if rollbackErr := updateKookChannelWithRetry(ctx, gateway, kookChannelPosition{ChannelID: item.ChannelID, ParentID: item.FromParentID, Level: item.FromLevel}, wait); rollbackErr != nil {
						result.RollbackFailedChannelIDs = append(result.RollbackFailedChannelIDs, item.ChannelID)
					}
				}
				return result
			}
		}
	}
	return kookChannelSortApplyResult{MovedCount: len(completed)}
}

func applyKookChannelMoves(ctx context.Context, gateway kookChannelGateway, moves []kookChannelMove, renew func() error) (int, error, error) {
	result := applyKookChannelSortPlan(ctx, gateway, kookChannelSortPlan{Moves: moves}, nil, renew)
	if len(result.RollbackFailedChannelIDs) > 0 {
		return result.MovedCount, result.Err, fmt.Errorf("rollback failed for %v", result.RollbackFailedChannelIDs)
	}
	return result.MovedCount, result.Err, nil
}

func (h *Handler) executeKookChannelSort(ctx context.Context, trigger string, executionKey *string) (model.KookChannelSortRun, error) {
	owner := fmt.Sprintf("%s-%d", trigger, time.Now().UnixNano())
	acquired, err := h.acquireKookChannelSortLease(owner, kookChannelSortLeaseTTL)
	if err != nil || !acquired {
		if err == nil {
			err = fmt.Errorf("KOOK channel movement is busy")
		}
		return model.KookChannelSortRun{}, err
	}
	defer h.releaseKookChannelSortLease(owner) //nolint:errcheck

	gateway := kookHTTPGateway{h: h}
	plan, start, end, err := h.planKookChannelSort(ctx, gateway)
	if err != nil {
		return model.KookChannelSortRun{}, err
	}
	groupJSON, _ := json.Marshal(plan.Groups)
	planJSON, _ := json.Marshal(plan)
	run := model.KookChannelSortRun{
		Trigger: trigger, ExecutionKey: executionKey, RangeStart: start, RangeEnd: end,
		GroupSnapshot: string(groupJSON), PlanSnapshot: string(planJSON), Status: "running",
		PlannedCount: len(plan.Moves), StartedAt: time.Now(),
	}
	if err := h.db.Create(&run).Error; err != nil {
		return run, err
	}
	moved, moveErr, rollbackErr := applyKookChannelMoves(ctx, gateway, plan.Moves, func() error {
		ok, err := h.renewKookChannelSortLease(owner, kookChannelSortLeaseTTL)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("KOOK channel movement lease was lost")
		}
		return nil
	})
	now := time.Now()
	run.MovedCount, run.FinishedAt = moved, &now
	if moveErr == nil {
		run.Status = "succeeded"
	} else {
		run.Status = "failed"
		message := moveErr.Error()
		if rollbackErr != nil {
			run.Status = "rollback_failed"
			message += "; rollback: " + rollbackErr.Error()
		}
		run.ErrorMessage = &message
	}
	h.db.Save(&run)
	return run, moveErr
}

type kookChannelMoveRequest struct {
	TargetParentID  string `json:"targetParentId" binding:"required"`
	Placement       string `json:"placement" binding:"required"`
	AnchorChannelID string `json:"anchorChannelId"`
}

func resolveKookManualPosition(channels []kookChannelDTO, channelID string, request kookChannelMoveRequest) (kookChannelPosition, error) {
	var source *kookChannelDTO
	for i := range channels {
		if channels[i].ID == channelID {
			source = &channels[i]
			break
		}
	}
	if source == nil {
		return kookChannelPosition{}, fmt.Errorf("channel not found")
	}
	siblings := make([]kookChannelDTO, 0)
	for _, item := range channels {
		if item.ID != channelID && item.ParentID == request.TargetParentID {
			siblings = append(siblings, item)
		}
	}
	sort.SliceStable(siblings, func(i, j int) bool { return siblings[i].KookSort < siblings[j].KookSort })
	level := 100
	switch request.Placement {
	case "top":
		if len(siblings) > 0 {
			level = siblings[0].KookSort - 100
		}
	case "bottom":
		if len(siblings) > 0 {
			level = siblings[len(siblings)-1].KookSort + 100
		}
	case "before", "after":
		anchorIndex := -1
		for i := range siblings {
			if siblings[i].ID == request.AnchorChannelID {
				anchorIndex = i
				break
			}
		}
		if anchorIndex < 0 {
			return kookChannelPosition{}, fmt.Errorf("anchor channel not found in target group")
		}
		level = siblings[anchorIndex].KookSort
		if request.Placement == "before" {
			level--
		} else {
			level++
		}
	default:
		return kookChannelPosition{}, fmt.Errorf("invalid placement")
	}
	return kookChannelPosition{ChannelID: channelID, ParentID: request.TargetParentID, Level: level}, nil
}
