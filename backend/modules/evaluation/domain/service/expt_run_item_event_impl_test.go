// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestNewExptRecordEvalService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := NewExptRecordEvalService(
		svcmocks.NewMockIExptManager(ctrl),
		componentMocks.NewMockIConfiger(ctrl),
		eventmocks.NewMockExptEventPublisher(ctrl),
		repoMocks.NewMockIExptItemResultRepo(ctrl),
		repoMocks.NewMockIExptTurnResultRepo(ctrl),
		repoMocks.NewMockIExptStatsRepo(ctrl),
		repoMocks.NewMockIExperimentRepo(ctrl),
		repoMocks.NewMockIExptItemRefRepo(ctrl),
		repoMocks.NewMockQuotaRepo(ctrl),
		lockmocks.NewMockILocker(ctrl),
		idemmocks.NewMockIdempotentService(ctrl),
		auditmocks.NewMockIAuditService(ctrl),
		metricsmocks.NewMockExptMetric(ctrl),
		svcmocks.NewMockExptResultService(ctrl),
		svcmocks.NewMockIEvalTargetService(ctrl),
		svcmocks.NewMockEvaluationSetItemService(ctrl),
		svcmocks.NewMockEvaluatorRecordService(ctrl),
		svcmocks.NewMockEvaluatorService(ctrl),
		idgenmocks.NewMockIIDGenerator(ctrl),
		benefitmocks.NewMockIBenefitService(ctrl),
		repoMocks.NewMockIEvalAsyncRepo(ctrl),
		nil,
	)
	assert.NotNil(t, service)
}

func TestExptItemEventEvalServiceImpl_Eval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockPublisher := eventmocks.NewMockExptEventPublisher(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockQuotaRepo := repoMocks.NewMockQuotaRepo(ctrl)
	mockMutex := lockmocks.NewMockILocker(ctrl)
	mockIdem := idemmocks.NewMockIdempotentService(ctrl)
	mockAudit := auditmocks.NewMockIAuditService(ctrl)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockResultSvc := svcmocks.NewMockExptResultService(ctrl)
	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockEvaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockBenefit := benefitmocks.NewMockIBenefitService(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		publisher:                mockPublisher,
		exptItemResultRepo:       mockExptItemResultRepo,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptStatsRepo:            mockExptStatsRepo,
		experimentRepo:           mockExperimentRepo,
		configer:                 mockConfiger,
		quotaRepo:                mockQuotaRepo,
		mutex:                    mockMutex,
		idem:                     mockIdem,
		auditClient:              mockAudit,
		metric:                   mockMetric,
		resultSvc:                mockResultSvc,
		evaTargetService:         mockEvalTargetSvc,
		evaluationSetItemService: mockEvalSetItemSvc,
		evaluatorRecordService:   mockEvaluatorRecordSvc,
		evaluatorService:         mockEvaluatorSvc,
		idgen:                    mockIdgen,
		benefitService:           mockBenefit,
	}

	// Test case for event stream
	tests := []struct {
		name    string
		prepare func()
		event   *entity.ExptItemEvalEvent
		wantErr bool
	}{
		{
			name: "Normal flow - all success",
			prepare: func() {
				// Mock all endpoints returning nil
				service.endpoints = func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
					return nil
				}
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			wantErr: false,
		},
		{
			name: "Chain returns error",
			prepare: func() {
				service.endpoints = func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
					return errors.New("mock error")
				}
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			err := service.Eval(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptItemEventEvalServiceImpl_HandleEventCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	service := &ExptItemEventEvalServiceImpl{
		manager: mockManager,
	}

	tests := []struct {
		name    string
		prepare func()
		event   *entity.ExptItemEvalEvent
		wantErr bool
	}{
		{
			name: "Expt finished - return nil",
			prepare: func() {
				mockManager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil)
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			wantErr: false,
		},
		{
			name: "Expt terminating - return nil",
			prepare: func() {
				mockManager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Terminating)}, nil)
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			wantErr: false,
		},
		{
			name: "Expt draining - return nil",
			prepare: func() {
				mockManager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Draining)}, nil)
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			wantErr: false,
		},
		{
			name: "Expt processing - continue",
			prepare: func() {
				mockManager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil)
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			wantErr: false,
		},
		{
			name: "Get run log failed",
			prepare: func() {
				mockManager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("mock error"))
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			nextCalled := false
			next := func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				nextCalled = true
				return nil
			}
			handler := service.HandleEventCheck(next)
			err := handler(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, nextCalled)
			} else {
				assert.NoError(t, err)
				if tt.name == "Expt processing - continue" {
					assert.True(t, nextCalled)
				} else {
					assert.False(t, nextCalled)
				}
			}
		})
	}
}

