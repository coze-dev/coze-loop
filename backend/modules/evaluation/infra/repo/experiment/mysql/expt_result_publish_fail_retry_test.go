// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type crossRunItemStorage struct{ *singleSetItemStorage }

func (s *crossRunItemStorage) MGetItemRunLog(ctx context.Context, expt, run int64, ids []int64, space int64) ([]*entity.ExptItemResultRunLog, error) {
	rows, err := mysql.NewExptItemResultDAO(s.f.provider).MGetItemRunLog(ctx, expt, run, ids, space)
	var result []*entity.ExptItemResultRunLog
	for _, row := range rows {
		result = append(result, convert.NewExptItemResultRunLogConverter().PO2DO(row))
	}
	return result, err
}

type crossRunTurnStorage struct{ *singleSetTurnStorage }

func (s *crossRunTurnStorage) MGetItemTurnRunLogs(ctx context.Context, expt, run int64, ids []int64, space int64) ([]*entity.ExptTurnResultRunLog, error) {
	rows, err := mysql.NewExptTurnResultDAO(s.f.provider).MGetItemTurnRunLogs(ctx, expt, run, ids, space)
	var result []*entity.ExptTurnResultRunLog
	for _, row := range rows {
		log, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(row)
		if err != nil {
			return nil, err
		}
		result = append(result, log)
	}
	return result, err
}

func crossRunRetryManager(t *testing.T, f *manualRetryFixture) service.IExptManager {
	t.Helper()
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)}).AnyTimes()
	locker := lockmocks.NewMockILocker(ctrl)
	locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), 5*time.Second, 5*time.Minute).DoAndReturn(func(ctx context.Context, key string, ttl, renewal time.Duration) (bool, context.Context, func(), error) {
		child, cancel := context.WithCancel(ctx)
		return true, child, cancel, nil
	}).AnyTimes()
	s := &singleSetRetryStorage{f: f}
	items := &crossRunItemStorage{&singleSetItemStorage{singleSetRetryStorage: s}}
	turns := &crossRunTurnStorage{&singleSetTurnStorage{singleSetRetryStorage: s}}
	nextID := f.base + 20000
	generator := idgenmocks.NewMockIIDGenerator(ctrl)
	generator.EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) {
		nextID++
		return nextID, nil
	}).AnyTimes()
	port := mysql.NewExptManualRetryDAO(f.provider, generator)
	return service.NewExptManager(
		nil, &singleSetExperimentStorage{singleSetRetryStorage: s}, &singleSetRunStorage{singleSetRetryStorage: s}, nil,
		items, &singleSetRefStorage{singleSetRetryStorage: s}, turns, config, nil, locker, nil, nil,
		nil, nil, nil, nil, nil, nil, &singleSetTargetStorage{singleSetRetryStorage: s}, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, componentmocks.NewMockICentralReservationGuard(ctrl), port, &singleSetEvaluatorStorage{singleSetRetryStorage: s}, singleSetRetryWaker{},
		func() lock.ILocker { return locker },
	)
}

func TestResultPublishMySQLFailRetryHistoricalSuccessAllowsNextRetry(t *testing.T) {
	f := newManualRetryFixture(t)
	prepareSingleSetFixture(t, f, false, false)
	ctx := context.Background()
	r1, r2, r3 := f.run, f.run+1000, f.run+2000
	var item model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Take(&item).Error)
	require.Equal(t, int32(entity.ItemRunState_Fail), item.Status)
	var r1Turns []model.ExptTurnResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Order("id").Find(&r1Turns).Error)
	require.Len(t, r1Turns, 2)
	for index, status := range []entity.TurnRunState{entity.TurnRunState_Success, entity.TurnRunState_Fail} {
		r1Turns[index].Status = int32(status)
		require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("id=?", r1Turns[index].ID).Update("status", status).Error)
		require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND turn_id=?", f.space, f.expt, r1, 11, r1Turns[index].TurnID).Update("status", status).Error)
	}

	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: r2, SpaceID: f.space, ExptID: f.expt, ExptRunID: r2, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeFailRetry)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[11,12,13],"CreateAt":1700000000}]`))}).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).Update("latest_run_id", r2).Error)
	manual, err := json.Marshal(&entity.ManualRetryContext{RunID: r2, RequestID: "r2", Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1})
	require.NoError(t, err)
	ext, err := json.Marshal(map[string]string{entity.ManualRetryExtKey: string(manual)})
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("id=?", item.ID).Updates(map[string]any{"expt_run_id": r2, "ext": ext}).Error)
	var r1ItemLog model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, r1, 11).Take(&r1ItemLog).Error)
	r2ItemLog := r1ItemLog
	r2ItemLog.ID = r2 + 300
	r2ItemLog.ExptRunID = r2
	r2ItemLog.ResultState = gptr.Of(int32(entity.ExptItemResultStateLogged))
	require.NoError(t, f.conn.Create(&r2ItemLog).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("id=?", r1Turns[1].ID).Update("expt_run_id", r2).Error)
	var r1Logs []model.ExptTurnResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, r1, 11).Order("id").Find(&r1Logs).Error)
	require.Len(t, r1Logs, 2)
	for _, old := range r1Logs {
		log := old
		log.ID = r2 + old.ID - r1
		log.ExptRunID = r2
		require.NoError(t, f.conn.Create(&log).Error)
	}

	port := mysql.NewExptResultPublishDAO(f.provider)
	source, err := port.GetItemResultPublishSource(ctx, f.space, f.expt, r2, 11)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, source.Source.CanonicalTurns, 2)
	require.Equal(t, r1, source.Source.CanonicalTurns[0].ExptRunID)
	publication := &entity.ExptItemResultPublication{Source: source, Refs: source.Source.CurrentRefs}
	for _, old := range source.Source.CanonicalTurns {
		turn := *old
		log := source.Source.Turns[old.TurnID]
		turn.Status = int32(log.Status)
		turn.TargetResultID = log.TargetResultID
		turn.TraceID = log.TraceID
		turn.LogID = log.LogID
		turn.ErrMsg = log.ErrMsg
		turn.ExptRunID = r2
		turn.WeightedScore = nil
		publication.Turns = append(publication.Turns, &turn)
	}
	applied, err := port.CommitItemResult(ctx, publication)
	require.NoError(t, err)
	require.True(t, applied)
	var other model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 12).Take(&other).Error)
	require.Equal(t, int32(entity.ItemRunState_Processing), other.Status)
	var experiment model.Experiment
	require.NoError(t, f.conn.Where("space_id=? AND id=?", f.space, f.expt).Take(&experiment).Error)
	require.Equal(t, int32(entity.ExptStatus_Processing), experiment.Status)

	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: r3, SpaceID: f.space, ExptID: f.expt, ExptRunID: r3, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeFailRetry)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[11,12,13],"CreateAt":1700000000}]`))}).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).Update("latest_run_id", r3).Error)
	receipt, err := crossRunRetryManager(t, f).RetryInCurrentRun(ctx, &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: r3, Mode: entity.EvaluationModeFailRetry})
	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Equal(t, int32(1), receipt.AcceptedCount)
}
