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
				mockPublisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockManager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockExptItemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptItemResultRunLog{}, nil)
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
			got, err := NewRecordEvalMode(tt.event, mockExptItemResultRepo, mockExptTurnResultRepo, mockExptStatsRepo, mockExperimentRepo, mockMetric, mockResultSvc, mockIdgen)
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
