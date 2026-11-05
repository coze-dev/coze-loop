// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

const writerReceiptJSON = `[{"ItemIDs":[11],"CreateAt":1700000000},{"ItemIDs":null,"CreateAt":1700000001,"manual_retry_receipt":{"request_id":"rq1","payload_hash":"hash","accepted_count":1,"rejected_items":[{"item_id":12,"reason":"busy"}],"snapshot_at_ms":1700000000123},"future":{"id":9223372036854775806}}]`

func newReceiptWriterManager(t *testing.T) (*ExptMangerImpl, db.Provider) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mgr := newTestExptManager(ctrl)
	provider := db.NewTestDB(t, &model.ExptRunLog{}, &model.Experiment{}, &model.ExptStats{})
	require.NotNil(t, provider)
	ctx := context.Background()
	require.NoError(t, provider.NewSession(ctx).Create(&model.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, Status: int32(entity.ExptStatus_Processing)}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptStats{ID: 10, SpaceID: 9, ExptID: 7, SuccessCnt: 2}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptRunLog{
		ID: 8, ExptID: 7, ExptRunID: 8, SpaceID: 9,
		ItemIds: gptr.Of([]byte(`[{"ItemIDs":[11],"CreateAt":1700000000}]`)), Status: gptr.Of(int64(entity.ExptStatus_Processing)),
	}).Error)
	mgr.runLogRepo = experiment.NewExptRunLogRepo(mysql.NewExptRunLogDAO(provider))
	return mgr, provider
}

func TestRecordExptDataDoesNotReopenFinishedRun(t *testing.T) {
	mgr, provider := newReceiptWriterManager(t)
	ctx := context.Background()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().ListTurnResult(ctx, int64(9), int64(7), nil, gomock.Any(), false).
		DoAndReturn(func(context.Context, int64, int64, *entity.ExptTurnResultFilter, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
			injectWriterReceipt(t, provider)
			require.NoError(t, provider.NewSession(ctx).Model(&model.ExptRunLog{}).Where("id = ?", 8).Updates(map[string]any{"status": int64(entity.ExptStatus_Terminated), "success_cnt": 2}).Error)
			return []*entity.ExptTurnResult{{Status: int32(entity.TurnRunState_Success)}}, int64(1), nil
		})
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().CalculateStats(ctx, int64(7), int64(9), gomock.Any()).Return(&entity.ExptCalculateStats{SuccessItemCnt: 1}, nil).AnyTimes()
	mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	require.NoError(t, mgr.RecordExptData(ctx, 7, 8, 9, &entity.Session{}))
	var row model.ExptRunLog
	require.NoError(t, provider.NewSession(ctx).First(&row, 8).Error)
	require.Equal(t, int64(entity.ExptStatus_Terminated), gptr.Indirect(row.Status))
	require.Equal(t, int32(2), row.SuccessCnt)
	var stats model.ExptStats
	require.NoError(t, provider.NewSession(ctx).First(&stats, 10).Error)
	require.Equal(t, int32(2), stats.SuccessCnt)
	requireWriterReceipt(t, provider)
}

func injectWriterReceipt(t *testing.T, provider db.Provider) {
	t.Helper()
	require.NoError(t, provider.NewSession(context.Background()).Model(&model.ExptRunLog{}).Where("id = ?", 8).
		UpdateColumn("item_ids", []byte(writerReceiptJSON)).Error)
}

func requireWriterReceipt(t *testing.T, provider db.Provider) {
	t.Helper()
	var row model.ExptRunLog
	require.NoError(t, provider.NewSession(context.Background()).First(&row, 8).Error)
	require.JSONEq(t, writerReceiptJSON, string(gptr.Indirect(row.ItemIds)))
	log, err := experiment.NewExptRunLogRepo(mysql.NewExptRunLogDAO(provider)).Get(context.Background(), 7, 8)
	require.NoError(t, err)
	require.Equal(t, []int64{11}, log.GetItemIDs())
	require.Equal(t, "rq1", log.ItemIds[1].ManualRetryReceipt.RequestID)
}

func TestCompleteRunKeepsReceiptForIndirectWriters(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status entity.ExptStatus
	}{
		{"scheduler_complete", entity.ExptStatus_Success},
		{"cancel", entity.ExptStatus_Terminated},
		{"scheduler_outer_error", entity.ExptStatus_Failed},
		{"consumer_credit_termination", entity.ExptStatus_SystemTerminated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, provider := newReceiptWriterManager(t)
			ctx := context.Background()
			mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().ListTurnResult(ctx, int64(9), int64(7), nil, gomock.Any(), false).
				DoAndReturn(func(context.Context, int64, int64, *entity.ExptTurnResultFilter, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
					injectWriterReceipt(t, provider)
					return []*entity.ExptTurnResult{{Status: int32(entity.TurnRunState_Success)}}, int64(1), nil
				})
			mgr.mutex.(*lockMocks.MockILocker).EXPECT().UnlockForce(ctx, mgr.makeExptMutexLockKey(7)).Return(true, nil)
			require.NoError(t, mgr.CompleteRun(ctx, 7, 8, 9, &entity.Session{UserID: "u"}, entity.WithStatus(tc.status)))
			requireWriterReceipt(t, provider)
		})
	}
}

func TestCompleteRunPersistsBeforeLifecycleUnlock(t *testing.T) {
	mgr, provider := newReceiptWriterManager(t)
	ctx := context.Background()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().ListTurnResult(ctx, int64(9), int64(7), nil, gomock.Any(), false).
		Return([]*entity.ExptTurnResult{}, int64(0), nil)
	mgr.mutex.(*lockMocks.MockILocker).EXPECT().UnlockForce(ctx, mgr.makeExptMutexLockKey(7)).
		DoAndReturn(func(context.Context, string) (bool, error) {
			var row model.ExptRunLog
			require.NoError(t, provider.NewSession(ctx).First(&row, 8).Error)
			require.Equal(t, int64(entity.ExptStatus_Terminated), gptr.Indirect(row.Status))
			return true, nil
		})
	require.NoError(t, mgr.CompleteRun(ctx, 7, 8, 9, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Terminated)))
}

func TestRecordExptDataKeepsReceipt(t *testing.T) {
	mgr, provider := newReceiptWriterManager(t)
	ctx := context.Background()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().ListTurnResult(ctx, int64(9), int64(7), nil, gomock.Any(), false).
		DoAndReturn(func(context.Context, int64, int64, *entity.ExptTurnResultFilter, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
			injectWriterReceipt(t, provider)
			return []*entity.ExptTurnResult{{Status: int32(entity.TurnRunState_Success)}}, int64(1), nil
		})
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().CalculateStats(ctx, int64(7), int64(9), gomock.Any()).Return(&entity.ExptCalculateStats{SuccessItemCnt: 1}, nil)
	require.NoError(t, mgr.RecordExptData(ctx, 7, 8, 9, &entity.Session{}))
	requireWriterReceipt(t, provider)
}
