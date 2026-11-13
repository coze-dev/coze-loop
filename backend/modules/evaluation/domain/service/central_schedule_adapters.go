// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central_schedule"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// CentralScheduleAdapter 把现有 repo/publisher 适配成中心调度端口。
//
// DB 是 item 生命周期权威（Queueing/Processing/Success/Fail/Terminal），
// 账本是额度权威。适配层不做任何超卖/去重判断，只如实读写。
type CentralScheduleAdapter struct {
	ScheduleRepo repo.IRunScheduleRepo
	ItemRepo     repo.IExptItemResultRepo
	ExptRepo     repo.IExperimentRepo
	QuotaRepo    repo.QuotaRepo
	Configer     component.IConfiger
	Publisher    events.ExptEventPublisher
}

func NewCentralScheduleAdapter(
	scheduleRepo repo.IRunScheduleRepo,
	itemRepo repo.IExptItemResultRepo,
	exptRepo repo.IExperimentRepo,
	quotaRepo repo.QuotaRepo,
	configer component.IConfiger,
	publisher events.ExptEventPublisher,
) *CentralScheduleAdapter {
	return &CentralScheduleAdapter{
		ScheduleRepo: scheduleRepo,
		ItemRepo:     itemRepo,
		ExptRepo:     exptRepo,
		QuotaRepo:    quotaRepo,
		Configer:     configer,
		Publisher:    publisher,
	}
}

// ScanActive 全局扫描：repo 保证按 owner_scope + 活跃状态 + priority/created_at
// 返回；这里不做“只返回 legacy”的过滤，central/legacy 都返回，由 Scheduler 判定。
func (a *CentralScheduleAdapter) ScanActive(ctx context.Context, ownerScope string, limit int) ([]*central_schedule.RunView, error) {
	rows, err := a.ScheduleRepo.ScanActive(ctx, ownerScope, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*central_schedule.RunView, 0, len(rows))
	for _, r := range rows {
		out = append(out, &central_schedule.RunView{
			OwnerScope:        r.OwnerScope,
			SpaceID:           r.SpaceID,
			ExptID:            r.ExptID,
			RunID:             r.RunID,
			Mode:              central_schedule.Mode(r.ScheduleMode),
			Priority:          r.Priority,
			TargetConcurrency: r.TargetConcurrency,
			DimensionLimits:   a.dimensionLimits(ctx, r),
			CreatedAt:         r.CreatedAt,
		})
	}
	return out, nil
}

// dimensionLimits 额度维度来源：item 取 run 目标并发；evaluator/sandbox 维度
// 由实验配置实际引用的评估器/沙箱租户展开，额度上限优先取 TCC 配置，缺省
// 视为不限制（只受 item 并发约束）。维度 key 可继续追加，不需要改账本脚本。
func (a *CentralScheduleAdapter) dimensionLimits(ctx context.Context, r *entity.ExptRunSchedule) map[string]int64 {
	limits := map[string]int64{"item": r.TargetConcurrency}
	expt, err := a.ExptRepo.GetByID(ctx, r.ExptID, r.SpaceID)
	if err != nil || expt == nil {
		logs.CtxWarn(ctx, "[CentralSchedule] load expt for dimensions fail, expt_id=%d space_id=%d err=%v", r.ExptID, r.SpaceID, err)
		return limits
	}
	for _, ev := range expt.Evaluators {
		if ev == nil || ev.ID == 0 {
			continue
		}
		key := central_schedule.EvaluatorDimension(ev.ID)
		if _, ok := limits[key]; !ok {
			limits[key] = 0 // 0 表示未配置上限，由 item 并发约束
		}
	}
	return limits
}

func (a *CentralScheduleAdapter) Check(ctx context.Context, run *central_schedule.RunView) error {
	return NewQuotaService(a.QuotaRepo, a.Configer).AllowExptRun(ctx, run.ExptID, run.SpaceID, &entity.Session{})
}