func TestExptItemEventEvalServiceImpl_HandleEventErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockPublisher := eventmocks.NewMockExptEventPublisher(ctrl)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:   mockManager,
		configer:  mockConfiger,
		publisher: mockPublisher,
		metric:    mockMetric,
	}

	tests := []struct {
		name    string
		prepare func()
		event   *entity.ExptItemEvalEvent
		nextErr error
		wantErr bool
	}{
		{
			name: "Success - no retry",
			prepare: func() {
				mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.RetryConf{
						RetryTimes:          3,
						RetryIntervalSecond: 60,
						IsInDebt:            false,
					})
				mockMetric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			nextErr: nil,
			wantErr: false,
		},
		{
			name: "Failed - retry needed",
			prepare: func() {
				mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.RetryConf{
						RetryTimes:          3,
						RetryIntervalSecond: 60,
						IsInDebt:            false,
					})
				mockPublisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockMetric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), true, true, gomock.Any(), gomock.Any(), gomock.Any())
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, RetryTimes: 1},
			nextErr: errors.New("mock error"),
			wantErr: false,
		},
		{
			name: "Failed - retry limit exceeded",
			prepare: func() {
				mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.RetryConf{
						RetryTimes:          3,
						RetryIntervalSecond: 60,
						IsInDebt:            false,
					})
				mockMetric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), true, false, gomock.Any(), gomock.Any(), gomock.Any())
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, RetryTimes: 3},
			nextErr: errors.New("mock error"),
			wantErr: false,
		},
		{
			name: "Failed - in debt termination",
			prepare: func() {
				mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.RetryConf{
						RetryTimes:          3,
						RetryIntervalSecond: 60,
						IsInDebt:            true,
					})
				// CompleteRun: ctx, exptID, exptRunID, spaceID, session, WithCID, WithCompleteInterval (7 parameters)
				mockManager.EXPECT().CompleteRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				// CompleteExpt: ctx, exptID, spaceID, session, WithStatus, WithStatusMessage, WithCID, WithCompleteInterval (8 parameters)
				mockManager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockMetric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
			nextErr: errors.New("mock error"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			next := func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				return tt.nextErr
			}
			handler := service.HandleEventErr(next)
			err := handler(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptItemEventEvalServiceImpl_HandleEventLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMutex := lockmocks.NewMockILocker(ctrl)
	service := &ExptItemEventEvalServiceImpl{
		mutex: mockMutex,
	}

	tests := []struct {
		name    string
		prepare func()
		event   *entity.ExptItemEvalEvent
		wantErr bool
	}{
		{
			name: "Acquire lock success",
			prepare: func() {
				mockMutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true, context.Background(), func() {}, nil)
				mockMutex.EXPECT().Unlock(gomock.Any()).Return(true, nil)
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3},
			wantErr: false,
		},
		{
			name: "Acquire lock failed - already occupied",
			prepare: func() {
				mockMutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false, nil, nil, nil)
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3},
			wantErr: false,
		},
		{
			name: "Acquire lock failed - error",
			prepare: func() {
				mockMutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false, nil, nil, errors.New("mock error"))
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			nextCalled := false
			next := func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				nextCalled = true
				return nil
			}
			handler := service.HandleEventLock(next)
			err := handler(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, nextCalled)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.name == "Acquire lock success", nextCalled)
			}
		})
	}
}

func TestExptItemEventEvalServiceImpl_WithCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := &ExptItemEventEvalServiceImpl{}

	tests := []struct {
		name      string
		eiec      *entity.ExptItemEvalCtx
		wantLogID string
	}{
		{
			name: "Normal flow",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
				},
				Expt: &entity.Experiment{
					SourceID: "test_source",
				},
			},
			wantLogID: "test_source:1:2:3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			service.WithCtx(ctx, tt.eiec)
		})
	}
}

func TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemSvc,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptItemResultRepo:       mockExptItemResultRepo,
	}

	mockExpt := &entity.Experiment{
		ID: 1,
		EvalSet: &entity.EvaluationSet{
			EvaluationSetVersion: &entity.EvaluationSetVersion{
				ID:              1,
				EvaluationSetID: 1,
			},
		},
	}

	mockEvalSetItem := &entity.EvaluationSetItem{
		ID: 1,
	}

	tests := []struct {
		name    string
		prepare func()
		event   *entity.ExptItemEvalEvent
		want    *entity.ExptItemEvalCtx
		wantErr bool
	}{
		{
			name: "Normal flow",
			prepare: func() {
				mockManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(mockExpt, nil)
				mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{mockEvalSetItem}, nil)
				mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)
				mockExptItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptItemResultRunLog{}, nil).AnyTimes().AnyTimes()
			},
			event: &entity.ExptItemEvalEvent{
				ExptID:        1,
				SpaceID:       3,
				EvalSetItemID: 1,
			},
			want: &entity.ExptItemEvalCtx{
				Event:       &entity.ExptItemEvalEvent{ExptID: 1, SpaceID: 3, EvalSetItemID: 1},
				Expt:        mockExpt,
				EvalSetItem: mockEvalSetItem,
			},
			wantErr: false,
		},
		{
			name: "Get expt detail failed",
			prepare: func() {
				mockManager.EXPECT().GetDetail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			event: &entity.ExptItemEvalEvent{
				ExptID:  1,
				SpaceID: 3,
			},
			wantErr: true,
		},
		{
			name: "Get eval set item failed",
			prepare: func() {
				mockManager.EXPECT().GetDetail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockExpt, nil)
				mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			event: &entity.ExptItemEvalEvent{
				ExptID:  1,
				SpaceID: 3,
			},
			wantErr: true,
		},
		{
			name: "Eval set item count mismatch",
			prepare: func() {
				mockManager.EXPECT().GetDetail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockExpt, nil)
				mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{}, nil)
			},
			event: &entity.ExptItemEvalEvent{
				ExptID:  1,
				SpaceID: 3,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			got, err := service.BuildExptRecordEvalCtx(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want.Event, got.Event)
			assert.Equal(t, tt.want.Expt, got.Expt)
			assert.Equal(t, tt.want.EvalSetItem, got.EvalSetItem)
		})
	}
}

// TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_MultiSet 覆盖 item-centric 多评测集场景:
// 非主集 item 必须用 expt_item_ref 里 per-item 的 (eval_set_id, eval_set_version_id) 去拉 item,
// 而不是实验级主集 —— 否则非主集 item 用主集去捞返回 0 条, 报错卡死永远 incomplete。
func TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_MultiSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptItemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemSvc,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptItemResultRepo:       mockExptItemResultRepo,
		exptItemRefRepo:          mockExptItemRefRepo,
	}

	const (
		primarySetID = int64(100)
		primaryVerID = int64(101)
		secondSetID  = int64(200)
		secondVerID  = int64(201)
		secondItemID = int64(2002)
	)

	// 实验主集是 set1; 待执行的是 set2 的 item
	mockExpt := &entity.Experiment{
		ID:                1,
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSet: &entity.EvaluationSet{
			EvaluationSetVersion: &entity.EvaluationSetVersion{
				ID:              primaryVerID,
				EvaluationSetID: primarySetID,
			},
		},
	}
	itemConfig := &entity.ExptItemConfig{}
	secondRef := &entity.ExptItemRef{
		ItemID:           secondItemID,
		EvalSetID:        secondSetID,
		EvalSetVersionID: secondVerID,
		ItemConfig:       itemConfig,
	}

	mockManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(mockExpt, nil)
	mockExptItemRefRepo.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(3), int64(1), secondItemID).Return(secondRef, nil)
	// ★ 关键断言: 统一走 ItemVersionQueries; 老数据集(ref 无 item 版本) query 只带 ItemID, 集 id/version 用 ref 里 set2 的
	mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
			assert.Equal(t, secondSetID, param.EvaluationSetID)
			assert.NotNil(t, param.VersionID)
			assert.Equal(t, secondVerID, *param.VersionID)
			assert.Empty(t, param.ItemIDs)
			assert.Len(t, param.ItemVersionQueries, 1)
			assert.Equal(t, secondItemID, param.ItemVersionQueries[0].ItemID)
			assert.Nil(t, param.ItemVersionQueries[0].ItemVersionID) // 老数据集: versionID 留空
			return []*entity.EvaluationSetItem{{ID: secondItemID, ItemID: secondItemID}}, nil
		})
	mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)
	mockExptItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptItemResultRunLog{}, nil).AnyTimes()

	got, err := service.BuildExptRecordEvalCtx(context.Background(), &entity.ExptItemEvalEvent{
		ExptID:        1,
		SpaceID:       3,
		EvalSetItemID: secondItemID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, secondItemID, got.EvalSetItem.ItemID)
	assert.Equal(t, itemConfig, got.ItemConfig)
}

// TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_MultiSet_RefFail 回归:
// MultiSetConfig 实验里读 expt_item_ref 失败(repo 报错 / ref==nil / ref.ItemConfig==nil)时,
// BuildExptRecordEvalCtx 必须返回 error(触发重试),不能静默降级为 itemConfig=nil。
// 否则下游 CallEvaluators 会把 nil ItemConfig 当成"合法空评估器集"跑 0 个评估器并把 turn 标 Success —
// 本该有评估器的正常集被静默漏评还显示成功(fail-silent)。
// 正常调度 exptStartMultiSet 对每个 item 都写非 nil ItemConfig, 所以读到 nil 只可能是读失败, 一律报错。
func TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_MultiSet_RefFail(t *testing.T) {
	const (
		primarySetID = int64(100)
		primaryVerID = int64(101)
		itemID       = int64(2002)
	)
	newMultiSetExpt := func() *entity.Experiment {
		return &entity.Experiment{
			ID:                1,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
			EvalSet: &entity.EvaluationSet{
				EvaluationSetVersion: &entity.EvaluationSetVersion{
					ID:              primaryVerID,
					EvaluationSetID: primarySetID,
				},
			},
		}
	}

	tests := []struct {
		name  string
		refFn func() (*entity.ExptItemRef, error)
	}{
		{
			name:  "repo error -> return err, no silent fallback",
			refFn: func() (*entity.ExptItemRef, error) { return nil, errors.New("db timeout") },
		},
		{
			name:  "ref nil -> return err",
			refFn: func() (*entity.ExptItemRef, error) { return nil, nil },
		},
		{
			name: "ref with nil ItemConfig -> return err",
			refFn: func() (*entity.ExptItemRef, error) {
				return &entity.ExptItemRef{ItemID: itemID, EvalSetID: primarySetID, EvalSetVersionID: primaryVerID, ItemConfig: nil}, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockManager := svcmocks.NewMockIExptManager(ctrl)
			mockExptItemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)
			service := &ExptItemEventEvalServiceImpl{
				manager:         mockManager,
				exptItemRefRepo: mockExptItemRefRepo,
			}

			ref, refErr := tt.refFn()
			mockManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(newMultiSetExpt(), nil)
			mockExptItemRefRepo.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(3), int64(1), itemID).Return(ref, refErr)
			// 关键: 报错即返回, 不应再往下调 BatchGetEvaluationSetItems 等(不给这些 mock 期望, 一旦调用 gomock 会 fail)。

			got, err := service.BuildExptRecordEvalCtx(context.Background(), &entity.ExptItemEvalEvent{
				ExptID:        1,
				SpaceID:       3,
				EvalSetItemID: itemID,
			})
			assert.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

// 草稿集 item: ref 落 EvalSetVersionID=0 + ItemVersionID=0 → 单行取数按 item_id (version=nil) 读当前草稿。
// 冻结粒度=item_id 集合 (ExptStart 固定了哪些 item 参与); item 内容是实时读 (草稿无 item 版本, 这是预期)。
func TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_MultiSet_Draft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptItemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemSvc,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptItemResultRepo:       mockExptItemResultRepo,
		exptItemRefRepo:          mockExptItemRefRepo,
	}

	const (
		primarySetID = int64(100)
		primaryVerID = int64(101)
		draftSetID   = int64(7656754417005232130)
		draftItemID  = int64(2002)
	)

	mockExpt := &entity.Experiment{
		ID:                1,
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSet: &entity.EvaluationSet{
			EvaluationSetVersion: &entity.EvaluationSetVersion{
				ID:              primaryVerID,
				EvaluationSetID: primarySetID,
			},
		},
	}
	// 草稿集 ref: EvalSetVersionID 落 0 (扫描层 resolveSetRefVersionID 的口径)
	draftRef := &entity.ExptItemRef{
		ItemID:           draftItemID,
		EvalSetID:        draftSetID,
		EvalSetVersionID: 0,
		ItemConfig:       &entity.ExptItemConfig{},
	}

	mockManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(mockExpt, nil)
	mockExptItemRefRepo.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(3), int64(1), draftItemID).Return(draftRef, nil)
	// ★ 关键断言: 草稿集 VersionID 必须为 nil → 走 live 读当前草稿; 集 id 用 ref 里草稿集
	mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
			assert.Equal(t, draftSetID, param.EvaluationSetID)
			assert.Nil(t, param.VersionID, "草稿集执行侧取数 VersionID 必须为 nil (live)")
			assert.Len(t, param.ItemVersionQueries, 1)
			assert.Equal(t, draftItemID, param.ItemVersionQueries[0].ItemID)
			return []*entity.EvaluationSetItem{{ID: draftItemID, ItemID: draftItemID}}, nil
		})
	mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)
	mockExptItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptItemResultRunLog{}, nil).AnyTimes()

	got, err := service.BuildExptRecordEvalCtx(context.Background(), &entity.ExptItemEvalEvent{
		ExptID:        1,
		SpaceID:       3,
		EvalSetItemID: draftItemID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, draftItemID, got.EvalSetItem.ItemID)
}

