// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configermocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type stubItemCompletePublisher struct {
	events []*component.ItemCompleteEvent
	err    error
}

func (s *stubItemCompletePublisher) PublishItemComplete(_ context.Context, event *component.ItemCompleteEvent) error {
	s.events = append(s.events, event)
	return s.err
}

func Test_NewExptItemEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
	mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)

	tests := []struct {
		name                   string
		turnResultRepo         repo.IExptTurnResultRepo
		itemResultRepo         repo.IExptItemResultRepo
		configer               component.IConfiger
		metric                 metrics.ExptMetric
		evalTargetService      IEvalTargetService
		evaluatorRecordService EvaluatorRecordService
		evaluatorService       EvaluatorService
		benefitService         benefit.IBenefitService
		evalAsyncRepo          repo.IEvalAsyncRepo
		evalSetItemSvc         EvaluationSetItemService
	}{
		{
			name:                   "所有参数有效",
			turnResultRepo:         mockTurnResultRepo,
			itemResultRepo:         mockItemResultRepo,
			configer:               mockConfiger,
			metric:                 mockMetric,
			evalTargetService:      mockEvalTargetService,
			evaluatorRecordService: mockEvaluatorRecordService,
			evaluatorService:       mockEvaluatorService,
			benefitService:         mockBenefitService,
			evalAsyncRepo:          mockEvalAsyncRepo,
			evalSetItemSvc:         servicemocks.NewMockEvaluationSetItemService(ctrl),
		},
		{
			name:                   "部分参数为nil",
			turnResultRepo:         nil,
			itemResultRepo:         mockItemResultRepo,
			configer:               mockConfiger,
			metric:                 mockMetric,
			evalTargetService:      mockEvalTargetService,
			evaluatorRecordService: mockEvaluatorRecordService,
			evaluatorService:       mockEvaluatorService,
			benefitService:         mockBenefitService,
			evalAsyncRepo:          mockEvalAsyncRepo,
			evalSetItemSvc:         servicemocks.NewMockEvaluationSetItemService(ctrl),
		},
		{
			name:                   "全部为nil",
			turnResultRepo:         nil,
			itemResultRepo:         nil,
			configer:               nil,
			metric:                 nil,
			evalTargetService:      nil,
			evaluatorRecordService: nil,
			evaluatorService:       nil,
			benefitService:         nil,
			evalAsyncRepo:          nil,
			evalSetItemSvc:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := NewExptItemEvaluation(
				tt.turnResultRepo,
				tt.itemResultRepo,
				tt.configer,
				tt.metric,
				tt.evalTargetService,
				tt.evaluatorRecordService,
				tt.evaluatorService,
				tt.benefitService,
				tt.evalAsyncRepo,
				tt.evalSetItemSvc,
				nil,
				nil,
			)
			assert.NotNil(t, inst)
		})
	}
}

func Test_ExptItemEvalCtxExecutor_Eval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	type fields struct {
		turnResultRepo         repo.IExptTurnResultRepo
		itemResultRepo         repo.IExptItemResultRepo
		configer               component.IConfiger
		metric                 metrics.ExptMetric
		evalTargetService      IEvalTargetService
		evaluatorRecordService EvaluatorRecordService
		evaluatorService       EvaluatorService
		benefitService         benefit.IBenefitService
	}

	type args struct {
		execCtx *entity.ExptItemEvalCtx
	}

	tests := []struct {
		name       string
		fields     fields
		args       args
		mockSetup  func()
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "参数校验失败 - EvalSetItem为nil",
			fields: fields{
				turnResultRepo:         mockTurnResultRepo,
				itemResultRepo:         mockItemResultRepo,
				configer:               mockConfiger,
				metric:                 mockMetric,
				evalTargetService:      mockEvalTargetService,
				evaluatorRecordService: mockEvaluatorRecordService,
				evaluatorService:       mockEvaluatorService,
				benefitService:         mockBenefitService,
			},
			args: args{
				execCtx: &entity.ExptItemEvalCtx{
					Event:       &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, ExptRunMode: 1, EvalSetItemID: 4, CreateAt: 123456, RetryTimes: 0, Ext: map[string]string{"k": "v"}},
					EvalSetItem: nil,
				},
			},
			mockSetup: func() {
				mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&entity.RetryConf{IsInDebt: false, RetryTimes: 1, RetryIntervalSecond: 1})
			},
			wantErr:    true,
			wantErrMsg: "invalid empty eval_set_item",
		},
		{
			name: "正常流程",
			fields: fields{
				turnResultRepo:         mockTurnResultRepo,
				itemResultRepo:         mockItemResultRepo,
				configer:               mockConfiger,
				metric:                 mockMetric,
				evalTargetService:      mockEvalTargetService,
				evaluatorRecordService: mockEvaluatorRecordService,
				evaluatorService:       mockEvaluatorService,
				benefitService:         mockBenefitService,
			},
			args: args{
				execCtx: &entity.ExptItemEvalCtx{
					Event:       &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, ExptRunMode: 1, EvalSetItemID: 4, CreateAt: 123456, RetryTimes: 0, Ext: map[string]string{"k": "v"}},
					EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{}},
				},
			},
			mockSetup: func() {
				mockItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&entity.RetryConf{IsInDebt: false, RetryTimes: 1, RetryIntervalSecond: 1})
				mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil, nil)
				mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil, nil)
			},
			wantErr: false,
		},
		{
			name: "CompleteSetItemRun返回错误-UpdateItemRunLog error",
			fields: fields{
				turnResultRepo:         mockTurnResultRepo,
				itemResultRepo:         mockItemResultRepo,
				configer:               mockConfiger,
				metric:                 mockMetric,
				evalTargetService:      mockEvalTargetService,
				evaluatorRecordService: mockEvaluatorRecordService,
				evaluatorService:       mockEvaluatorService,
				benefitService:         mockBenefitService,
			},
			args: args{
				execCtx: &entity.ExptItemEvalCtx{
					Event:       &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, ExptRunMode: 1, EvalSetItemID: 4, CreateAt: 123456, RetryTimes: 0, Ext: map[string]string{"k": "v"}},
					EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{}},
				},
			},
			mockSetup: func() {
				mockItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("mock updateitemrunlog error"))
				mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&entity.RetryConf{IsInDebt: false, RetryTimes: 1, RetryIntervalSecond: 1})
			},
			wantErr:    true,
			wantErrMsg: "mock updateitemrunlog error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}
			executor := &ExptItemEvalCtxExecutor{
				TurnResultRepo:         tt.fields.turnResultRepo,
				ItemResultRepo:         tt.fields.itemResultRepo,
				Configer:               tt.fields.configer,
				Metric:                 tt.fields.metric,
				evalTargetService:      tt.fields.evalTargetService,
				evaluatorRecordService: tt.fields.evaluatorRecordService,
				evaluatorService:       tt.fields.evaluatorService,
				benefitService:         tt.fields.benefitService,
			}
			err := executor.Eval(context.Background(), tt.args.execCtx)
			if tt.wantErr {
				assert.Error(t, err)
				fmt.Println(err.Error())
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ExptItemEvalCtxExecutor_EvalTurns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo:         mockTurnResultRepo,
		ItemResultRepo:         mockItemResultRepo,
		Configer:               mockConfiger,
		Metric:                 mockMetric,
		evalTargetService:      mockEvalTargetService,
		evaluatorRecordService: mockEvaluatorRecordService,
		evaluatorService:       mockEvaluatorService,
		benefitService:         mockBenefitService,
	}

	t.Run("参数校验失败-EvalSetItem为nil", func(t *testing.T) {
		execCtx := &entity.ExptItemEvalCtx{EvalSetItem: nil}
		_, err := executor.EvalTurns(context.Background(), execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid empty eval_set_item")
	})

	t.Run("正常流程-无turns", func(t *testing.T) {
		execCtx := &entity.ExptItemEvalCtx{EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{}}}
		_, err := executor.EvalTurns(context.Background(), execCtx)
		assert.NoError(t, err)
	})
}

