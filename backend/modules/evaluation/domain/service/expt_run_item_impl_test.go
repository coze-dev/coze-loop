// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
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
)

func Test_NewExptItemEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
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
				mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil, nil)
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
		event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4}
		err := executor.CompleteItemRun(context.Background(), event, nil)
		assert.NoError(t, err)
	})

	t.Run("UpdateItemRunLog返回错误", func(t *testing.T) {
		mockItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("mock updateitemrunlog error"))
		event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4}
		err := executor.CompleteItemRun(context.Background(), event, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mock updateitemrunlog error")
	})
}

func Test_ExptItemEvalCtxExecutor_storeTurnRunResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
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
		result := &entity.ExptTurnRunResult{
			TargetResult:     &entity.EvalTargetRecord{ID: 10},
			EvaluatorResults: map[int64]*entity.EvaluatorRecord{1: {ID: 100, EvaluatorVersionID: 1}},
		}
		mockTurnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil)
		err := executor.storeTurnRunResult(context.Background(), etec, result)
		assert.NoError(t, err)
	})
}

func Test_buildExptTurnEvalCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTurnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	mockItemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	mockConfiger := configermocks.NewMockIConfiger(ctrl)
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
		mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("mock batchget error"))
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
		mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{ID: 100, EvaluatorVersionID: 1}}, nil)
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