// 新数据集: ref 带 ItemVersionID → 单行取数走 ItemVersionQueries (item_id + item_version_id), 不走 ItemIDs。
func TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_MultiSet_ItemVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptItemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemSvc,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptItemResultRepo:       mockExptItemResultRepo,
		exptItemRefRepo:          mockExptItemRefRepo,
	}

	const (
		setID     = int64(300)
		setVerID  = int64(301)
		itemID    = int64(3003)
		itemVerID = int64(999)
	)

	mockExpt := &entity.Experiment{
		ID:                1,
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSet: &entity.EvaluationSet{
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: setVerID, EvaluationSetID: setID},
		},
	}
	itemConfig := &entity.ExptItemConfig{}
	ref := &entity.ExptItemRef{
		ItemID:           itemID,
		ItemVersionID:    itemVerID, // ★ 带 item 级版本
		EvalSetID:        setID,
		EvalSetVersionID: setVerID,
		ItemConfig:       itemConfig,
	}

	mockManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(mockExpt, nil)
	mockExptItemRefRepo.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(3), int64(1), itemID).Return(ref, nil)
	// ★ 关键断言: query 带 item_version_id (新数据集); 集 VersionID 仍透传 (setID != verID)
	mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
			assert.Equal(t, setID, param.EvaluationSetID)
			assert.NotNil(t, param.VersionID)
			assert.Empty(t, param.ItemIDs)
			assert.Len(t, param.ItemVersionQueries, 1)
			assert.Equal(t, itemID, param.ItemVersionQueries[0].ItemID)
			assert.NotNil(t, param.ItemVersionQueries[0].ItemVersionID)
			assert.Equal(t, itemVerID, *param.ItemVersionQueries[0].ItemVersionID)
			retVerID := itemVerID
			return []*entity.EvaluationSetItem{{ID: itemID, ItemID: itemID, ItemVersionID: &retVerID}}, nil
		})
	mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)
	mockExptItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptItemResultRunLog{}, nil).AnyTimes()

	got, err := service.BuildExptRecordEvalCtx(context.Background(), &entity.ExptItemEvalEvent{
		ExptID:        1,
		SpaceID:       3,
		EvalSetItemID: itemID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, itemID, got.EvalSetItem.ItemID)
}

// TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_SingleSet_ItemVersionFromRunLog
// 老链路 (SingleSet) 版本评测集: ExptStart 已把 item 版本落进 run_log, 单行执行从 run_log
// 读回并带进 ItemVersionQueries, 使下游能按 item 版本取数 (修复版本评测集走旧链路时版本缺失)。
func TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_SingleSet_ItemVersionFromRunLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptItemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemSvc,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptItemResultRepo:       mockExptItemResultRepo,
		exptItemRefRepo:          mockExptItemRefRepo,
	}

	const (
		setID     = int64(400)
		setVerID  = int64(401) // committed 版本集 (setVerID != setID)
		itemID    = int64(4004)
		itemVerID = int64(888)
	)

	// SingleSet 老实验: 不读 expt_item_ref。
	mockExpt := &entity.Experiment{
		ID:                1,
		EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
		EvalSet: &entity.EvaluationSet{
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: setVerID, EvaluationSetID: setID},
		},
	}

	mockManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(mockExpt, nil)
	// ★ run_log 带 item 版本 (ExptStart 已落); SingleSet 不进 ref 分支, itemVersionID 由此读回。
	mockExptItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.ExptItemResultRunLog{ItemID: itemID, ItemVersionID: itemVerID}, nil).AnyTimes()
	// ★ 关键断言: 老链路 query 也带上了 item_version_id
	mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
			assert.Equal(t, setID, param.EvaluationSetID)
			assert.NotNil(t, param.VersionID) // committed 集, 集 VersionID 保留
			assert.Len(t, param.ItemVersionQueries, 1)
			assert.Equal(t, itemID, param.ItemVersionQueries[0].ItemID)
			assert.NotNil(t, param.ItemVersionQueries[0].ItemVersionID)
			assert.Equal(t, itemVerID, *param.ItemVersionQueries[0].ItemVersionID)
			retVerID := itemVerID
			return []*entity.EvaluationSetItem{{ID: itemID, ItemID: itemID, ItemVersionID: &retVerID}}, nil
		})
	mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)

	got, err := service.BuildExptRecordEvalCtx(context.Background(), &entity.ExptItemEvalEvent{
		ExptID:        1,
		SpaceID:       3,
		EvalSetItemID: itemID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, itemID, got.EvalSetItem.ItemID)
}

// TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_SingleSet_NoItemVersion
// 老链路 + 无版本评测集 (run_log.ItemVersionID==0): query 不带 item 版本, 按集版本定位, 行为不变 (回归守卫)。
func TestExptItemEventEvalServiceImpl_BuildExptRecordEvalCtx_SingleSet_NoItemVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptItemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemSvc,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptItemResultRepo:       mockExptItemResultRepo,
		exptItemRefRepo:          mockExptItemRefRepo,
	}

	const (
		setID    = int64(500)
		setVerID = int64(501)
		itemID   = int64(5005)
	)

	mockExpt := &entity.Experiment{
		ID:                1,
		EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
		EvalSet: &entity.EvaluationSet{
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: setVerID, EvaluationSetID: setID},
		},
	}

	mockManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(mockExpt, nil)
	// run_log 无 item 版本 (老数据集)
	mockExptItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.ExptItemResultRunLog{ItemID: itemID, ItemVersionID: 0}, nil).AnyTimes()
	mockEvalSetItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
			assert.Len(t, param.ItemVersionQueries, 1)
			assert.Equal(t, itemID, param.ItemVersionQueries[0].ItemID)
			assert.Nil(t, param.ItemVersionQueries[0].ItemVersionID) // 无版本 → 不带 item 版本
			return []*entity.EvaluationSetItem{{ID: itemID, ItemID: itemID}}, nil
		})
	mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)

	got, err := service.BuildExptRecordEvalCtx(context.Background(), &entity.ExptItemEvalEvent{
		ExptID:        1,
		SpaceID:       3,
		EvalSetItemID: itemID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, itemID, got.EvalSetItem.ItemID)
}

func TestExptItemEventEvalServiceImpl_GetExistExptRecordEvalResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		exptTurnResultRepo: mockTurnResultRepo,
		exptItemResultRepo: mockItemResultRepo,
	}

	mockTurnRunLogs := []*entity.ExptTurnResultRunLog{
		{
			ID:     1,
			ItemID: 1,
			TurnID: 1,
		},
	}

	mockItemRunLog := &entity.ExptItemResultRunLog{
		ID:     1,
		ItemID: 1,
	}

	tests := []struct {
		name    string
		prepare func()
		event   *entity.ExptItemEvalEvent
		want    *entity.ExptItemEvalResult
		wantErr bool
	}{
		{
			name: "Normal flow",
			prepare: func() {
				mockTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), int64(1), int64(3)).Return(mockTurnRunLogs, nil)
				mockItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), int64(1), int64(2), int64(1), int64(3)).Return(mockItemRunLog, nil)
			},
			event: &entity.ExptItemEvalEvent{
				ExptID:        1,
				ExptRunID:     2,
				EvalSetItemID: 1,
				SpaceID:       3,
			},
			want: &entity.ExptItemEvalResult{
				ItemResultRunLog: mockItemRunLog,
				TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
					1: mockTurnRunLogs[0],
				},
			},
			wantErr: false,
		},
		{
			name: "Get turn run logs failed",
			prepare: func() {
				mockTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			event: &entity.ExptItemEvalEvent{
				ExptID:        1,
				ExptRunID:     2,
				EvalSetItemID: 1,
				SpaceID:       3,
			},
			wantErr: true,
		},
		{
			name: "Get item run log failed",
			prepare: func() {
				mockTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTurnRunLogs, nil)
				mockItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			event: &entity.ExptItemEvalEvent{
				ExptID:        1,
				ExptRunID:     2,
				EvalSetItemID: 1,
				SpaceID:       3,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			got, err := service.GetExistExptRecordEvalResult(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want.ItemResultRunLog, got.ItemResultRunLog)
			assert.Equal(t, tt.want.TurnResultRunLogs, got.TurnResultRunLogs)
		})
	}
}

func TestNewRecordEvalMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockResultSvc := svcmocks.NewMockExptResultService(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvalTarget := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecord := svcmocks.NewMockEvaluatorRecordService(ctrl)

	tests := []struct {
		name    string
		event   *entity.ExptItemEvalEvent
		want    RecordEvalMode
		wantErr bool
	}{
		{
			name: "Submit mode",
			event: &entity.ExptItemEvalEvent{
				ExptRunMode: entity.EvaluationModeSubmit,
			},
			want:    &ExptRecordEvalModeSubmit{},
			wantErr: false,
		},
		{
			name: "Append mode",
			event: &entity.ExptItemEvalEvent{
				ExptRunMode: entity.EvaluationModeAppend,
			},
			want:    &ExptRecordEvalModeSubmit{},
			wantErr: false,
		},
		{
			name: "FailRetry mode",
			event: &entity.ExptItemEvalEvent{
				ExptRunMode: entity.EvaluationModeFailRetry,
			},
			want:    &ExptRecordEvalModeFailRetry{},
			wantErr: false,
		},
		{
			name: "Unknown mode",
			event: &entity.ExptItemEvalEvent{
				ExptRunMode: entity.ExptRunMode(999),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRecordEvalMode(tt.event, mockExptItemResultRepo, mockExptTurnResultRepo, mockExptStatsRepo, mockExperimentRepo, mockMetric, mockResultSvc, mockIdgen, mockEvalTarget, mockEvaluatorRecord)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.IsType(t, tt.want, got)
		})
	}
}

