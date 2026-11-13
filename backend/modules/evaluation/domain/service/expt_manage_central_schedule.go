// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central_schedule"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ExptManagerOption 可选装配。中心调度未装配时，legacy 行为完全不变。
type ExptManagerOption func(*ExptMangerImpl)

// WithCentralSchedule 装配 run 级调度投影与账本入口。
func WithCentralSchedule(scheduleRepo repo.IRunScheduleRepo, scheduler *central_schedule.Scheduler) ExptManagerOption {
	return func(e *ExptMangerImpl) {
		e.scheduleRepo = scheduleRepo
		e.centralScheduler = scheduler
		if scheduler == nil {
			return
		}
		// DB 已无待处理 item 时按旧链路的同一组幂等 key 结束 run/实验。
		scheduler.RunDone = func(ctx context.Context, run *central_schedule.RunView) error {
			cid := fmt.Sprintf("exptexec:onend:%d", run.RunID)
			runID := run.RunID
			if err := e.CompleteRun(ctx, run.ExptID, runID, run.SpaceID, &entity.Session{}, entity.WithCID(cid), entity.WithCompleteInterval(2*time.Second)); err != nil {
				return err
			}
			if err := e.CompleteExpt(ctx, run.ExptID, &runID, run.SpaceID, &entity.Session{}, entity.WithCID(cid), entity.WithCompleteInterval(2*time.Second)); err != nil {
				return err
			}
			e.finishRunSchedule(ctx, run.ExptID, runID, run.SpaceID)
			return nil
		}
	}
}

// runScheduleMode 读取实验固定的调度模式。重试创建的新 run 只复制该值，
// 请求参数不能覆盖。
func runScheduleMode(expt *entity.Experiment) central_schedule.Mode {
	if expt == nil || expt.EvalConf == nil {
		return central_schedule.ModeLegacy
	}
	switch central_schedule.Mode(expt.EvalConf.ScheduleMode) {
	case central_schedule.ModeCentral:
		return central_schedule.ModeCentral
	default:
		return central_schedule.ModeLegacy
	}
}

func (e *ExptMangerImpl) runOwnerScope(ctx context.Context, spaceID int64) string {
	if e.configer == nil {
		return ""
	}
	conf := e.configer.GetExptExecConf(ctx, spaceID).GetCentralScheduleConf()
	if conf == nil {
		return ""
	}
	return conf.OwnerScope()
}

func (e *ExptMangerImpl) runTargetConcurrency(ctx context.Context, expt *entity.Experiment, spaceID int64) int64 {
	if expt != nil && expt.EvalConf != nil {
		if v := gptr.Indirect(expt.EvalConf.ItemConcurNum); v > 0 {
			return int64(v)
		}
	}
	if e.configer != nil {
		if conf := e.configer.GetExptExecConf(ctx, spaceID).GetCentralScheduleConf(); conf != nil && conf.DefaultTargetConcur > 0 {
			return conf.DefaultTargetConcur
		}
	}
	return int64(entity.DefaultSubmitItemConcurNum)
}

func (e *ExptMangerImpl) dimensionLimits(target int64) map[string]int64 {
	// item 维度是硬约束；evaluator/sandbox 维度按需追加，账本不感知具体数量。
	return map[string]int64{central_schedule.ItemDimension: target}
}

