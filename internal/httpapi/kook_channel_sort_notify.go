package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"steam-game-takeover-backend/internal/model"

	"github.com/go-resty/resty/v2"
)

const kookChannelManagePermission = int64(32)

func kookChannelSortPermissionValue(value interface{}) int64 {
	if number, ok := value.(float64); ok {
		return int64(number)
	}
	if number, ok := value.(json.Number); ok {
		parsed, _ := strconv.ParseInt(number.String(), 10, 64)
		return parsed
	}
	parsed, _ := strconv.ParseInt(stringFromAny(value), 10, 64)
	return parsed
}

func hasKookChannelAdminPermission(row map[string]interface{}) bool {
	allow := kookChannelSortPermissionValue(row["allow"])
	deny := kookChannelSortPermissionValue(row["deny"])
	return allow&kookChannelManagePermission != 0 && deny&kookChannelManagePermission == 0 ||
		allow&1 != 0 && deny&1 == 0
}

func kookChannelSortRows(value interface{}) []map[string]interface{} {
	items, _ := value.([]interface{})
	rows := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if row, ok := item.(map[string]interface{}); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func kookChannelSortAdminUserIDs(data interface{}, members []model.KookMember) []string {
	root, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	ids := make([]string, 0)
	seen := map[string]struct{}{}
	add := func(id string) {
		if id != "" {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	for _, row := range kookChannelSortRows(root["permission_users"]) {
		if !hasKookChannelAdminPermission(row) {
			continue
		}
		add(kookRoleUserID(row))
	}
	roleIDs := map[string]struct{}{}
	for _, row := range kookChannelSortRows(root["permission_overwrites"]) {
		if hasKookChannelAdminPermission(row) {
			if roleID := stringFromAny(row["role_id"]); roleID != "" {
				roleIDs[roleID] = struct{}{}
			}
		}
	}
	for _, member := range members {
		for _, roleID := range kookRoleIDsFromJSON(member.RoleIDs) {
			if _, ok := roleIDs[roleID]; ok {
				add(member.KookUserID)
				break
			}
		}
	}
	return ids
}

func (h *Handler) fetchKookChannelSortPermissions(ctx context.Context, channelID string) (interface{}, error) {
	var result kookProxyResponse
	resp, err := resty.New().SetTimeout(20*time.Second).R().
		SetContext(ctx).
		SetHeader("Authorization", "Bot "+h.kookBotToken()).
		SetQueryParam("channel_id", channelID).
		SetResult(&result).
		Get(kookAPIBaseURL + "/channel-role/index")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		return nil, fmt.Errorf("kook channel permission query failed: %s", kookProxyErrorMessage(resp.StatusCode(), result))
	}
	return result.Data, nil
}

func (h *Handler) sendKookDirectMessage(ctx context.Context, targetID, content string) error {
	var result kookProxyResponse
	resp, err := resty.New().SetTimeout(20*time.Second).R().
		SetContext(ctx).
		SetHeader("Authorization", "Bot "+h.kookBotToken()).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{"target_id": targetID, "type": 1, "content": content}).
		SetResult(&result).
		Post(kookAPIBaseURL + "/direct-message/create")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK || result.Code != 0 {
		return fmt.Errorf("kook direct message failed: %s", kookProxyErrorMessage(resp.StatusCode(), result))
	}
	return nil
}

func (h *Handler) notifyKookChannelSort(ctx context.Context, plan kookChannelSortPlan) error {
	if h.kookBotToken() == "" || h.db == nil {
		return nil
	}
	names := make(map[string]string, len(plan.Groups))
	for _, group := range plan.Groups {
		names[group.ID] = group.Name
	}
	var members []model.KookMember
	if err := h.db.Where("guild_id = ?", h.kookGuildID()).Find(&members).Error; err != nil {
		return err
	}
	messages := map[string][]string{}
	for _, move := range plan.Moves {
		if move.FromParentID == move.ToParentID && move.FromLevel == move.ToLevel {
			continue
		}
		permissions, err := h.fetchKookChannelSortPermissions(ctx, move.ChannelID)
		if err != nil {
			log.Printf("query KOOK channel sort notification recipients channel=%s: %v", move.ChannelID, err)
			continue
		}
		line := fmt.Sprintf("频道「%s」已从「%s」移动到「%s」。", move.ChannelName, firstNonEmpty(names[move.FromParentID], "未分组"), firstNonEmpty(names[move.ToParentID], "未分组"))
		for _, userID := range kookChannelSortAdminUserIDs(permissions, members) {
			messages[userID] = append(messages[userID], line)
		}
	}
	var firstErr error
	for userID, lines := range messages {
		content := "KOOK 频道自动排序通知\n" + strings.Join(lines, "\n")
		if err := h.sendKookDirectMessage(ctx, userID, content); err != nil {
			log.Printf("send KOOK channel sort notification user=%s: %v", userID, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