func (a *CentralScheduleAdapter) ScanQueueing(ctx context.Context, run *central_schedule.RunView, limit int) ([]*central_schedule.ItemView, error) {
	rls, _, err := a.ItemRepo.ScanItemRunLogs(ctx, run.ExptID, run.RunID,
		&entity.ExptItemRunLogFilter{Status: []entity.ItemRunState{entity.ItemRunState_Queueing}},
		0, int64(limit), run.SpaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*central_schedule.ItemView, 0, len(rls))
	for _, rl := range rls {
		out = append(out, &central_schedule.ItemView{
			SpaceID:    run.SpaceID,
			ExptID:     run.ExptID,
			RunID:      run.RunID,
			ItemID:     rl.ItemID,
			Attempt:    0,
			Status:     central_schedule.ItemQueueing,
			Dimensions: a.itemDimensions(ctx, run),
		})
	}
	return out, nil
}

func (a *CentralScheduleAdapter) GetItemStatus(ctx context.Context, run *central_schedule.RunView, itemID int64) (central_schedule.ItemStatus, error) {
	rl, err := a.ItemRepo.GetItemRunLog(ctx, run.ExptID, run.RunID, itemID, run.SpaceID)
	if err != nil {
		return central_schedule.ItemQueueing, err
	}
	if rl == nil {
		return central_schedule.ItemQueueing, nil
	}
	return central_schedule.ItemStatus(rl.Status), nil
}

func (a *CentralScheduleAdapter) ListProcessing(ctx context.Context, run *central_schedule.RunView) ([]*central_schedule.ItemView, error) {
	rls, _, err := a.ItemRepo.ScanItemRunLogs(ctx, run.ExptID, run.RunID,
		&entity.ExptItemRunLogFilter{Status: []entity.ItemRunState{entity.ItemRunState_Processing}},
		0, 10000, run.SpaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*central_schedule.ItemView, 0, len(rls))
	for _, rl := range rls {
		out = append(out, &central_schedule.ItemView{
			SpaceID: run.SpaceID, ExptID: run.ExptID, RunID: run.RunID, ItemID: rl.ItemID,
			Attempt: 0, Status: central_schedule.ItemProcessing,
			Dimensions: a.itemDimensions(ctx, run),
		})
	}
	return out, nil
}

func (a *CentralScheduleAdapter) itemDimensions(ctx context.Context, run *central_schedule.RunView) []central_schedule.Dimension {
	dims := []central_schedule.Dimension{{Key: central_schedule.ItemDimension, Amount: 1}}
	expt, err := a.ExptRepo.GetByID(ctx, run.ExptID, run.SpaceID)
	if err != nil || expt == nil {
		return dims
	}
	for _, ev := range expt.Evaluators {
		if ev == nil || ev.ID == 0 {
			continue
		}
		dims = append(dims, central_schedule.Dimension{Key: central_schedule.EvaluatorDimension(ev.ID), Amount: 1})
	}
	return central_schedule.Dimensions(dims)
}

func (a *CentralScheduleAdapter) Publish(ctx context.Context, item *central_schedule.ItemView, reservationID, dedupKey string) error {
	interval := a.Configer.GetExptExecConf(ctx, item.SpaceID).GetExptItemEvalConf().GetInterval()
	return a.Publisher.PublishExptRecordEvalEvent(ctx, &entity.ExptItemEvalEvent{
		SpaceID:               item.SpaceID,
		ExptID:                item.ExptID,
		ExptRunID:             item.RunID,
		EvalSetItemID:         item.ItemID,
		CreateAt:              time.Now().Unix(),
		ScheduleReservationID: reservationID,
		ScheduleDedupKey:      dedupKey,
		ScheduleAttempt:       item.Attempt,
	}, gptr.Of(interval), nil)
}

// MarkProcessing 把派发结果投影到 DB（DB 是生命周期权威）。
func (a *CentralScheduleAdapter) MarkProcessing(ctx context.Context, item *central_schedule.ItemView, reservationID string) error {
	if err := a.ItemRepo.UpdateItemRunLog(ctx, item.ExptID, item.RunID, []int64{item.ItemID},
		map[string]any{"status": int32(entity.ItemRunState_Processing)}, item.SpaceID); err != nil {
		return err
	}
	return a.ItemRepo.UpdateItemsResult(ctx, item.SpaceID, item.ExptID, []int64{item.ItemID},
		map[string]any{"status": int32(entity.ItemRunState_Processing)})
}

func (a *CentralScheduleAdapter) MarkReleased(ctx context.Context, runID, itemID int64, reservationID string) error {
	// 释放后的 DB 状态由执行侧/对账决定，投影层不擅自回写 Queueing。
	return nil
}