func Test_ExptItemEvalCtxExecutor_buildExptTurnEvalCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo:         mockTurnResultRepo,
		ItemResultRepo:         mockItemResultRepo,
		Configer:               mockConfiger,
		Metric:                 mockMetric,
		evalTargetService:      mockEvalTargetService,
		evaluatorRecordService: mockEvaluatorRecordService,
		evaluatorService:       mockEvaluatorService,
		benefitService:         mockBenefitService,
	}

	t.Run("无existTurnRunResult", func(t *testing.T) {
		turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
		execCtx := &entity.ExptItemEvalCtx{
			Event:               &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 1, EvalSetItemID: 1},
			EvalSetItem:         &entity.EvaluationSetItem{Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{}, nil)
		etec, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, etec)
	})
}

func Test_ExptItemEvalCtxExecutor_CompleteSetItemRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo:         mockTurnResultRepo,
		ItemResultRepo:         mockItemResultRepo,
		Configer:               mockConfiger,
		Metric:                 mockMetric,
		evalTargetService:      mockEvalTargetService,
		evaluatorRecordService: mockEvaluatorRecordService,
		evaluatorService:       mockEvaluatorService,
		benefitService:         mockBenefitService,
	}

	mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&entity.RetryConf{IsInDebt: false, RetryTimes: 1, RetryIntervalSecond: 1})

	t.Run("正常流程", func(t *testing.T) {
		mockItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		eiec := &entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4}}
		err := executor.CompleteItemRun(context.Background(), eiec, nil)
		assert.NoError(t, err)
	})

	t.Run("UpdateItemRunLog返回错误", func(t *testing.T) {
		mockItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("mock updateitemrunlog error"))
		eiec := &entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4}}
		err := executor.CompleteItemRun(context.Background(), eiec, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mock updateitemrunlog error")
	})

	t.Run("ctx取消后仍落item失败状态", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), int64(4), gomock.Any()).AnyTimes().Return(&entity.RetryConf{IsInDebt: false})
		mockItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), int64(1), int64(2), []int64{3}, gomock.Any(), int64(4)).
			DoAndReturn(func(ctx context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
				require.NoError(t, ctx.Err())
				assert.Equal(t, int32(entity.ItemRunState_Fail), ufields["status"])
				return nil
			})

		eiec := &entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4, RetryTimes: 1}}
		err := executor.CompleteItemRun(ctx, eiec, errors.New("target timeout"))
		assert.NoError(t, err)
	})
}

// item-complete(success) 发送点已从链路A(CompleteItemRun)后移到链路B(scheduler)。
// 本测试固化"链路A 不再发送任何 item-complete"的行为, 无论成功/失败/是否接了 publisher;
// 事件字段组装的正确性由 Test_buildItemCompleteEventFromScheduler 覆盖。
func Test_ExptItemEvalCtxExecutor_CompleteItemRun_NoItemCompletePublish(t *testing.T) {
	const (
		exptID      = int64(1)
		exptRunID   = int64(2)
		itemID      = int64(3)
		spaceID     = int64(4)
		targetID    = int64(5)
		targetSpace = int64(6)
	)

	tests := []struct {
		name    string
		evalErr error
	}{
		{name: "success does not publish"},
		{name: "evaluation error does not publish", evalErr: errors.New("evaluation failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			itemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
			configer := configermocks.NewMockIConfiger(ctrl)
			publisher := &stubItemCompletePublisher{}
			expt := &entity.Experiment{
				CreatedBy:          "creator_user_id",
				ExperimentGroupKey: "group-key",
				TargetID:           targetID,
				Target: &entity.EvalTarget{
					SpaceID:        targetSpace,
					SourceTargetID: "source-target-id",
					EvalTargetVersion: &entity.EvalTargetVersion{
						SandboxAgent: &entity.SandboxAgent{EnableAnalysis: true},
					},
				},
			}

			execCtx := &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{
					ExptID: exptID, ExptRunID: exptRunID, EvalSetItemID: itemID, SpaceID: spaceID,
				},
				Expt: expt,
			}
			wantFields := map[string]any{
				"result_state": entity.ExptItemResultStateLogged,
				"status":       int32(entity.ItemRunState_Success),
			}
			if tt.evalErr != nil {
				wantFields["status"] = int32(entity.ItemRunState_Fail)
				wantFields["err_msg"] = tt.evalErr.Error()
				configer.EXPECT().GetErrRetryConf(gomock.Any(), spaceID, tt.evalErr).
					Times(2).
					Return(&entity.RetryConf{})
			}
			itemResultRepo.EXPECT().UpdateItemRunLog(
				gomock.Any(), exptID, exptRunID, []int64{itemID}, gomock.Any(), spaceID,
			).DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, fields map[string]any, _ int64) error {
				require.Equal(t, wantFields, fields)
				return nil
			})

			executor := &ExptItemEvalCtxExecutor{
				ItemResultRepo:        itemResultRepo,
				Configer:              configer,
				itemCompletePublisher: publisher,
			}
			require.NoError(t, executor.CompleteItemRun(context.Background(), execCtx, tt.evalErr))

			// 链路A 已不再发送 item-complete, publisher 必为空。
			require.Empty(t, publisher.events)
		})
	}
}