func TestExptRecordEvalModeSubmit_PreEval(t *testing.T) {
	mockEvalSetItem := &entity.EvaluationSetItem{
		ID: 1,
		Turns: []*entity.Turn{
			{ID: 1},
		},
	}

	tests := []struct {
		name    string
		prepare func(mockExptItemResultRepo *repoMocks.MockIExptItemResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockIdgen *idgenmocks.MockIIDGenerator)
		eiec    *entity.ExptItemEvalCtx
		wantErr bool
	}{
		{
			name: "Normal flow",
			prepare: func(_ *repoMocks.MockIExptItemResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, _ *idgenmocks.MockIIDGenerator) {
				// placeholder to satisfy type; real expectations set below per-correct types
			},
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{
					ExptID:        1,
					ExptRunID:     2,
					SpaceID:       3,
					EvalSetItemID: 1,
				},
				EvalSetItem: mockEvalSetItem,
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
				},
			},
			wantErr: false,
		},
		{
			name: "Generate ID failed",
			prepare: func(_ *repoMocks.MockIExptItemResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockIdgen *idgenmocks.MockIIDGenerator) {
				mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)
				mockIdgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			eiec: &entity.ExptItemEvalCtx{
				Event:       &entity.ExptItemEvalEvent{},
				EvalSetItem: mockEvalSetItem,
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
				},
			},
			wantErr: true,
		},
		{
			name: "Create run log failed",
			prepare: func(_ *repoMocks.MockIExptItemResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockIdgen *idgenmocks.MockIIDGenerator) {
				mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)
				mockIdgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockExptTurnResultRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).Return(errors.New("mock error"))
			},
			eiec: &entity.ExptItemEvalCtx{
				Event:       &entity.ExptItemEvalEvent{},
				EvalSetItem: mockEvalSetItem,
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)

			// Set expectations for each sub-test
			if tt.name == "Normal flow" {
				mockExptTurnResultRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultRunLog{}, nil)
				mockIdgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockExptTurnResultRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).Return(nil)
			} else {
				tt.prepare(mockExptItemResultRepo, mockExptTurnResultRepo, mockIdgen)
			}

			mode := &ExptRecordEvalModeSubmit{
				exptItemResultRepo: mockExptItemResultRepo,
				exptTurnResultRepo: mockExptTurnResultRepo,
				idgen:              mockIdgen,
			}

			err := mode.PreEval(context.Background(), tt.eiec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestExptRecordEvalModeSubmit_PostEval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mode := &ExptRecordEvalModeSubmit{}

	tests := []struct {
		name    string
		eiec    *entity.ExptItemEvalCtx
		wantErr bool
	}{
		{
			name:    "Normal flow",
			eiec:    &entity.ExptItemEvalCtx{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mode.PostEval(context.Background(), tt.eiec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestExptRecordEvalModeFailRetry_PreEval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockResultSvc := svcmocks.NewMockExptResultService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)

	mode := &ExptRecordEvalModeFailRetry{
		resultSvc:          mockResultSvc,
		exptTurnResultRepo: mockExptTurnResultRepo,
		idgen:              mockIdgen,
		evalTargetService:  nil,
		evaluatorRecordSvc: nil,
	}

	mockTurnResults := []*entity.ExptTurnResult{
		{ID: 1},
	}

	tests := []struct {
		name    string
		prepare func()
		eiec    *entity.ExptItemEvalCtx
		wantErr bool
	}{
		{
			name: "Normal flow",
			prepare: func() {
				mockResultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTurnResults, nil)
				mockIdgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockExptTurnResultRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).Return(nil)
			},
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
				},
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
				},
			},
			wantErr: false,
		},
		{
			name: "Get turn results failed",
			prepare: func() {
				mockResultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{},
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
				},
			},
			wantErr: true,
		},
		{
			name: "Generate ID failed",
			prepare: func() {
				mockResultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTurnResults, nil)
				mockIdgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{},
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
				},
			},
			wantErr: true,
		},
		{
			name: "Create run log failed",
			prepare: func() {
				mockResultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTurnResults, nil)
				mockIdgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1}, nil)
				mockExptTurnResultRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).Return(errors.New("mock error"))
			},
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{},
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			err := mode.PreEval(context.Background(), tt.eiec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestExptRecordEvalModeFailRetry_PostEval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mode := &ExptRecordEvalModeFailRetry{}

	tests := []struct {
		name    string
		eiec    *entity.ExptItemEvalCtx
		wantErr bool
	}{
		{
			name:    "Normal flow",
			eiec:    &entity.ExptItemEvalCtx{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mode.PostEval(context.Background(), tt.eiec)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestExptItemEventEvalServiceImpl_HandleEventExec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManager := svcmocks.NewMockIExptManager(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockBenefit := benefitmocks.NewMockIBenefitService(ctrl)
	mockEvalAsyncRepo := repoMocks.NewMockIEvalAsyncRepo(ctrl)
	mockResultSvc := svcmocks.NewMockExptResultService(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)

	service := &ExptItemEventEvalServiceImpl{
		manager:                  mockManager,
		evaluationSetItemService: mockEvalSetItemSvc,
		exptTurnResultRepo:       mockExptTurnResultRepo,
		exptItemResultRepo:       mockExptItemResultRepo,
		configer:                 mockConfiger,
		metric:                   mockMetric,
		evaTargetService:         mockEvalTargetSvc,
		evaluatorRecordService:   mockEvaluatorRecordSvc,
		evaluatorService:         mockEvaluatorSvc,
		benefitService:           mockBenefit,
		evalAsyncRepo:            mockEvalAsyncRepo,
		resultSvc:                mockResultSvc,
		idgen:                    mockIdgen,
		experimentRepo:           mockExperimentRepo,
		exptStatsRepo:            mockExptStatsRepo,
	}

	tests := []struct {
		name    string
		prepare func()
		event   *entity.ExptItemEvalEvent
		wantErr bool
	}{
		{
			name: "Eval error",
			prepare: func() {
				mockManager.EXPECT().GetDetail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			event:   &entity.ExptItemEvalEvent{ExptID: 1, SpaceID: 3, EvalSetItemID: 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			nextCalled := false
			next := func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				nextCalled = true
				return nil
			}
			handler := service.HandleEventExec(next)
			err := handler(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, nextCalled)
			} else {
				assert.NoError(t, err)
				assert.True(t, nextCalled)
			}
		})
	}
}

func Test_failRetrySelectTurnRunLogRefs(t *testing.T) {
	tests := []struct {
		name            string
		spaceID         int64
		tr              *entity.ExptTurnResult
		setupEvalTarget func(ctrl *gomock.Controller) IEvalTargetService
		setupEvalRecord func(ctrl *gomock.Controller) EvaluatorRecordService
		wantTargetID    int64
		wantEvalResults *entity.EvaluatorResults
	}{
		{
			name:    "tr is nil -> returns 0, nil",
			spaceID: 1,
			tr:      nil,
			setupEvalTarget: func(ctrl *gomock.Controller) IEvalTargetService {
				return svcmocks.NewMockIEvalTargetService(ctrl)
			},
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				return svcmocks.NewMockEvaluatorRecordService(ctrl)
			},
			wantTargetID:    0,
			wantEvalResults: nil,
		},
		{
			name:    "TargetResultID > 0, evalTarget is nil -> returns 0, nil",
			spaceID: 1,
			tr: &entity.ExptTurnResult{
				TargetResultID: 100,
			},
			setupEvalTarget: func(ctrl *gomock.Controller) IEvalTargetService {
				return nil
			},
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				return svcmocks.NewMockEvaluatorRecordService(ctrl)
			},
			wantTargetID:    0,
			wantEvalResults: nil,
		},
		{
			name:    "TargetResultID > 0, GetRecordByID returns error -> returns 0, nil",
			spaceID: 1,
			tr: &entity.ExptTurnResult{
				TargetResultID: 100,
			},
			setupEvalTarget: func(ctrl *gomock.Controller) IEvalTargetService {
				m := svcmocks.NewMockIEvalTargetService(ctrl)
				m.EXPECT().GetRecordByID(gomock.Any(), int64(1), int64(100)).Return(nil, errors.New("db error"))
				return m
			},
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				return svcmocks.NewMockEvaluatorRecordService(ctrl)
			},
			wantTargetID:    0,
			wantEvalResults: nil,
		},
		{
			name:    "TargetResultID > 0, target record status is not Success -> returns 0, nil",
			spaceID: 1,
			tr: &entity.ExptTurnResult{
				TargetResultID: 100,
			},
			setupEvalTarget: func(ctrl *gomock.Controller) IEvalTargetService {
				m := svcmocks.NewMockIEvalTargetService(ctrl)
				failStatus := entity.EvalTargetRunStatusFail
				m.EXPECT().GetRecordByID(gomock.Any(), int64(1), int64(100)).Return(&entity.EvalTargetRecord{
					Status: &failStatus,
				}, nil)
				return m
			},
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				return svcmocks.NewMockEvaluatorRecordService(ctrl)
			},
			wantTargetID:    0,
			wantEvalResults: nil,
		},
		{
			name:    "TargetResultID > 0, target record status is Success, with evaluator records -> returns targetResultID and pruned results",
			spaceID: 1,
			tr: &entity.ExptTurnResult{
				TargetResultID: 100,
				EvaluatorResults: &entity.EvaluatorResults{
					EvalVerIDToResID: map[int64]int64{
						10: 1001,
						20: 1002,
					},
				},
			},
			setupEvalTarget: func(ctrl *gomock.Controller) IEvalTargetService {
				m := svcmocks.NewMockIEvalTargetService(ctrl)
				successStatus := entity.EvalTargetRunStatusSuccess
				m.EXPECT().GetRecordByID(gomock.Any(), int64(1), int64(100)).Return(&entity.EvalTargetRecord{
					Status: &successStatus,
				}, nil)
				return m
			},
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				m := svcmocks.NewMockEvaluatorRecordService(ctrl)
				m.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{
					{ID: 1001, EvaluatorVersionID: 10, Status: entity.EvaluatorRunStatusSuccess},
					{ID: 1002, EvaluatorVersionID: 20, Status: entity.EvaluatorRunStatusFail},
				}, nil)
				return m
			},
			wantTargetID: 100,
			wantEvalResults: &entity.EvaluatorResults{
				EvalVerIDToResID: map[int64]int64{
					10: 1001,
				},
			},
		},
		{
			name:    "TargetResultID == 0, with evaluator records -> returns 0 and pruned results",
			spaceID: 1,
			tr: &entity.ExptTurnResult{
				TargetResultID: 0,
				EvaluatorResults: &entity.EvaluatorResults{
					EvalVerIDToResID: map[int64]int64{
						10: 1001,
						20: 1002,
					},
				},
			},
			setupEvalTarget: func(ctrl *gomock.Controller) IEvalTargetService {
				return svcmocks.NewMockIEvalTargetService(ctrl)
			},
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				m := svcmocks.NewMockEvaluatorRecordService(ctrl)
				m.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{
					{ID: 1001, EvaluatorVersionID: 10, Status: entity.EvaluatorRunStatusSuccess},
					{ID: 1002, EvaluatorVersionID: 20, Status: entity.EvaluatorRunStatusSuccess},
				}, nil)
				return m
			},
			wantTargetID: 0,
			wantEvalResults: &entity.EvaluatorResults{
				EvalVerIDToResID: map[int64]int64{
					10: 1001,
					20: 1002,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			evalTarget := tt.setupEvalTarget(ctrl)
			evalRecord := tt.setupEvalRecord(ctrl)

			gotTargetID, gotEvalResults := failRetrySelectTurnRunLogRefs(
				context.Background(), tt.spaceID, tt.tr, evalTarget, evalRecord,
			)
			assert.Equal(t, tt.wantTargetID, gotTargetID)
			assert.Equal(t, tt.wantEvalResults, gotEvalResults)
		})
	}
}

