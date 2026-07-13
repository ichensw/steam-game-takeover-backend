package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeKookChannelGateway struct {
	channels []kookChannelDTO
	updates  []kookChannelPosition
	update   func(kookChannelPosition, int) error
	attempts map[string]int
}

func (f *fakeKookChannelGateway) ListChannels(context.Context) ([]kookChannelDTO, error) {
	return append([]kookChannelDTO(nil), f.channels...), nil
}

func (f *fakeKookChannelGateway) UpdateChannel(_ context.Context, position kookChannelPosition) error {
	if f.attempts == nil {
		f.attempts = map[string]int{}
	}
	f.updates = append(f.updates, position)
	f.attempts[position.ChannelID]++
	if f.update != nil {
		return f.update(position, f.attempts[position.ChannelID])
	}
	return nil
}

func TestPreviewKookChannelSortMakesNoUpdates(t *testing.T) {
	gateway := &fakeKookChannelGateway{channels: []kookChannelDTO{
		{ID: "group-1", Name: "Group 1", Type: 0, KookSort: 100},
		{ID: "group-2", Name: "Group 2", Type: 0, KookSort: 200},
		{ID: "voice-1", Name: "Voice 1", Type: 2, ParentID: "group-1", KookSort: 100},
		{ID: "voice-2", Name: "Voice 2", Type: 2, ParentID: "group-2", KookSort: 100},
	}}
	usage := map[string]kookChannelSortUsage{
		"voice-1": {OccupiedSeconds: 10},
		"voice-2": {OccupiedSeconds: 20},
	}

	plan, err := previewKookChannelSort(context.Background(), gateway, []string{"group-1", "group-2"}, usage)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(plan.Groups) != 2 || plan.Groups[0].Channels[0].ChannelID != "voice-2" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if len(gateway.updates) != 0 {
		t.Fatalf("preview sent updates: %#v", gateway.updates)
	}
}

func TestApplyKookChannelSortPlanSendsOnlyChangedMoves(t *testing.T) {
	gateway := &fakeKookChannelGateway{}
	plan := kookChannelSortPlan{Moves: []kookChannelMove{
		{ChannelID: "unchanged", FromParentID: "group-1", ToParentID: "group-1", FromLevel: 100, ToLevel: 100},
		{ChannelID: "moved", FromParentID: "group-1", ToParentID: "group-2", FromLevel: 100, ToLevel: 200},
	}}

	result := applyKookChannelSortPlan(context.Background(), gateway, plan, noKookChannelSortWait, nil)
	if result.Err != nil || result.MovedCount != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(gateway.updates) != 1 || gateway.updates[0].ChannelID != "moved" {
		t.Fatalf("updates = %#v", gateway.updates)
	}
}

func TestApplyKookChannelSortPlanRetriesTransientFailureTwice(t *testing.T) {
	gateway := &fakeKookChannelGateway{update: func(_ kookChannelPosition, attempt int) error {
		if attempt < 3 {
			return errors.New("network unavailable")
		}
		return nil
	}}
	plan := kookChannelSortPlan{Moves: []kookChannelMove{{ChannelID: "voice-1", ToParentID: "group-1", ToLevel: 100}}}

	result := applyKookChannelSortPlan(context.Background(), gateway, plan, noKookChannelSortWait, nil)
	if result.Err != nil || gateway.attempts["voice-1"] != 3 {
		t.Fatalf("result = %#v, attempts = %#v", result, gateway.attempts)
	}
}

func TestApplyKookChannelSortPlanDoesNotRetryBusinessFailure(t *testing.T) {
	gateway := &fakeKookChannelGateway{update: func(_ kookChannelPosition, _ int) error {
		return &kookChannelGatewayError{StatusCode: 400, Code: 40000, Message: "invalid level"}
	}}
	plan := kookChannelSortPlan{Moves: []kookChannelMove{{ChannelID: "voice-1", ToParentID: "group-1", ToLevel: 100}}}

	result := applyKookChannelSortPlan(context.Background(), gateway, plan, noKookChannelSortWait, nil)
	if result.Err == nil || gateway.attempts["voice-1"] != 1 {
		t.Fatalf("result = %#v, attempts = %#v", result, gateway.attempts)
	}
}

func TestApplyKookChannelSortPlanRollsBackInReverseOrder(t *testing.T) {
	gateway := &fakeKookChannelGateway{update: func(position kookChannelPosition, _ int) error {
		if position.ChannelID == "voice-3" {
			return &kookChannelGatewayError{StatusCode: 400, Code: 40000, Message: "invalid level"}
		}
		return nil
	}}
	plan := kookChannelSortPlan{Moves: []kookChannelMove{
		{ChannelID: "voice-1", FromParentID: "old-1", ToParentID: "new", FromLevel: 100, ToLevel: 100},
		{ChannelID: "voice-2", FromParentID: "old-2", ToParentID: "new", FromLevel: 200, ToLevel: 200},
		{ChannelID: "voice-3", FromParentID: "old-3", ToParentID: "new", FromLevel: 300, ToLevel: 300},
	}}

	result := applyKookChannelSortPlan(context.Background(), gateway, plan, noKookChannelSortWait, nil)
	if result.Err == nil || result.MovedCount != 2 || len(result.RollbackFailedChannelIDs) != 0 {
		t.Fatalf("result = %#v", result)
	}
	want := []string{"voice-1:new", "voice-2:new", "voice-3:new", "voice-2:old-2", "voice-1:old-1"}
	if len(gateway.updates) != len(want) {
		t.Fatalf("updates = %#v", gateway.updates)
	}
	for index, update := range gateway.updates {
		got := update.ChannelID + ":" + update.ParentID
		if got != want[index] {
			t.Fatalf("update %d = %q, want %q", index, got, want[index])
		}
	}
}

func noKookChannelSortWait(context.Context, time.Duration) error { return nil }