// Test_buildItemCompleteEventFromScheduler 覆盖链路B 组装函数的字段保真(承接原链路A 发送测试的字段断言)。
func Test_buildItemCompleteEventFromScheduler(t *testing.T) {
	const (
		spaceID     = int64(1)
		exptID      = int64(2)
		exptRunID   = int64(3)
		itemID      = int64(4)
		targetID    = int64(5)
		targetSpace = int64(6)
		datasetID   = int64(7)
		datasetVer  = int64(8)
	)

	tests := []struct {
		name         string
		mutateExpt   func(*entity.Experiment)
		wantAnalysis bool
	}{
		{name: "sandbox agent enable analysis", wantAnalysis: true},
		{name: "nil target", mutateExpt: func(e *entity.Experiment) { e.Target = nil }},
		{name: "nil target version", mutateExpt: func(e *entity.Experiment) { e.Target.EvalTargetVersion = nil }},
		{name: "nil sandbox agent", mutateExpt: func(e *entity.Experiment) { e.Target.EvalTargetVersion.SandboxAgent = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expt := &entity.Experiment{
				CreatedBy:          "creator_user_id",
				ExperimentGroupKey: "group-key",
				TargetID:           targetID,
				EvalSetSourceType:  entity.ExptEvalSetSourceType_SingleSet,
				EvalSet: &entity.EvaluationSet{
					ID:                   datasetID,
					EvaluationSetVersion: &entity.EvaluationSetVersion{ID: datasetVer, Version: "0.0.1"},
				},
				Target: &entity.EvalTarget{
					SpaceID:        targetSpace,
					SourceTargetID: "source-target-id",
					EvalTargetVersion: &entity.EvalTargetVersion{
						SandboxAgent: &entity.SandboxAgent{EnableAnalysis: true},
					},
				},
			}
			if tt.mutateExpt != nil {
				tt.mutateExpt(expt)
			}
			item := &entity.ExptEvalItem{ExptID: exptID, ItemID: itemID, State: entity.ItemRunState_Success}
			evalSetItem := &entity.EvaluationSetItem{
				ItemID:          itemID,
				ItemKey:         "item-key",
				SpaceID:         spaceID,
				EvaluationSetID: datasetID,
			}

			ev := buildItemCompleteEventFromScheduler(spaceID, exptID, exptRunID, expt, item, evalSetItem, datasetVer)

			require.Equal(t, "2", ev.ExptID)
			require.Equal(t, "3", ev.ExptRunID)
			require.Equal(t, "4", ev.ItemID)
			require.Equal(t, "creator_user_id", ev.CreatedBy)
			require.Equal(t, "group-key", ev.ExperimentGroupKey)
			require.Equal(t, "item-key", ev.ItemKey)
			require.Equal(t, "7", ev.DatasetID)
			require.Equal(t, "8", ev.DatasetVersionID)
			require.Equal(t, tt.wantAnalysis, ev.EnableAnalysis)
			if tt.wantAnalysis {
				require.Equal(t, "6", ev.EvalTargetWorkspaceID)
				require.Equal(t, "source-target-id", ev.SourceTargetID)
			}
		})
	}
}

