// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// CentralItemSettlementDeps 是消费者侧结算所需的最小依赖，避免改动原有 service 构造函数。
type CentralItemSettlementDeps struct {
	Ledger   repo.ICentralLedgerRepo
	Env      component.IEnvironmentProvider
	ItemRepo repo.ICentralExperimentRepo
}

// NewCentralItemEvalService 包装 item eval：调用原链路后按 DB 投影结算预占。
// DB 结果不明或仍 Processing 时保持待补偿，绝不猜测成功/失败。
func NewCentralItemEvalService(inner ExptItemEvalEvent, deps ...CentralItemSettlementDeps) ExptItemEvalEvent {
	if inner == nil || len(deps) == 0 || deps[0].Ledger == nil || deps[0].Env == nil || deps[0].ItemRepo == nil {
		return inner
	}
	return &centralItemEvalService{inner: inner, deps: deps[0]}
}

type centralItemEvalService struct {
	inner ExptItemEvalEvent
	deps  CentralItemSettlementDeps
}

func (s *centralItemEvalService) Eval(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	err := s.inner.Eval(ctx, event)
	if event == nil || event.CentralReservationID == "" {
		return err
	}
	// 处理返回错误说明下游结果不明（或仍在重试），留给对账判定，不能确认也不能释放。
	if err != nil {
		return err
	}
	current, eerr := s.deps.Env.CurrentEnvID(ctx)
	if eerr != nil || event.CentralEnv == "" || event.CentralEnv != current {
		logs.CtxWarn(ctx, "[CentralSettle] skip reservation %s: env event=%q current=%q err=%v",
			event.CentralReservationID, event.CentralEnv, current, eerr)
		return nil
	}
	states, ierr := s.deps.ItemRepo.GetItemStates(ctx, event.ExptID, event.ExptRunID, event.SpaceID, []int64{event.EvalSetItemID})
	if ierr != nil {
		logs.CtxWarn(ctx, "[CentralSettle] read item state fail, keep pending, resv=%s err=%v", event.CentralReservationID, ierr)
		return nil
	}
	state := states[event.EvalSetItemID]
	switch {
	case entity.IsItemRunFinished(state) && state == entity.ItemRunState_Success:
		if _, cerr := s.deps.Ledger.Confirm(ctx, current, event.CentralReservationID); cerr != nil {
			logs.CtxWarn(ctx, "[CentralSettle] confirm fail, resv=%s err=%v", event.CentralReservationID, cerr)
			return nil
		}
		if _, rerr := s.deps.Ledger.Release(ctx, current, event.CentralReservationID, "item_success"); rerr != nil {
			logs.CtxWarn(ctx, "[CentralSettle] release after success fail, resv=%s err=%v", event.CentralReservationID, rerr)
		}
	case entity.IsItemRunFinished(state):
		if _, rerr := s.deps.Ledger.Release(ctx, current, event.CentralReservationID, "item_terminal_fail"); rerr != nil {
			logs.CtxWarn(ctx, "[CentralSettle] release on fail fail, resv=%s err=%v", event.CentralReservationID, rerr)
		}
	default:
		// 仍在处理（含异步 abort）→ 待补偿，不重复确认。
		if _, perr := s.deps.Ledger.MarkPendingCompensation(ctx, current, event.CentralReservationID); perr != nil {
			logs.CtxWarn(ctx, "[CentralSettle] mark pending fail, resv=%s err=%v", event.CentralReservationID, perr)
		}
	}
	return nil
}