// initRunSchedule 在 run 进入 Pending 时落库并初始化账本。
// 返回 true 表示该 run 走 central 模式：调用方不得再发旧 MQ 自循环事件。
func (e *ExptMangerImpl) initRunSchedule(ctx context.Context, expt *entity.Experiment, runID, spaceID int64) (bool, error) {
	if e.scheduleRepo == nil {
		return false, nil
	}
	mode := runScheduleMode(expt)
	// 在线实验是常驻 daemon，本需求不改变其链路；即使配置了 central 也按 legacy。
	if expt.ExptType == entity.ExptType_Online {
		mode = central_schedule.ModeLegacy
	}
	if mode != central_schedule.ModeCentral {
		// legacy run 的投影只用于观测/对账可见性，写失败绝不能影响旧链路。
		// 新表未部署、DB 抖动、DDL 灰度差异都只能记日志，不能返回错误。
		e.persistLegacyRunSchedule(ctx, expt, runID, spaceID, mode)
		return false, nil
	}
	if !e.centralEnabled(ctx, spaceID) {
		// 配置关闭时保持旧链路，不因为新字段出现而改变存量行为。
		e.persistLegacyRunSchedule(ctx, expt, runID, spaceID, central_schedule.ModeLegacy)
		return false, nil
	}
	scope := e.runOwnerScope(ctx, spaceID)
	if scope == "" {
		// 隔离缺失 fail-closed：绝不回落到公共池。
		return false, central_schedule.ErrOwnerScopeMissing
	}
	if err := e.persistRunSchedule(ctx, expt, runID, spaceID, mode); err != nil {
		return false, err
	}
	if e.centralScheduler == nil {
		return true, nil
	}
	target := e.runTargetConcurrency(ctx, expt, spaceID)
	if err := e.centralScheduler.InitRun(ctx, &central_schedule.RunAccount{
		OwnerScope:        scope,
		ExptID:            expt.ID,
		RunID:             runID,
		Mode:              mode,
		Priority:          expt.EvalConf.Priority,
		TargetConcurrency: target,
		DimensionLimits:   e.dimensionLimits(target),
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (e *ExptMangerImpl) centralEnabled(ctx context.Context, spaceID int64) bool {
	if e.configer == nil {
		return false
	}
	conf := e.configer.GetExptExecConf(ctx, spaceID).GetCentralScheduleConf()
	return conf != nil && conf.Enabled
}

// persistLegacyRunSchedule 旧链路投影 best-effort：任何失败只记日志，不影响旧实验派发。
func (e *ExptMangerImpl) persistLegacyRunSchedule(ctx context.Context, expt *entity.Experiment, runID, spaceID int64, mode central_schedule.Mode) {
	if e.scheduleRepo == nil || expt == nil {
		return
	}
	if err := e.persistRunSchedule(ctx, expt, runID, spaceID, mode); err != nil {
		logs.CtxWarn(ctx, "[CentralSchedule] legacy projection skipped, expt_id=%d run_id=%d err=%v", expt.ID, runID, err)
	}
}

func (e *ExptMangerImpl) persistRunSchedule(ctx context.Context, expt *entity.Experiment, runID, spaceID int64, mode central_schedule.Mode) error {
	if expt == nil {
		return nil
	}
	scope := e.runOwnerScope(ctx, spaceID)
	var priority int32
	if expt != nil && expt.EvalConf != nil {
		priority = expt.EvalConf.Priority
	}
	row := &entity.ExptRunSchedule{
		SpaceID:           spaceID,
		ExptID:            expt.ID,
		RunID:             runID,
		ScheduleMode:      int32(mode),
		Priority:          priority,
		OwnerScope:        scope,
		TargetConcurrency: e.runTargetConcurrency(ctx, expt, spaceID),
		Status:            entity.ExptRunScheduleStatusPending,
	}
	if err := e.scheduleRepo.Upsert(ctx, row); err != nil {
		logs.CtxError(ctx, "[CentralSchedule] persist run schedule fail, expt_id=%d run_id=%d err=%v", expt.ID, runID, err)
		return err
	}
	return nil
}

// CentralScheduleAckGate 由执行侧消费入口调用；scheduler 自己绝不填确认。
type CentralScheduleAckGate interface {
	AckCentralScheduleItem(ctx context.Context, event *entity.ExptItemEvalEvent) (bool, error)
}

// CentralSchedulerProvider 让进程启动侧拿到调度器并启动独立 tick。
type CentralSchedulerProvider interface {
	CentralScheduler() *central_schedule.Scheduler
}

func (e *ExptMangerImpl) CentralScheduler() *central_schedule.Scheduler {
	return e.centralScheduler
}

// AckCentralScheduleItem 执行侧确认：只有首次 CAS 成功才允许执行。
// 重复/迟到/账本缺失一律返回 false，调用方跳过执行，不重复扣额度。
func (e *ExptMangerImpl) AckCentralScheduleItem(ctx context.Context, event *entity.ExptItemEvalEvent) (bool, error) {
	if e.centralScheduler == nil || event == nil || event.ScheduleReservationID == "" {
		// legacy 事件或未装配中心调度：没有派发凭证，直接放行。
		return true, nil
	}
	ok, err := e.centralScheduler.AckItem(ctx, event.ScheduleReservationID, &central_schedule.ItemView{
		SpaceID:   event.SpaceID,
		ExptID:    event.ExptID,
		RunID:     event.ExptRunID,
		ItemID:    event.EvalSetItemID,
		Attempt:   event.ScheduleAttempt,
		Status:    central_schedule.ItemProcessing,
	})
	if err != nil {
		return false, err
	}
	return ok, nil
}

// ReleaseCentralScheduleItem 执行侧 item 落终态后归还该条预占。
func (e *ExptMangerImpl) ReleaseCentralScheduleItem(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	if e.centralScheduler == nil || event == nil || event.ScheduleReservationID == "" {
		return nil
	}
	return e.centralScheduler.ReleaseItem(ctx, event.ExptRunID, event.EvalSetItemID, event.ScheduleAttempt)
}

// GetRunSchedule 供内部运维入口回读 run 级调度元数据。
func (e *ExptMangerImpl) GetRunSchedule(ctx context.Context, spaceID, exptID, runID int64) (*entity.ExptRunSchedule, error) {
	if e.scheduleRepo == nil {
		return nil, central_schedule.ErrNotInitialized
	}
	return e.scheduleRepo.Get(ctx, spaceID, exptID, runID)
}

// CentralRunView 从持久化投影构造对账单 run 视图。
func (e *ExptMangerImpl) CentralRunView(ctx context.Context, row *entity.ExptRunSchedule) *central_schedule.RunView {
	if row == nil {
		return nil
	}
	return &central_schedule.RunView{
		OwnerScope:        row.OwnerScope,
		SpaceID:           row.SpaceID,
		ExptID:            row.ExptID,
		RunID:             row.RunID,
		Mode:              central_schedule.Mode(row.ScheduleMode),
		Priority:          row.Priority,
		TargetConcurrency: row.TargetConcurrency,
		DimensionLimits:   e.dimensionLimits(row.TargetConcurrency),
		CreatedAt:         row.CreatedAt,
	}
}

// ReconcileCentralRun 运维入口：增量对账或单 run 全量重建。
func (e *ExptMangerImpl) ReconcileCentralRun(ctx context.Context, ownerScope string, spaceID, exptID, runID int64, fullRebuild bool) (central_schedule.ReconcileStats, error) {
	if e.centralScheduler == nil {
		return central_schedule.ReconcileStats{}, central_schedule.ErrNotInitialized
	}
	row, err := e.GetRunSchedule(ctx, spaceID, exptID, runID)
	if err != nil {
		return central_schedule.ReconcileStats{}, err
	}
	if ownerScope != "" && row.OwnerScope != ownerScope {
		return central_schedule.ReconcileStats{}, central_schedule.ErrOwnerScopeMissing
	}
	run := e.CentralRunView(ctx, row)
	rec := *e.centralScheduler.Reconciler
	if fullRebuild {
		return rec.RebuildRun(ctx, run)
	}
	return rec.Incremental(ctx, run, 0)
}

// RebuildCentralScope 运维入口：scope 级全量重建。
func (e *ExptMangerImpl) RebuildCentralScope(ctx context.Context, ownerScope string) (central_schedule.ReconcileStats, error) {
	if e.centralScheduler == nil || e.centralScheduler.Reconciler == nil {
		return central_schedule.ReconcileStats{}, central_schedule.ErrNotInitialized
	}
	if ownerScope == "" {
		return central_schedule.ReconcileStats{}, central_schedule.ErrOwnerScopeMissing
	}
	rec := *e.centralScheduler.Reconciler
	return rec.FullRebuild(ctx, ownerScope)
}

// finishRunSchedule 结束路径统一投影终态并归还额度。重复调用幂等。
func (e *ExptMangerImpl) finishRunSchedule(ctx context.Context, exptID, runID, spaceID int64) {
	if e.scheduleRepo != nil {
		if err := e.scheduleRepo.UpdateStatus(ctx, spaceID, exptID, runID, entity.ExptRunScheduleStatusTerminal, 0); err != nil {
			logs.CtxWarn(ctx, "[CentralSchedule] mark run terminal fail, expt_id=%d run_id=%d err=%v", exptID, runID, err)
		}
	}
	if e.centralScheduler != nil {
		if _, err := e.centralScheduler.ReleaseRun(ctx, runID); err != nil {
			logs.CtxWarn(ctx, "[CentralSchedule] release run reservations fail, run_id=%d err=%v", runID, err)
		}
	}
}