func Test_ExptItemEvalCtxExecutor_storeTurnRunResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo:         mockTurnResultRepo,
		ItemResultRepo:         mockItemResultRepo,
		Configer:               mockConfiger,
		Metric:                 mockMetric,
		evalTargetService:      mockEvalTargetService,
		evaluatorRecordService: mockEvaluatorRecordService,
		evaluatorService:       mockEvaluatorService,
		benefitService:         mockBenefitService,
	}

	t.Run("result为nil", func(t *testing.T) {
		etec := &entity.ExptTurnEvalCtx{Turn: &entity.Turn{ID: 1}}
		err := executor.storeTurnRunResult(context.Background(), etec, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil result")
	})

	t.Run("turnResultLog为nil", func(t *testing.T) {
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt:                &entity.Experiment{},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}},
			},
		}
		result := &entity.ExptTurnRunResult{}
		err := executor.storeTurnRunResult(context.Background(), etec, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid turn result log")
	})

	t.Run("正常流程", func(t *testing.T) {
		turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1, ErrMsg: "old error"}
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt:                &entity.Experiment{ID: 1, SourceID: "src", SpaceID: 2},
				Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
			},
		}
		result := &entity.ExptTurnRunResult{
			TargetResult:     &entity.EvalTargetRecord{ID: 10},
			EvaluatorResults: []*entity.EvaluatorRecord{{ID: 100, EvaluatorVersionID: 1}},
		}
		mockTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			assert.Equal(t, entity.TurnRunState_Success, logs[0].Status)
			assert.Empty(t, logs[0].ErrMsg)
			return nil
		})
		err := executor.storeTurnRunResult(context.Background(), etec, result)
		assert.NoError(t, err)
	})

	t.Run("缺少评估器结果时落失败状态", func(t *testing.T) {
		turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt: &entity.Experiment{
					ID:      1,
					SpaceID: 2,
					Evaluators: []*entity.Evaluator{
						{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 1}},
						{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 2}},
					},
					EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{EvaluatorsConf: &entity.EvaluatorsConf{}}},
				},
				Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
			},
		}
		result := &entity.ExptTurnRunResult{
			TargetResult:     &entity.EvalTargetRecord{ID: 10},
			EvaluatorResults: []*entity.EvaluatorRecord{{ID: 100, EvaluatorVersionID: 1}},
		}
		mockConfiger.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl())
		mockTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			assert.Equal(t, entity.TurnRunState_Fail, logs[0].Status)
			assert.Contains(t, logs[0].ErrMsg, "evaluator result missing")
			isEvaluatorFailure, _ := errno.ParseEvaluatorResultErr(errno.DeserializeErr([]byte(logs[0].ErrMsg)))
			assert.True(t, isEvaluatorFailure)
			return nil
		})

		err := executor.storeTurnRunResult(context.Background(), etec, result)
		assert.NoError(t, err)
		assert.Error(t, result.GetEvalErr())
	})

	t.Run("target成功后评估器调用错误沿用TurnOther错误码", func(t *testing.T) {
		turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
		var savedRunLog *entity.ExptTurnResultRunLog
		targetStatus := entity.EvalTargetRunStatusSuccess
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt:                &entity.Experiment{ID: 1, SourceID: "src", SpaceID: 2},
				Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
			},
		}
		result := &entity.ExptTurnRunResult{
			TargetResult: &entity.EvalTargetRecord{ID: 10, Status: &targetStatus},
			EvalErr:      errors.New("evaluator call failed"),
		}

		mockConfiger.EXPECT().GetErrCtrl(gomock.Any()).Return(&entity.ExptErrCtrl{ResultErrConverts: []*entity.ResultErrConvert{{
			MatchedText: "evaluator call failed", ToErrMsg: "evaluator temporarily unavailable",
		}}})
		mockTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			assert.Equal(t, int64(10), logs[0].TargetResultID)
			assert.Equal(t, entity.TurnRunState_Fail, logs[0].Status)
			isTurnOther, errMsg := errno.ParseTurnOtherErr(errno.DeserializeErr([]byte(logs[0].ErrMsg)))
			assert.True(t, isTurnOther)
			assert.Equal(t, "evaluator temporarily unavailable", errMsg)
			savedRunLog = logs[0]
			return nil
		})

		err := executor.storeTurnRunResult(context.Background(), etec, result)
		assert.NoError(t, err)
		require.NotNil(t, savedRunLog)
		isTurnOther, resultErrMsg := errno.ParseTurnOtherErr(result.GetEvalErr())
		assert.True(t, isTurnOther)
		assert.Equal(t, "evaluator temporarily unavailable", resultErrMsg)
		builder := &ExptResultBuilder{
			ItemIDTurnID2TurnResultID: map[int64]map[int64]int64{2: {1: 1}},
			turnResultDO: []*entity.ExptTurnResult{{
				ID: 1, ExptRunID: savedRunLog.ExptRunID, ItemID: 2, TurnID: 1,
				Status: int32(savedRunLog.Status), LogID: savedRunLog.LogID, ErrMsg: savedRunLog.ErrMsg,
			}},
		}
		systemInfo := builder.getTurnSystemInfo(context.Background(), 2, 1)
		require.NotNil(t, systemInfo.Error)
		require.NotNil(t, systemInfo.Error.Detail)
		assert.Equal(t, "evaluator temporarily unavailable", *systemInfo.Error.Detail)
	})

	t.Run("target成功后自定义评估器错误沿用TurnOther错误码并保留原始文案", func(t *testing.T) {
		turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
		targetStatus := entity.EvalTargetRunStatusSuccess
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt:                &entity.Experiment{ID: 1, SourceID: "src", SpaceID: 2},
				Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
			},
		}
		result := &entity.ExptTurnRunResult{
			TargetResult: &entity.EvalTargetRecord{ID: 10, Status: &targetStatus},
			EvalErr: errorx.NewByCode(
				errno.CustomRPCEvaluatorRunFailedCode,
				errorx.WithExtraMsg("custom rpc evaluator failed"),
			),
		}
		mockTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			isTurnOther, errMsg := errno.ParseTurnOtherErr(errno.DeserializeErr([]byte(logs[0].ErrMsg)))
			assert.True(t, isTurnOther)
			assert.Contains(t, errMsg, "custom rpc evaluator failed")
			return nil
		})

		err := executor.storeTurnRunResult(context.Background(), etec, result)
		assert.NoError(t, err)
		isTurnOther, resultErrMsg := errno.ParseTurnOtherErr(result.GetEvalErr())
		assert.True(t, isTurnOther)
		assert.Contains(t, resultErrMsg, "custom rpc evaluator failed")
	})

	t.Run("已有失败评估器记录时保留原始错误优先级", func(t *testing.T) {
		localTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
		localConfiger := configermocks.NewMockIConfiger(ctrl)
		localExecutor := *executor
		localExecutor.TurnResultRepo = localTurnResultRepo
		localExecutor.Configer = localConfiger

		turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
		targetStatus := entity.EvalTargetRunStatusSuccess
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt:                &entity.Experiment{ID: 1, SourceID: "src", SpaceID: 2},
				Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
			},
		}
		result := &entity.ExptTurnRunResult{
			TargetResult: &entity.EvalTargetRecord{ID: 10, Status: &targetStatus},
			EvaluatorResults: []*entity.EvaluatorRecord{{
				ID: 20, EvaluatorVersionID: 30,
				EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorRunError: &entity.EvaluatorRunError{
					Code: 40, Message: "evaluator record failed",
				}},
			}},
			EvalErr: errorx.NewByCode(
				errno.CustomRPCEvaluatorRunFailedCode,
				errorx.WithExtraMsg("evaluator orchestration failed"),
			),
		}
		localConfiger.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl()).AnyTimes()

		localTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			persistedErr := errno.DeserializeErr([]byte(logs[0].ErrMsg))
			isEvaluatorRecordFailure, _ := errno.ParseEvaluatorResultErr(persistedErr)
			assert.False(t, isEvaluatorRecordFailure)
			isTurnOther, errMsg := errno.ParseTurnOtherErr(persistedErr)
			assert.True(t, isTurnOther)
			assert.Contains(t, errMsg, "evaluator orchestration failed")
			return nil
		})

		err := localExecutor.storeTurnRunResult(context.Background(), etec, result)
		assert.NoError(t, err)
		isTurnOther, errMsg := errno.ParseTurnOtherErr(result.GetEvalErr())
		assert.True(t, isTurnOther)
		assert.Contains(t, errMsg, "evaluator orchestration failed")
	})

	t.Run("target未成功且缺少对象错误详情时沿用TurnOther错误码", func(t *testing.T) {
		turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
		targetStatus := entity.EvalTargetRunStatusFail
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt:                &entity.Experiment{ID: 1, SourceID: "src", SpaceID: 2},
				Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
			},
		}
		result := &entity.ExptTurnRunResult{
			TargetResult: &entity.EvalTargetRecord{ID: 10, Status: &targetStatus},
			EvalErr:      errors.New("target failed without output error"),
		}

		mockConfiger.EXPECT().GetErrCtrl(gomock.Any()).Return(&entity.ExptErrCtrl{ResultErrConverts: []*entity.ResultErrConvert{{
			MatchedText: "target failed", ToErrMsg: "target failed",
		}}})
		mockTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			isTurnOther, _ := errno.ParseTurnOtherErr(errno.DeserializeErr([]byte(logs[0].ErrMsg)))
			assert.True(t, isTurnOther)
			return nil
		})

		err := executor.storeTurnRunResult(context.Background(), etec, result)
		assert.NoError(t, err)
	})

	t.Run("ctx取消后仍落turn失败状态", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
		etec := &entity.ExptTurnEvalCtx{
			Turn: &entity.Turn{ID: 1},
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt:                &entity.Experiment{ID: 1, SourceID: "src", SpaceID: 2},
				Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
			},
		}
		result := &entity.ExptTurnRunResult{EvalErr: errors.New("target timeout")}

		mockConfiger.EXPECT().GetErrCtrl(gomock.Any()).DoAndReturn(func(ctx context.Context) *entity.ExptErrCtrl {
			require.NoError(t, ctx.Err())
			return entity.DefaultExptErrCtrl()
		})
		mockTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.NoError(t, ctx.Err())
			require.Len(t, logs, 1)
			assert.Equal(t, entity.TurnRunState_Fail, logs[0].Status)
			return nil
		})

		err := executor.storeTurnRunResult(ctx, etec, result)
		assert.NoError(t, err)
	})
}