func Test_pruneSuccessfulEvaluatorRecords(t *testing.T) {
	tests := []struct {
		name            string
		setupEvalRecord func(ctrl *gomock.Controller) EvaluatorRecordService
		tr              *entity.ExptTurnResult
		want            *entity.EvaluatorResults
	}{
		{
			name: "evalRecord is nil -> nil",
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				return nil
			},
			tr: &entity.ExptTurnResult{
				EvaluatorResults: &entity.EvaluatorResults{
					EvalVerIDToResID: map[int64]int64{10: 1001},
				},
			},
			want: nil,
		},
		{
			name: "tr.EvaluatorResults is nil -> nil",
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				return svcmocks.NewMockEvaluatorRecordService(ctrl)
			},
			tr: &entity.ExptTurnResult{
				EvaluatorResults: nil,
			},
			want: nil,
		},
		{
			name: "tr.EvaluatorResults.EvalVerIDToResID is empty -> nil",
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				return svcmocks.NewMockEvaluatorRecordService(ctrl)
			},
			tr: &entity.ExptTurnResult{
				EvaluatorResults: &entity.EvaluatorResults{
					EvalVerIDToResID: map[int64]int64{},
				},
			},
			want: nil,
		},
		{
			name: "BatchGetEvaluatorRecord returns error -> nil",
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				m := svcmocks.NewMockEvaluatorRecordService(ctrl)
				m.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return(nil, errors.New("batch error"))
				return m
			},
			tr: &entity.ExptTurnResult{
				EvaluatorResults: &entity.EvaluatorResults{
					EvalVerIDToResID: map[int64]int64{10: 1001},
				},
			},
			want: nil,
		},
		{
			name: "All records are non-success -> nil",
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				m := svcmocks.NewMockEvaluatorRecordService(ctrl)
				m.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{
					{ID: 1001, EvaluatorVersionID: 10, Status: entity.EvaluatorRunStatusFail},
					{ID: 1002, EvaluatorVersionID: 20, Status: entity.EvaluatorRunStatusAsyncInvoking},
				}, nil)
				return m
			},
			tr: &entity.ExptTurnResult{
				EvaluatorResults: &entity.EvaluatorResults{
					EvalVerIDToResID: map[int64]int64{10: 1001, 20: 1002},
				},
			},
			want: nil,
		},
		{
			name: "Mix of success and failed records -> returns only success ones",
			setupEvalRecord: func(ctrl *gomock.Controller) EvaluatorRecordService {
				m := svcmocks.NewMockEvaluatorRecordService(ctrl)
				m.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{
					{ID: 1001, EvaluatorVersionID: 10, Status: entity.EvaluatorRunStatusSuccess},
					{ID: 1002, EvaluatorVersionID: 20, Status: entity.EvaluatorRunStatusFail},
					{ID: 1003, EvaluatorVersionID: 30, Status: entity.EvaluatorRunStatusSuccess},
				}, nil)
				return m
			},
			tr: &entity.ExptTurnResult{
				EvaluatorResults: &entity.EvaluatorResults{
					EvalVerIDToResID: map[int64]int64{10: 1001, 20: 1002, 30: 1003},
				},
			},
			want: &entity.EvaluatorResults{
				EvalVerIDToResID: map[int64]int64{
					10: 1001,
					30: 1003,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			evalRecord := tt.setupEvalRecord(ctrl)
			got := pruneSuccessfulEvaluatorRecords(context.Background(), evalRecord, tt.tr)
			assert.Equal(t, tt.want, got)
		})
	}
}
