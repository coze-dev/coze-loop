// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newCancelTrialFixture(t *testing.T) *pendingRunFixture {
	t.Helper()
	p := newPendingRunFixture(t, entity.EvaluationModeTrialRun)
	// Run the production TrialRun initializer rather than relabeling a Submit run.
	for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result"} {
		require.NoError(t, p.f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", p.f.space, p.f.expt).Error)
	}
	require.NoError(t, p.manager.idem.Del(context.Background(), makeStartIdemKey(p.event)))
	ctrl := gomock.NewController(t)
	items := svcmocks.NewMockEvaluationSetItemService(ctrl)
	items.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: p.f.item, ItemVersionID: &p.f.version, Turns: []*entity.Turn{{ID: 0}}}}, gptr.Of(int64(1)), nil, nil, nil)
	p.start.evaluationSetItemService = items
	expt := &entity.Experiment{ID: p.f.expt, SpaceID: p.f.space, LatestRunID: p.f.run, ExptType: entity.ExptType_Offline, TrialRunItemCount: 1, EvalSet: &entity.EvaluationSet{ID: p.f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: p.f.base + 81}}}
	p.start.resultSvc = &cancelTrialStartResult{}
	require.NoError(t, (&ExptTrialRunExec{ExptSubmitExec: p.start}).ExptStart(context.Background(), p.event, expt))
	require.NoError(t, p.f.turnRepo().BatchCreateNXRunLog(context.Background(), []*entity.ExptTurnResultRunLog{{ID: p.f.base + 9910, SpaceID: p.f.space, ExptID: p.f.expt, ExptRunID: p.f.run, ItemID: p.f.item, ItemVersionID: p.f.version, TurnID: 0, Status: entity.TurnRunState_Queueing}}))
	return p
}

type cancelTrialStartResult struct{ ExptResultService }

func (*cancelTrialStartResult) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

func TestTrialFailureRealSchedulerExplicitFailed(t *testing.T) {
	for _, state := range []string{"pending", "terminal_success"} {
		t.Run(state, func(t *testing.T) {
			p := newCancelTrialFixture(t)
			if state == "terminal_success" {
				p.publishSuccess(t, false)
			}
			scheduler := &ExptSchedulerImpl{Manager: p.manager}
			err := scheduler.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error { return errors.New("trial business failure") })(context.Background(), p.event)
			require.NoError(t, err, "explicit failure must not wait for natural pending publication")
			run, err := p.manager.runLogRepo.Get(context.Background(), p.f.expt, p.f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Failed), run.Status)
			expt, err := p.manager.exptRepo.GetByID(context.Background(), p.f.expt, p.f.space)
			require.NoError(t, err)
			require.Equal(t, entity.ExptStatus_Failed, expt.Status)
			before := p.f.snapshot()
			require.NoError(t, scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
				t.Fatal("finished trial reentered execution")
				return nil
			})(context.Background(), p.event))
			require.Equal(t, before, p.f.snapshot())
		})
	}
}

func TestTrialFailureOldRunHasZeroActions(t *testing.T) {
	p := newCancelTrialFixture(t)
	next := p.f.run + 100
	require.NoError(t, p.f.conn.Create(&model.ExptRunLog{ID: next, SpaceID: p.f.space, ExptID: p.f.expt, ExptRunID: next, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeTrialRun))}).Error)
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=?", p.f.space).Update("latest_run_id", next).Error)
	before := p.f.snapshot()
	err := (&ExptSchedulerImpl{Manager: p.manager}).HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error { return errors.New("old trial failure") })(context.Background(), p.event)
	require.NoError(t, err)
	require.Equal(t, before, p.f.snapshot())
	require.Zero(t, p.resources.Load())
	require.Zero(t, p.notifications.Load())
}