func Test_buildExptTurnEvalCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo:         mockTurnResultRepo,
		ItemResultRepo:         mockItemResultRepo,
		Configer:               mockConfiger,
		Metric:                 mockMetric,
		evalTargetService:      mockEvalTargetService,
		evaluatorRecordService: mockEvaluatorRecordService,
		evaluatorService:       mockEvaluatorService,
		benefitService:         mockBenefitService,
	}

	t.Run("GetRecordByID返回错误", func(t *testing.T) {
		turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
		execCtx := &entity.ExptItemEvalCtx{
			Event:               &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 1, EvalSetItemID: 1},
			EvalSetItem:         &entity.EvaluationSetItem{Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: {TargetResultID: 123, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 100}}}}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{}, nil)
		mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock get record error"))
		_, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mock get record error")
	})

	t.Run("BatchGetEvaluatorRecord返回错误", func(t *testing.T) {
		turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
		execCtx := &entity.ExptItemEvalCtx{
			Event:               &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 1, EvalSetItemID: 1},
			EvalSetItem:         &entity.EvaluationSetItem{Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: {TargetResultID: 123, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 100}}}}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{}, nil)
		mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvalTargetRecord{ID: 123}, nil)
		mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock batchget error"))
		_, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mock batchget error")
	})

	t.Run("BatchGetEvaluatorRecord返回正常", func(t *testing.T) {
		turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
		execCtx := &entity.ExptItemEvalCtx{
			Event:               &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 1, EvalSetItemID: 1},
			EvalSetItem:         &entity.EvaluationSetItem{Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: {TargetResultID: 123, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 100}}}}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{}, nil)
		mockEvalTargetService.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvalTargetRecord{ID: 123}, nil)
		mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{ID: 100, EvaluatorVersionID: 1}}, nil)
		etec, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, etec)
		assert.NotNil(t, etec.ExptTurnRunResult.EvaluatorResults)
	})

	t.Run("Ext字段处理_从Event.Ext和ItemResult.Ext合并", func(t *testing.T) {
		turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
		execCtx := &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				SpaceID:       1,
				ExptID:        1,
				EvalSetItemID: 1,
				Ext: map[string]string{
					"event_key1": "event_value1",
					"event_key2": "event_value2",
				},
			},
			EvalSetItem: &entity.EvaluationSetItem{
				Turns:    []*entity.Turn{turn},
				BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))},
			},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		itemResult := &entity.ExptItemResult{
			ID:     1,
			ItemID: 1,
			Ext: map[string]string{
				"item_key1":  "item_value1",
				"event_key2": "item_value2_override",
			},
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{itemResult}, nil)
		etec, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, etec)
		assert.NotNil(t, etec.Ext)
		assert.Equal(t, "event_value1", etec.Ext["event_key1"])
		assert.Equal(t, "item_value2_override", etec.Ext["event_key2"])
		assert.Equal(t, "item_value1", etec.Ext["item_key1"])
		assert.Equal(t, "taskid", etec.Ext["task_id"])
		assert.Equal(t, "1", etec.Ext["workspace_id"])
		assert.Equal(t, "1000", etec.Ext["start_time"])
	})

	t.Run("Ext字段处理_从FieldDataList提取span_id_run_id_trace_id", func(t *testing.T) {
		turn := &entity.Turn{
			ID: 1,
			FieldDataList: []*entity.FieldData{
				{Name: "span_id", Content: &entity.Content{Text: gptr.Of("span123")}},
				{Name: "run_id", Content: &entity.Content{Text: gptr.Of("run456")}},
				{Name: "trace_id", Content: &entity.Content{Text: gptr.Of("trace789")}},
			},
		}
		execCtx := &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				SpaceID:       1,
				ExptID:        1,
				EvalSetItemID: 1,
				Ext:           map[string]string{},
			},
			EvalSetItem: &entity.EvaluationSetItem{
				Turns:    []*entity.Turn{turn},
				BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))},
			},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{}, nil)
		etec, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, etec)
		assert.NotNil(t, etec.Ext)
		assert.Equal(t, "span123", etec.Ext["span_id"])
		assert.Equal(t, "run456", etec.Ext["run_id"])
		assert.Equal(t, "trace789", etec.Ext["trace_id"])
	})

	t.Run("Ext字段处理_ItemResult.Ext为nil", func(t *testing.T) {
		turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
		execCtx := &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				SpaceID:       1,
				ExptID:        1,
				EvalSetItemID: 1,
				Ext: map[string]string{
					"event_key": "event_value",
				},
			},
			EvalSetItem: &entity.EvaluationSetItem{
				Turns:    []*entity.Turn{turn},
				BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))},
			},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		itemResult := &entity.ExptItemResult{
			ID:     1,
			ItemID: 1,
			Ext:    nil,
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{itemResult}, nil)
		etec, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, etec)
		assert.NotNil(t, etec.Ext)
		assert.Equal(t, "event_value", etec.Ext["event_key"])
	})

	t.Run("Ext字段处理_BatchGet返回错误", func(t *testing.T) {
		turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
		execCtx := &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				SpaceID:       1,
				ExptID:        1,
				EvalSetItemID: 1,
				Ext: map[string]string{
					"event_key": "event_value",
				},
			},
			EvalSetItem: &entity.EvaluationSetItem{
				Turns:    []*entity.Turn{turn},
				BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))},
			},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}},
			Expt:                &entity.Experiment{SourceID: "taskid", SpaceID: 1},
		}
		mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return(nil, errors.New("batch get error"))
		etec, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, etec)
		assert.NotNil(t, etec.Ext)
		assert.Equal(t, "event_value", etec.Ext["event_key"])
	})
}

