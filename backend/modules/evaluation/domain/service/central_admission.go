// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// CentralAdmission is the optional seam between the existing per-experiment
// scheduler and the central scheduler. A nil implementation (the default) keeps
// the historical MQ self-loop and legacy 200-running-experiment admission.
//
// Implementations are attached through ExptSchedulerImpl.SetCentralAdmission.
type CentralAdmission interface {
	// Mode returns the effective mode for this event. ModeLegacy means "do
	// nothing": the scheduler behaves exactly as before.
	Mode(ctx context.Context, event *entity.ExptScheduleEvent) central.Mode
	// AcquireItems atomically reserves multi-resource holds for the items about
	// to be dispatched and returns the subset that may be published.
	AcquireItems(ctx context.Context, event *entity.ExptScheduleEvent, itemIDs []int64) ([]int64, error)
	// ReleaseItems releases holds for items that reached a terminal state.
	ReleaseItems(ctx context.Context, event *entity.ExptScheduleEvent, itemIDs []int64)
	// ReleaseRun releases all holds for a completed/cancelled/retried run.
	ReleaseRun(ctx context.Context, event *entity.ExptScheduleEvent)
	// Heartbeat renews registrations/leases and replaces the self-loop in enforce mode.
	Heartbeat(ctx context.Context, event *entity.ExptScheduleEvent) error
}

// CentralGate adapts central.Admission to an ExptScheduleEvent.
type CentralGate struct {
	admission *central.Admission
	configer  component.IConfiger
}

var _ CentralAdmission = (*CentralGate)(nil)

func NewCentralGate(admission *central.Admission, configer component.IConfiger) CentralAdmission {
	return &CentralGate{admission: admission, configer: configer}
}

func (g *CentralGate) Mode(ctx context.Context, event *entity.ExptScheduleEvent) central.Mode {
	if g == nil || g.admission == nil || event == nil {
		return central.ModeLegacy
	}
	conf := g.configer.GetExptExecConf(ctx, event.SpaceID)
	mode := central.ParseMode(conf.GetCentralSchedulingMode())
	if mode == central.ModeEnforce && !conf.CentralEnforced(event.SpaceID, event.ExptID) {
		// Not in the allow-list: keep legacy exactly intact.
		return central.ModeLegacy
	}
	return mode
}

func (g *CentralGate) runFromEvent(ctx context.Context, event *entity.ExptScheduleEvent) central.ActiveRun {
	conf := g.configer.GetExptExecConf(ctx, event.SpaceID)
	maxConc := conf.GetExptItemEvalConf().GetConcurNum()
	if maxConc <= 0 {
		maxConc = 1
	}
	enqueued := time.Now()
	if event.CreatedAt > 0 {
		enqueued = time.Unix(event.CreatedAt, 0)
	}
	run := central.ActiveRun{
		Scope: g.admission.ScopeOf(ctx, event.SpaceID),
		Key: central.RunKey{
			SpaceID:   event.SpaceID,
			ExptID:    event.ExptID,
			ExptRunID: event.ExptRunID,
		},
		RunMode:        int32(event.ExptRunMode),
		ExptType:       int32(event.ExptType),
		EnqueuedAt:     enqueued,
		UpdatedAt:      enqueued,
		MaxConcurrency: int64(maxConc),
		RemainingItems: int64(maxConc),
		PerItem: map[central.ResourceKind]int64{
			central.ResourceItemSlot:    1,
			central.ResourceTargetSlot:  1,
			central.ResourceSandboxSlot: 1,
		},
	}
	if event.Session != nil {
		run.CreatedBy = event.Session.UserID
	}
	return run
}

func (g *CentralGate) AcquireItems(ctx context.Context, event *entity.ExptScheduleEvent, itemIDs []int64) ([]int64, error) {
	if g.Mode(ctx, event) == central.ModeLegacy {
		return nil, nil
	}
	run := g.runFromEvent(ctx, event)
	run.Budget = central.Plan(run, g.admission.HeldItems(ctx, run))
	return g.admission.Acquire(ctx, run, itemIDs)
}

func (g *CentralGate) ReleaseItems(ctx context.Context, event *entity.ExptScheduleEvent, itemIDs []int64) {
	if g.Mode(ctx, event) == central.ModeLegacy {
		return
	}
	g.admission.ReleaseItems(ctx, g.runFromEvent(ctx, event), itemIDs)
}

func (g *CentralGate) ReleaseRun(ctx context.Context, event *entity.ExptScheduleEvent) {
	if g.Mode(ctx, event) == central.ModeLegacy {
		return
	}
	g.admission.ReleaseRun(ctx, g.runFromEvent(ctx, event))
}

func (g *CentralGate) Heartbeat(ctx context.Context, event *entity.ExptScheduleEvent) error {
	if g.Mode(ctx, event) != central.ModeEnforce {
		return nil
	}
	return g.admission.Heartbeat(ctx, g.runFromEvent(ctx, event))
}

// CentralCapacity converts the entity-level config map into the central resource
// model.
func CentralCapacity(m map[string]int64) map[central.ResourceKind]int64 {
	out := make(map[central.ResourceKind]int64, len(m))
	for k, v := range m {
		out[central.ResourceKind(k)] = v
	}
	return out
}