func Test_buildHistoryMessage(t *testing.T) {
	assert.Nil(t, buildHistoryMessage(context.Background(), nil))
}

func Test_buildExptTurnEvalCtx_BuildEvalExtMerge(t *testing.T) {
	tests := []struct {
		name      string
		buildExt  map[string]string
		wantKey   string
		wantValue string
	}{
		{
			name:      "build eval ext merged into etec ext",
			buildExt:  map[string]string{"build_key": "build_value"},
			wantKey:   "build_key",
			wantValue: "build_value",
		},
		{
			name:      "build eval ext overrides existing key",
			buildExt:  map[string]string{"task_id": "override_task_id"},
			wantKey:   "task_id",
			wantValue: "override_task_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
			mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
			mockConfiger := configermocks.NewMockIConfiger(ctrl)
			mockConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(tt.buildExt)
			mockMetric := metricsmocks.NewMockExptMetric(ctrl)
			mockEvalTargetService := servicemocks.NewMockIEvalTargetService(ctrl)
			mockEvaluatorRecordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
			mockEvaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
			mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

			executor := &ExptItemEvalCtxExecutor{
				TurnResultRepo:         mockTurnResultRepo,
				ItemResultRepo:         mockItemResultRepo,
				Configer:               mockConfiger,
				Metric:                 mockMetric,
				evalTargetService:      mockEvalTargetService,
				evaluatorRecordService: mockEvaluatorRecordService,
				evaluatorService:       mockEvaluatorService,
				benefitService:         mockBenefitService,
			}

			turn := &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{}}
			execCtx := &entity.ExptItemEvalCtx{
				Event:       &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 1, EvalSetItemID: 1},
				EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}},
				ExistItemEvalResult: &entity.ExptItemEvalResult{
					TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{},
				},
				Expt: &entity.Experiment{SourceID: "taskid", SpaceID: 1},
			}
			mockItemResultRepo.EXPECT().BatchGet(gomock.Any(), int64(1), int64(1), []int64{1}).Return([]*entity.ExptItemResult{}, nil)

			etec, err := executor.buildExptTurnEvalCtx(context.Background(), turn, execCtx, nil)
			assert.NoError(t, err)
			assert.NotNil(t, etec)
			assert.Equal(t, tt.wantValue, etec.Ext[tt.wantKey])
		})
	}
}

func Test_buildItemCompleteEvent(t *testing.T) {
	tests := []struct {
		name               string
		eiec               *entity.ExptItemEvalCtx
		wantCreatedBy      string
		wantEnableAnalysis bool
	}{
		{
			name: "sandbox agent analysis enabled -> created_by + enable_analysis both set",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_abc",
					TargetID:  9,
					Target: &entity.EvalTarget{
						SpaceID: 1,
						EvalTargetVersion: &entity.EvalTargetVersion{
							SandboxAgent: &entity.SandboxAgent{EnableAnalysis: true},
						},
					},
				},
			},
			wantCreatedBy:      "user_abc",
			wantEnableAnalysis: true,
		},
		{
			name: "sandbox agent analysis disabled -> created_by set, enable_analysis false",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_def",
					Target: &entity.EvalTarget{
						SpaceID:           1,
						EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{EnableAnalysis: false}},
					},
				},
			},
			wantCreatedBy:      "user_def",
			wantEnableAnalysis: false,
		},
		{
			name: "nil sandbox agent -> enable_analysis false, no panic",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_ghi",
					Target:    &entity.EvalTarget{SpaceID: 1, EvalTargetVersion: &entity.EvalTargetVersion{}},
				},
			},
			wantCreatedBy:      "user_ghi",
			wantEnableAnalysis: false,
		},
		{
			name: "nil target version -> enable_analysis false, no panic",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt:  &entity.Experiment{CreatedBy: "user_jkl", Target: &entity.EvalTarget{SpaceID: 1}},
			},
			wantCreatedBy:      "user_jkl",
			wantEnableAnalysis: false,
		},
		{
			name: "nil target -> enable_analysis false, created_by still set, no panic",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt:  &entity.Experiment{CreatedBy: "user_mno"},
			},
			wantCreatedBy:      "user_mno",
			wantEnableAnalysis: false,
		},
		{
			name: "custom rpc server analysis enabled -> enable_analysis true",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_crpc",
					Target: &entity.EvalTarget{
						SpaceID:           1,
						EvalTargetVersion: &entity.EvalTargetVersion{CustomRPCServer: &entity.CustomRPCServer{EnableAnalysis: true}},
					},
				},
			},
			wantCreatedBy:      "user_crpc",
			wantEnableAnalysis: true,
		},
		{
			name: "custom agent analysis enabled -> enable_analysis true",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_cagent",
					Target: &entity.EvalTarget{
						SpaceID:           1,
						EvalTargetVersion: &entity.EvalTargetVersion{CustomAgent: &entity.CustomAgent{EnableAnalysis: true}},
					},
				},
			},
			wantCreatedBy:      "user_cagent",
			wantEnableAnalysis: true,
		},
		{
			name: "volcengine agent analysis enabled -> enable_analysis true",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_volc",
					Target: &entity.EvalTarget{
						SpaceID:           1,
						EvalTargetVersion: &entity.EvalTargetVersion{VolcengineAgent: &entity.VolcengineAgent{EnableAnalysis: true}},
					},
				},
			},
			wantCreatedBy:      "user_volc",
			wantEnableAnalysis: true,
		},
		{
			name: "web agent analysis enabled -> enable_analysis true",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_web",
					Target: &entity.EvalTarget{
						SpaceID:           1,
						EvalTargetVersion: &entity.EvalTargetVersion{WebAgent: &entity.WebAgent{EnableAnalysis: true}},
					},
				},
			},
			wantCreatedBy:      "user_web",
			wantEnableAnalysis: true,
		},
		{
			name: "a2a agent analysis enabled -> enable_analysis true",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_a2a",
					Target: &entity.EvalTarget{
						SpaceID:           1,
						EvalTargetVersion: &entity.EvalTargetVersion{A2AAgent: &entity.A2AAgent{EnableAnalysis: true}},
					},
				},
			},
			wantCreatedBy:      "user_a2a",
			wantEnableAnalysis: true,
		},
		{
			name: "prompt target -> enable_analysis false (not supported)",
			eiec: &entity.ExptItemEvalCtx{
				Event: &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
				Expt: &entity.Experiment{
					CreatedBy: "user_prompt",
					Target: &entity.EvalTarget{
						SpaceID:           1,
						EvalTargetVersion: &entity.EvalTargetVersion{Prompt: &entity.LoopPrompt{}},
					},
				},
			},
			wantCreatedBy:      "user_prompt",
			wantEnableAnalysis: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := buildItemCompleteEvent(tt.eiec)
			assert.NotNil(t, ev)
			assert.Equal(t, tt.wantCreatedBy, ev.CreatedBy)
			assert.Equal(t, tt.wantEnableAnalysis, ev.EnableAnalysis)
			// 基础字段恒填充
			assert.Equal(t, "100", ev.ExptID)
			assert.Equal(t, "300", ev.ItemID)
		})
	}
}

// Test_buildItemCompleteEvent_LinkAB_Equivalence 钉死"发送内容与原链路A一致"契约:
// 同一份输入, 原组装 buildItemCompleteEvent(链路A) 与 buildItemCompleteEventFromScheduler(链路B拆参)
// 必须产出完全相等的 ItemCompleteEvent。任一函数改动导致漂移, 此测试立即失败。
func Test_buildItemCompleteEvent_LinkAB_Equivalence(t *testing.T) {
	const (
		spaceID   = int64(1)
		exptID    = int64(100)
		exptRunID = int64(200)
		itemID    = int64(300)
		datasetID = int64(700)
		datasetV  = int64(800)
	)
	expt := &entity.Experiment{
		TargetID:           int64(9),
		ExperimentGroupKey: "group-key",
		CreatedBy:          "creator",
		Target: &entity.EvalTarget{
			SpaceID:        int64(6),
			SourceTargetID: "source-target-id",
			EvalTargetVersion: &entity.EvalTargetVersion{
				SandboxAgent: &entity.SandboxAgent{EnableAnalysis: true},
			},
		},
		EvalSet: &entity.EvaluationSet{
			ID:                   datasetID,
			DatasetKey:           "ds-key",
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: datasetV, Version: "0.0.1"},
		},
	}
	evalSetItem := &entity.EvaluationSetItem{
		SpaceID:         spaceID,
		EvaluationSetID: datasetID,
		ItemKey:         "item-key",
	}

	// 链路A: 原组装函数直接吃 eiec
	eiec := &entity.ExptItemEvalCtx{
		Event:            &entity.ExptItemEvalEvent{SpaceID: spaceID, ExptID: exptID, ExptRunID: exptRunID, EvalSetItemID: itemID},
		Expt:             expt,
		EvalSetItem:      evalSetItem,
		EvalSetVersionID: datasetV,
	}
	fromLinkA := buildItemCompleteEvent(eiec)

	// 链路B: scheduler 拆参入口
	item := &entity.ExptEvalItem{ExptID: exptID, ItemID: itemID, State: entity.ItemRunState_Success}
	fromLinkB := buildItemCompleteEventFromScheduler(spaceID, exptID, exptRunID, expt, item, evalSetItem, datasetV)

	require.Equal(t, fromLinkA, fromLinkB, "链路A与链路B的 item-complete 组装结果必须完全一致")
}

func TestExptItemEvalCtxExecutor_storeTurnRunResult_ArmsAsyncEvaluatorAfterSave(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil)
	evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(100)).Return(nil)

	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, evaluatorService: evaluatorSvc}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:        &entity.Experiment{ID: 1, SpaceID: 2},
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 4},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				1: {ID: 5, TurnID: 1},
			}},
		},
	}
	result := &entity.ExptTurnRunResult{EvaluatorResults: []*entity.EvaluatorRecord{{
		ID: 100, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking,
	}}}
	require.NoError(t, executor.storeTurnRunResult(context.Background(), etec, result))
}

func TestExptItemEvalCtxExecutor_storeTurnRunResult_ArmingHasIndependentTimeoutBudget(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)

	var persistDeadline time.Time
	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ []*entity.ExptTurnResultRunLog) error {
			var ok bool
			persistDeadline, ok = ctx.Deadline()
			require.True(t, ok)
			time.Sleep(20 * time.Millisecond)
			return nil
		},
	)
	evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(100)).DoAndReturn(
		func(ctx context.Context, _ int64) error {
			resumeDeadline, ok := ctx.Deadline()
			require.True(t, ok)
			assert.True(t, resumeDeadline.After(persistDeadline), "arming must not consume the turn-log persistence timeout budget")
			return nil
		},
	)

	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, evaluatorService: evaluatorSvc}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:        &entity.Experiment{ID: 1, SpaceID: 2},
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 4},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				1: {ID: 5, TurnID: 1},
			}},
		},
	}
	result := &entity.ExptTurnRunResult{EvaluatorResults: []*entity.EvaluatorRecord{{
		ID: 100, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking,
	}}}
	require.NoError(t, executor.storeTurnRunResult(context.Background(), etec, result))
}

func TestExptItemEvalCtxExecutor_storeTurnRunResult_ArmsPendingEvaluatorsConcurrently(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil)

	secondStarted := make(chan struct{})
	firstObservedSecond := make(chan bool, 1)
	evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(100)).DoAndReturn(
		func(context.Context, int64) error {
			select {
			case <-secondStarted:
				firstObservedSecond <- true
			case <-time.After(time.Second):
				firstObservedSecond <- false
			}
			return nil
		},
	)
	evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(200)).DoAndReturn(
		func(context.Context, int64) error {
			close(secondStarted)
			return nil
		},
	)

	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, evaluatorService: evaluatorSvc}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:        &entity.Experiment{ID: 1, SpaceID: 2},
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 4},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				1: {ID: 5, TurnID: 1},
			}},
		},
	}
	result := &entity.ExptTurnRunResult{EvaluatorResults: []*entity.EvaluatorRecord{
		{ID: 100, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking},
		{ID: 200, EvaluatorVersionID: 201, Status: entity.EvaluatorRunStatusAsyncInvoking},
	}}

	require.NoError(t, executor.storeTurnRunResult(context.Background(), etec, result))
	require.True(t, <-firstObservedSecond, "one slow arming operation must not delay the other pending evaluators")
}

func TestExptItemEvalCtxExecutor_storeTurnRunResult_DoesNotArmWhenSaveFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(errors.New("save failed"))
	evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), gomock.Any()).Times(0)

	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, evaluatorService: evaluatorSvc}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:        &entity.Experiment{ID: 1, SpaceID: 2},
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 4},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				1: {ID: 5, TurnID: 1},
			}},
		},
	}
	result := &entity.ExptTurnRunResult{EvaluatorResults: []*entity.EvaluatorRecord{{
		ID: 100, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking,
	}}}
	require.Error(t, executor.storeTurnRunResult(context.Background(), etec, result))
}

func TestExptItemEvalCtxExecutor_storeTurnRunResult_ArmErrorKeepsAsyncProcessing(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil)
	evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(100)).Return(errors.New("arm failed"))

	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, evaluatorService: evaluatorSvc}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:        &entity.Experiment{ID: 1, SpaceID: 2},
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 4},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				1: {ID: 5, TurnID: 1},
			}},
		},
	}
	result := &entity.ExptTurnRunResult{AsyncAbort: true, EvaluatorResults: []*entity.EvaluatorRecord{{
		ID: 100, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking,
	}}}
	require.NoError(t, executor.storeTurnRunResult(context.Background(), etec, result))
}

func TestExptItemEvalCtxExecutor_storeTurnRunResult_ArmsEveryPendingEvaluator(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)

	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			require.NotNil(t, logs[0].EvaluatorResultIds)
			require.Len(t, logs[0].EvaluatorResultIds.Registered, 5)
			seen := make(map[int64]int)
			for _, ref := range logs[0].EvaluatorResultIds.Registered {
				require.NotNil(t, ref)
				seen[ref.VersionID]++
			}
			for _, versionID := range []int64{101, 102, 201, 202, 203} {
				assert.Equal(t, 1, seen[versionID])
			}
			return nil
		},
	)
	for _, recordID := range []int64{1201, 1202, 1203} {
		evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), recordID).Return(nil)
	}

	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, evaluatorService: evaluatorSvc}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:        &entity.Experiment{ID: 1, SpaceID: 2},
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 4},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				1: {ID: 5, TurnID: 1},
			}},
		},
	}
	result := &entity.ExptTurnRunResult{
		AsyncAbort: true,
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 1101, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 1102, EvaluatorVersionID: 102, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 1201, EvaluatorVersionID: 201, Status: entity.EvaluatorRunStatusAsyncInvoking},
			{ID: 1202, EvaluatorVersionID: 202, Status: entity.EvaluatorRunStatusAsyncInvoking},
			{ID: 1203, EvaluatorVersionID: 203, Status: entity.EvaluatorRunStatusAsyncInvoking},
		},
	}
	require.NoError(t, executor.storeTurnRunResult(context.Background(), etec, result))
}

func TestExptItemEvalCtxExecutor_storeTurnRunResult_AllEvaluatorsTerminalCompletesWithoutArming(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)

	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			assert.Equal(t, entity.TurnRunState_Success, logs[0].Status)
			require.NotNil(t, logs[0].EvaluatorResultIds)
			require.Len(t, logs[0].EvaluatorResultIds.Registered, 5)
			return nil
		},
	)
	evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), gomock.Any()).Times(0)

	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, evaluatorService: evaluatorSvc}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:        &entity.Experiment{ID: 1, SpaceID: 2},
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 4},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				1: {ID: 5, TurnID: 1, Status: entity.TurnRunState_Processing},
			}},
		},
	}
	result := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 1101, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 1102, EvaluatorVersionID: 102, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 1201, EvaluatorVersionID: 201, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 1202, EvaluatorVersionID: 202, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 1203, EvaluatorVersionID: 203, Status: entity.EvaluatorRunStatusSuccess},
		},
	}
	require.NoError(t, executor.storeTurnRunResult(context.Background(), etec, result))
}
