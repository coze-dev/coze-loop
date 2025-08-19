// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type exptSubmitExecFields struct {
	manager   *svcmocks.MockIExptManager
	idem      *idemmocks.MockIdempotentService
	configer  *configmocks.MockIConfiger
	itemRepo  *mock_repo.MockIExptItemResultRepo
	publisher *eventmocks.MockExptEventPublisher
}

type exptFailRetryExecFields struct {
	manager            *svcmocks.MockIExptManager
	exptItemResultRepo *mock_repo.MockIExptItemResultRepo
	exptTurnResultRepo *mock_repo.MockIExptTurnResultRepo
	exptStatsRepo      *mock_repo.MockIExptStatsRepo
	idgenerator        *idgenmocks.MockIIDGenerator
	exptRepo           *mock_repo.MockIExperimentRepo
	idem               *idemmocks.MockIdempotentService
	configer           *configmocks.MockIConfiger
	publisher          *eventmocks.MockExptEventPublisher
}

func TestExptSubmitExec_Mode(t *testing.T) {
	exec := &ExptSubmitExec{}
	assert.Equal(t, entity.EvaluationModeSubmit, exec.Mode())
}

func TestExptSubmitExec_ScheduleStart(t *testing.T) {
	testCases := []struct {
		name    string
		expt    *entity.Experiment
		event   *entity.ExptScheduleEvent
		wantErr bool
	}{
		{
			name:    "正常流程",
			expt:    &entity.Experiment{},
			event:   &entity.ExptScheduleEvent{},
			wantErr: false,
		},
	}

	exec := &ExptSubmitExec{}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := exec.ScheduleStart(context.Background(), tc.event, tc.expt)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptSubmitExec_ScheduleEnd(t *testing.T) {
	testCases := []struct {
		name       string
		event      *entity.ExptScheduleEvent
		expt       *entity.Experiment
		toSubmit   int
		incomplete int
		wantErr    bool
	}{
		{
			name:       "正常流程",
			event:      &entity.ExptScheduleEvent{},
			expt:       &entity.Experiment{},
			toSubmit:   0,
			incomplete: 0,
			wantErr:    false,
		},
	}

	exec := &ExptSubmitExec{}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := exec.ScheduleEnd(context.Background(), tc.event, tc.expt, tc.toSubmit, tc.incomplete)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptSubmitExec_ExptEnd(t *testing.T) {
	testCases := []struct {
		name       string
		mockSetup  func(f *exptSubmitExecFields)
		event      *entity.ExptScheduleEvent
		expt       *entity.Experiment
		toSubmit   int
		incomplete int
		wantErr    bool
		assertErr  func(t *testing.T, err error)
	}{
		{
			name: "正常流程",
			mockSetup: func(f *exptSubmitExecFields) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
				f.manager.EXPECT().CompleteRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
				f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: "u1"}},
			expt:       &entity.Experiment{},
			toSubmit:   0,
			incomplete: 0,
			wantErr:    false,
			assertErr:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "idem 已存在",
			mockSetup: func(f *exptSubmitExecFields) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil)
			},
			event:     &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: "u1"}},
			expt:      &entity.Experiment{},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "CompleteRun 报错",
			mockSetup: func(f *exptSubmitExecFields) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
				f.manager.EXPECT().CompleteRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("runerr"))
			},
			event:     &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: "u1"}},
			expt:      &entity.Experiment{},
			wantErr:   true,
			assertErr: func(t *testing.T, err error) { assert.ErrorContains(t, err, "runerr") },
		},
		{
			name: "CompleteExpt 报错",
			mockSetup: func(f *exptSubmitExecFields) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
				f.manager.EXPECT().CompleteRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("exptrerr"))
			},
			event:     &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: "u1"}},
			expt:      &entity.Experiment{},
			wantErr:   true,
			assertErr: func(t *testing.T, err error) { assert.ErrorContains(t, err, "exptrerr") },
		},
		{
			name: "idem Exist 报错",
			mockSetup: func(f *exptSubmitExecFields) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, errors.New("idemerr"))
			},
			event:     &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: "u1"}},
			expt:      &entity.Experiment{},
			wantErr:   true,
			assertErr: func(t *testing.T, err error) { assert.ErrorContains(t, err, "idemerr") },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := &exptSubmitExecFields{
				manager:   svcmocks.NewMockIExptManager(ctrl),
				idem:      idemmocks.NewMockIdempotentService(ctrl),
				configer:  configmocks.NewMockIConfiger(ctrl),
				itemRepo:  mock_repo.NewMockIExptItemResultRepo(ctrl),
				publisher: eventmocks.NewMockExptEventPublisher(ctrl),
			}
			if tc.mockSetup != nil {
				tc.mockSetup(f)
			}
			exec := &ExptSubmitExec{
				manager:            f.manager,
				idem:               f.idem,
				configer:           f.configer,
				exptItemResultRepo: f.itemRepo,
			}
			nextTick, err := exec.ExptEnd(context.Background(), tc.event, tc.expt, tc.toSubmit, tc.incomplete)
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			}
			if !tc.wantErr {
				assert.False(t, nextTick)
			}
		})
	}
}

func TestExptSubmitExec_NextTick(t *testing.T) {
	testCases := []struct {
		name      string
		nextTick  bool
		mockSetup func(f *exptSubmitExecFields)
		event     *entity.ExptScheduleEvent
		wantErr   bool
		assertErr func(t *testing.T, err error)
	}{
		{
			name:      "nextTick=false 不触发",
			nextTick:  false,
			mockSetup: nil,
			event:     &entity.ExptScheduleEvent{SpaceID: 1},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name:     "nextTick=true 正常发布",
			nextTick: true,
			mockSetup: func(f *exptSubmitExecFields) {
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(1)).Return(&entity.ExptExecConf{DaemonIntervalSecond: 1})
				f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			event:     &entity.ExptScheduleEvent{SpaceID: 1},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name:     "nextTick=true 发布报错",
			nextTick: true,
			mockSetup: func(f *exptSubmitExecFields) {
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(1)).Return(&entity.ExptExecConf{DaemonIntervalSecond: 1})
				f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("puberr"))
			},
			event:     &entity.ExptScheduleEvent{SpaceID: 1},
			wantErr:   true,
			assertErr: func(t *testing.T, err error) { assert.ErrorContains(t, err, "puberr") },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := &exptSubmitExecFields{
				configer:  configmocks.NewMockIConfiger(ctrl),
				publisher: eventmocks.NewMockExptEventPublisher(ctrl),
			}
			if tc.mockSetup != nil {
				tc.mockSetup(f)
			}
			exec := &ExptSubmitExec{
				configer:  f.configer,
				publisher: f.publisher,
			}
			err := exec.NextTick(context.Background(), tc.event, tc.nextTick)
			if tc.assertErr != nil {
				tc.assertErr(t, err)
			}
		})
	}
}

func TestExptSubmitExec_ExptStart(t *testing.T) {
	testUserID := "test_user_id_123"
	mockExpt := &entity.Experiment{
		ID:                  1,
		SpaceID:             3,
		CreatedBy:           "created_by",
		Name:                "created_by",
		Description:         "description",
		EvalSetVersionID:    1,
		EvalSetID:           1,
		TargetType:          1,
		TargetVersionID:     1,
		TargetID:            1,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 1}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1, IngressConf: &entity.TargetIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}},
			}},
			EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: ptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{
				{
					EvaluatorVersionID: 1,
					IngressConf:        &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}}},
				},
			}},
		}},
		Target: &entity.EvalTarget{ID: 1, SpaceID: 3, SourceTargetID: "source_target_id", EvalTargetType: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1, OutputSchema: []*entity.ArgsSchema{{Key: ptr.Of("key")}}}, BaseInfo: &entity.BaseInfo{}},
		EvalSet: &entity.EvaluationSet{
			ID: 1, SpaceID: 3, Name: "name", Description: "description", Status: 0, Spec: nil, Features: nil, ItemCount: 0, ChangeUncommitted: false,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1, AppID: 0, SpaceID: 3, EvaluationSetID: 1, Version: "version", VersionNum: 0, Description: "description", EvaluationSetSchema: nil, ItemCount: 0, BaseInfo: nil},
			LatestVersion:        "", NextVersionNum: 0, BaseInfo: nil, BizCategory: strconv.Itoa(1),
		},
		Evaluators:      []*entity.Evaluator{{}},
		Status:          0,
		StatusMessage:   "",
		LatestRunID:     0,
		CreditCost:      0,
		StartAt:         nil,
		EndAt:           nil,
		ExptType:        1,
		MaxAliveTime:    0,
		SourceType:      0,
		SourceID:        "",
		Stats:           nil,
		AggregateResult: nil,
	}

	type fields struct {
		manager                  *svcmocks.MockIExptManager
		exptItemResultRepo       *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo       *mock_repo.MockIExptTurnResultRepo
		exptStatsRepo            *mock_repo.MockIExptStatsRepo
		idgenerator              *idgenmocks.MockIIDGenerator
		evaluationSetItemService *svcmocks.MockEvaluationSetItemService
		exptRepo                 *mock_repo.MockIExperimentRepo
		idem                     *idemmocks.MockIdempotentService
		configer                 *configmocks.MockIConfiger
		publisher                *eventmocks.MockExptEventPublisher
		resultSvc                *svcmocks.MockExptResultService
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		expt  *entity.Experiment
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "正常流程-全部成功",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				f.evaluationSetItemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
					{ItemID: 1, Turns: []*entity.Turn{{ID: 1}}},
					{ItemID: 2, Turns: []*entity.Turn{{ID: 2}}},
				}, ptr.Of(int64(2)), nil, nil).Times(1)
				f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1, 2, 3, 4}, nil).Times(1)
				f.exptTurnResultRepo.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.exptItemResultRepo.EXPECT().BatchCreateNX(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{5, 6}, nil).Times(1)
				f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.exptStatsRepo.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1}).Times(1)
				f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "idem已存在",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil).Times(1)
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "idem检查失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, errors.New("idem error")).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "idem error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:                  svcmocks.NewMockIExptManager(ctrl),
				exptItemResultRepo:       mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:       mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:            mock_repo.NewMockIExptStatsRepo(ctrl),
				idgenerator:              idgenmocks.NewMockIIDGenerator(ctrl),
				evaluationSetItemService: svcmocks.NewMockEvaluationSetItemService(ctrl),
				exptRepo:                 mock_repo.NewMockIExperimentRepo(ctrl),
				idem:                     idemmocks.NewMockIdempotentService(ctrl),
				configer:                 configmocks.NewMockIConfiger(ctrl),
				publisher:                eventmocks.NewMockExptEventPublisher(ctrl),
				resultSvc:                svcmocks.NewMockExptResultService(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			e := &ExptSubmitExec{
				manager:                  f.manager,
				exptItemResultRepo:       f.exptItemResultRepo,
				exptTurnResultRepo:       f.exptTurnResultRepo,
				exptStatsRepo:            f.exptStatsRepo,
				idgenerator:              f.idgenerator,
				evaluationSetItemService: f.evaluationSetItemService,
				exptRepo:                 f.exptRepo,
				idem:                     f.idem,
				configer:                 f.configer,
				publisher:                f.publisher,
				resultSvc:                f.resultSvc,
			}

			err := e.ExptStart(tt.args.ctx, tt.args.event, tt.args.expt)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptSubmitExec_ScanEvalItems(t *testing.T) {
	testUserID := "test_user_id_123"
	mockExpt := &entity.Experiment{
		ID:                  1,
		SpaceID:             3,
		CreatedBy:           "created_by",
		Name:                "created_by",
		Description:         "description",
		EvalSetVersionID:    1,
		EvalSetID:           1,
		TargetType:          1,
		TargetVersionID:     1,
		TargetID:            1,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 1}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1, IngressConf: &entity.TargetIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}},
			}},
			EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: ptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{
				{
					EvaluatorVersionID: 1,
					IngressConf:        &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}}},
				},
			}},
		}},
		Target: &entity.EvalTarget{ID: 1, SpaceID: 3, SourceTargetID: "source_target_id", EvalTargetType: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1, OutputSchema: []*entity.ArgsSchema{{Key: ptr.Of("key")}}}, BaseInfo: &entity.BaseInfo{}},
		EvalSet: &entity.EvaluationSet{
			ID: 1, SpaceID: 3, Name: "name", Description: "description", Status: 0, Spec: nil, Features: nil, ItemCount: 0, ChangeUncommitted: false,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1, AppID: 0, SpaceID: 3, EvaluationSetID: 1, Version: "version", VersionNum: 0, Description: "description", EvaluationSetSchema: nil, ItemCount: 0, BaseInfo: nil},
			LatestVersion:        "", NextVersionNum: 0, BaseInfo: nil, BizCategory: strconv.Itoa(1),
		},
		Evaluators:      []*entity.Evaluator{{}},
		Status:          0,
		StatusMessage:   "",
		LatestRunID:     0,
		CreditCost:      0,
		StartAt:         nil,
		EndAt:           nil,
		ExptType:        1,
		MaxAliveTime:    0,
		SourceType:      0,
		SourceID:        "",
		Stats:           nil,
		AggregateResult: nil,
	}

	type fields struct {
		manager                  *svcmocks.MockIExptManager
		exptItemResultRepo       *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo       *mock_repo.MockIExptTurnResultRepo
		exptStatsRepo            *mock_repo.MockIExptStatsRepo
		idgenerator              *idgenmocks.MockIIDGenerator
		evaluationSetItemService *svcmocks.MockEvaluationSetItemService
		exptRepo                 *mock_repo.MockIExperimentRepo
		idem                     *idemmocks.MockIdempotentService
		configer                 *configmocks.MockIConfiger
		publisher                *eventmocks.MockExptEventPublisher
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		expt  *entity.Experiment
	}

	tests := []struct {
		name           string
		prepareMock    func(f *fields, ctrl *gomock.Controller, args args)
		args           args
		wantToSubmit   []*entity.ExptEvalItem
		wantIncomplete []*entity.ExptEvalItem
		wantComplete   []*entity.ExptEvalItem
		wantErr        bool
		assertErr      func(t *testing.T, err error)
	}{
		{
			name: "正常流程-全部成功",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 3}}).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 1, Status: int32(entity.ItemRunState_Processing)},
				}, int64(1), nil).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 2, Status: int32(entity.ItemRunState_Queueing)},
				}, int64(1), nil).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 3, Status: int32(entity.ItemRunState_Success)},
				}, int64(1), nil).Times(1)
			},
			wantToSubmit: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 2, State: entity.ItemRunState_Queueing},
			},
			wantIncomplete: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 1, State: entity.ItemRunState_Processing},
			},
			wantComplete: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 3, State: entity.ItemRunState_Success},
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "扫描失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("scan error")).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "scan error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:                  svcmocks.NewMockIExptManager(ctrl),
				exptItemResultRepo:       mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:       mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:            mock_repo.NewMockIExptStatsRepo(ctrl),
				idgenerator:              idgenmocks.NewMockIIDGenerator(ctrl),
				evaluationSetItemService: svcmocks.NewMockEvaluationSetItemService(ctrl),
				exptRepo:                 mock_repo.NewMockIExperimentRepo(ctrl),
				idem:                     idemmocks.NewMockIdempotentService(ctrl),
				configer:                 configmocks.NewMockIConfiger(ctrl),
				publisher:                eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			e := &ExptSubmitExec{
				manager:                  f.manager,
				exptItemResultRepo:       f.exptItemResultRepo,
				exptTurnResultRepo:       f.exptTurnResultRepo,
				exptStatsRepo:            f.exptStatsRepo,
				idgenerator:              f.idgenerator,
				evaluationSetItemService: f.evaluationSetItemService,
				exptRepo:                 f.exptRepo,
				idem:                     f.idem,
				configer:                 f.configer,
				publisher:                f.publisher,
			}

			toSubmit, incomplete, complete, err := e.ScanEvalItems(tt.args.ctx, tt.args.event, tt.args.expt)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
			if !tt.wantErr {
				assert.Equal(t, tt.wantToSubmit, toSubmit)
				assert.Equal(t, tt.wantIncomplete, incomplete)
				assert.Equal(t, tt.wantComplete, complete)
			}
		})
	}
}

func TestExptFailRetryExec_Mode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	f := &exptFailRetryExecFields{
		manager:            svcmocks.NewMockIExptManager(ctrl),
		exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
		exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
		exptStatsRepo:      mock_repo.NewMockIExptStatsRepo(ctrl),
		idgenerator:        idgenmocks.NewMockIIDGenerator(ctrl),
		exptRepo:           mock_repo.NewMockIExperimentRepo(ctrl),
		idem:               idemmocks.NewMockIdempotentService(ctrl),
		configer:           configmocks.NewMockIConfiger(ctrl),
		publisher:          eventmocks.NewMockExptEventPublisher(ctrl),
	}

	e := &ExptFailRetryExec{
		manager:            f.manager,
		exptItemResultRepo: f.exptItemResultRepo,
		exptTurnResultRepo: f.exptTurnResultRepo,
		exptStatsRepo:      f.exptStatsRepo,
		idgenerator:        f.idgenerator,
		exptRepo:           f.exptRepo,
		idem:               f.idem,
		configer:           f.configer,
		publisher:          f.publisher,
	}

	assert.Equal(t, entity.EvaluationModeFailRetry, e.Mode())
}

func TestExptFailRetryExec_ExptStart(t *testing.T) {
	testUserID := "test_user_id_123"
	mockExpt := &entity.Experiment{
		ID:                  1,
		SpaceID:             3,
		CreatedBy:           "created_by",
		Name:                "created_by",
		Description:         "description",
		EvalSetVersionID:    1,
		EvalSetID:           1,
		TargetType:          1,
		TargetVersionID:     1,
		TargetID:            1,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 1}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1, IngressConf: &entity.TargetIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}},
			}},
			EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: ptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{
				{
					EvaluatorVersionID: 1,
					IngressConf:        &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}}},
				},
			}},
		}},
		Target: &entity.EvalTarget{ID: 1, SpaceID: 3, SourceTargetID: "source_target_id", EvalTargetType: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1, OutputSchema: []*entity.ArgsSchema{{Key: ptr.Of("key")}}}, BaseInfo: &entity.BaseInfo{}},
		EvalSet: &entity.EvaluationSet{
			ID: 1, SpaceID: 3, Name: "name", Description: "description", Status: 0, Spec: nil, Features: nil, ItemCount: 0, ChangeUncommitted: false,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1, AppID: 0, SpaceID: 3, EvaluationSetID: 1, Version: "version", VersionNum: 0, Description: "description", EvaluationSetSchema: nil, ItemCount: 0, BaseInfo: nil},
			LatestVersion:        "", NextVersionNum: 0, BaseInfo: nil, BizCategory: strconv.Itoa(1),
		},
		Evaluators:      []*entity.Evaluator{{}},
		Status:          0,
		StatusMessage:   "",
		LatestRunID:     0,
		CreditCost:      0,
		StartAt:         nil,
		EndAt:           nil,
		ExptType:        1,
		MaxAliveTime:    0,
		SourceType:      0,
		SourceID:        "",
		Stats:           nil,
		AggregateResult: nil,
	}

	type fields struct {
		manager            *svcmocks.MockIExptManager
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo *mock_repo.MockIExptTurnResultRepo
		exptStatsRepo      *mock_repo.MockIExptStatsRepo
		idgenerator        *idgenmocks.MockIIDGenerator
		exptRepo           *mock_repo.MockIExperimentRepo
		idem               *idemmocks.MockIdempotentService
		configer           *configmocks.MockIConfiger
		publisher          *eventmocks.MockExptEventPublisher
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		expt  *entity.Experiment
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "正常流程-全部成功",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				f.exptTurnResultRepo.EXPECT().ScanTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{
					{ItemID: 1, TurnID: 1, Status: int32(entity.TurnRunState_Fail)},
					{ItemID: 2, TurnID: 2, Status: int32(entity.TurnRunState_Terminal)},
				}, int64(0), nil).Times(1)
				f.exptTurnResultRepo.EXPECT().ScanTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{}, int64(0), nil).Times(1)
				f.idgenerator.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1, 2}, nil).AnyTimes()
				f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().UpdateTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptItemResultRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptStatsRepo.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptStats{
					ExptID:            1,
					SpaceID:           3,
					PendingItemCnt:    1,
					FailItemCnt:       1,
					TerminatedItemCnt: 1,
					ProcessingItemCnt: 1,
				}, nil).Times(1)
				f.exptStatsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1}).Times(1)
				f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "idem已存在",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil).Times(1)
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "idem检查失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, errors.New("idem error")).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "idem error")
			},
		},
		{
			name: "扫描失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				f.exptTurnResultRepo.EXPECT().ScanTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("scan error")).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "scan error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:            svcmocks.NewMockIExptManager(ctrl),
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:      mock_repo.NewMockIExptStatsRepo(ctrl),
				idgenerator:        idgenmocks.NewMockIIDGenerator(ctrl),
				exptRepo:           mock_repo.NewMockIExperimentRepo(ctrl),
				idem:               idemmocks.NewMockIdempotentService(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
				publisher:          eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			e := &ExptFailRetryExec{
				manager:            f.manager,
				exptItemResultRepo: f.exptItemResultRepo,
				exptTurnResultRepo: f.exptTurnResultRepo,
				exptStatsRepo:      f.exptStatsRepo,
				idgenerator:        f.idgenerator,
				exptRepo:           f.exptRepo,
				idem:               f.idem,
				configer:           f.configer,
				publisher:          f.publisher,
			}

			err := e.ExptStart(tt.args.ctx, tt.args.event, tt.args.expt)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptFailRetryExec_ScanEvalItems(t *testing.T) {
	testUserID := "test_user_id_123"
	mockExpt := &entity.Experiment{
		ID:                  1,
		SpaceID:             3,
		CreatedBy:           "created_by",
		Name:                "created_by",
		Description:         "description",
		EvalSetVersionID:    1,
		EvalSetID:           1,
		TargetType:          1,
		TargetVersionID:     1,
		TargetID:            1,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 1}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1, IngressConf: &entity.TargetIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}},
			}},
			EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: ptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{
				{
					EvaluatorVersionID: 1,
					IngressConf:        &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}}},
				},
			}},
		}},
		Target: &entity.EvalTarget{ID: 1, SpaceID: 3, SourceTargetID: "source_target_id", EvalTargetType: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1, OutputSchema: []*entity.ArgsSchema{{Key: ptr.Of("key")}}}, BaseInfo: &entity.BaseInfo{}},
		EvalSet: &entity.EvaluationSet{
			ID: 1, SpaceID: 3, Name: "name", Description: "description", Status: 0, Spec: nil, Features: nil, ItemCount: 0, ChangeUncommitted: false,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1, AppID: 0, SpaceID: 3, EvaluationSetID: 1, Version: "version", VersionNum: 0, Description: "description", EvaluationSetSchema: nil, ItemCount: 0, BaseInfo: nil},
			LatestVersion:        "", NextVersionNum: 0, BaseInfo: nil, BizCategory: strconv.Itoa(1),
		},
		Evaluators:      []*entity.Evaluator{{}},
		Status:          0,
		StatusMessage:   "",
		LatestRunID:     0,
		CreditCost:      0,
		StartAt:         nil,
		EndAt:           nil,
		ExptType:        1,
		MaxAliveTime:    0,
		SourceType:      0,
		SourceID:        "",
		Stats:           nil,
		AggregateResult: nil,
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		expt  *entity.Experiment
	}

	tests := []struct {
		name           string
		prepareMock    func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args)
		args           args
		wantToSubmit   []*entity.ExptEvalItem
		wantIncomplete []*entity.ExptEvalItem
		wantComplete   []*entity.ExptEvalItem
		wantErr        bool
		assertErr      func(t *testing.T, err error)
	}{
		{
			name: "正常流程-全部成功",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 3}}).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 1, Status: int32(entity.ItemRunState_Processing)},
				}, int64(1), nil).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 2, Status: int32(entity.ItemRunState_Queueing)},
				}, int64(1), nil).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 3, Status: int32(entity.ItemRunState_Success)},
				}, int64(1), nil).Times(1)
			},
			wantToSubmit: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 2, State: entity.ItemRunState_Queueing},
			},
			wantIncomplete: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 1, State: entity.ItemRunState_Processing},
			},
			wantComplete: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 3, State: entity.ItemRunState_Success},
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "扫描失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("scan error")).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "scan error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &exptFailRetryExecFields{
				manager:            svcmocks.NewMockIExptManager(ctrl),
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:      mock_repo.NewMockIExptStatsRepo(ctrl),
				idgenerator:        idgenmocks.NewMockIIDGenerator(ctrl),
				exptRepo:           mock_repo.NewMockIExperimentRepo(ctrl),
				idem:               idemmocks.NewMockIdempotentService(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
				publisher:          eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			e := &ExptFailRetryExec{
				manager:            f.manager,
				exptItemResultRepo: f.exptItemResultRepo,
				exptTurnResultRepo: f.exptTurnResultRepo,
				exptStatsRepo:      f.exptStatsRepo,
				idgenerator:        f.idgenerator,
				exptRepo:           f.exptRepo,
				idem:               f.idem,
				configer:           f.configer,
				publisher:          f.publisher,
			}

			toSubmit, incomplete, complete, err := e.ScanEvalItems(tt.args.ctx, tt.args.event, tt.args.expt)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
			if !tt.wantErr {
				assert.Equal(t, tt.wantToSubmit, toSubmit)
				assert.Equal(t, tt.wantIncomplete, incomplete)
				assert.Equal(t, tt.wantComplete, complete)
			}
		})
	}
}

func TestExptFailRetryExec_ExptEnd(t *testing.T) {
	testUserID := "test_user_id_123"
	mockExpt := &entity.Experiment{
		ID:                  1,
		SpaceID:             3,
		CreatedBy:           "created_by",
		Name:                "created_by",
		Description:         "description",
		EvalSetVersionID:    1,
		EvalSetID:           1,
		TargetType:          1,
		TargetVersionID:     1,
		TargetID:            1,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 1}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1, IngressConf: &entity.TargetIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}},
			}},
			EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: ptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{
				{
					EvaluatorVersionID: 1,
					IngressConf:        &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}}},
				},
			}},
		}},
		Target: &entity.EvalTarget{ID: 1, SpaceID: 3, SourceTargetID: "source_target_id", EvalTargetType: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1, OutputSchema: []*entity.ArgsSchema{{Key: ptr.Of("key")}}}, BaseInfo: &entity.BaseInfo{}},
		EvalSet: &entity.EvaluationSet{
			ID: 1, SpaceID: 3, Name: "name", Description: "description", Status: 0, Spec: nil, Features: nil, ItemCount: 0, ChangeUncommitted: false,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1, AppID: 0, SpaceID: 3, EvaluationSetID: 1, Version: "version", VersionNum: 0, Description: "description", EvaluationSetSchema: nil, ItemCount: 0, BaseInfo: nil},
			LatestVersion:        "", NextVersionNum: 0, BaseInfo: nil, BizCategory: strconv.Itoa(1),
		},
		Evaluators:      []*entity.Evaluator{{}},
		Status:          0,
		StatusMessage:   "",
		LatestRunID:     0,
		CreditCost:      0,
		StartAt:         nil,
		EndAt:           nil,
		ExptType:        1,
		MaxAliveTime:    0,
		SourceType:      0,
		SourceID:        "",
		Stats:           nil,
		AggregateResult: nil,
	}

	type args struct {
		ctx        context.Context
		event      *entity.ExptScheduleEvent
		expt       *entity.Experiment
		toSubmit   int
		incomplete int
	}

	tests := []struct {
		name         string
		prepareMock  func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args)
		args         args
		wantNextTick bool
		wantErr      bool
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "正常流程-全部完成",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt:       mockExpt,
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				f.manager.EXPECT().CompleteRun(gomock.Any(), args.event.ExptID, args.event.ExptRunID, args.event.ExptRunMode, args.event.SpaceID, args.event.Session, gomock.Any()).Return(nil).Times(1)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), args.event.ExptID, args.event.SpaceID, args.event.Session, gomock.Any()).Return(nil).Times(1)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), args.event.SpaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 100}).Times(1)
				f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantNextTick: false,
			wantErr:      false,
			assertErr:    func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "正常流程-未完成",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt:       mockExpt,
				toSubmit:   1,
				incomplete: 1,
			},
			prepareMock:  func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {},
			wantNextTick: true,
			wantErr:      false,
			assertErr:    func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "idem已存在",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt:       mockExpt,
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil).Times(1)
			},
			wantNextTick: false,
			wantErr:      false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "idem检查失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt:       mockExpt,
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, errors.New("idem error")).Times(1)
			},
			wantNextTick: false,
			wantErr:      true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "idem error")
			},
		},
		{
			name: "完成运行失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt:       mockExpt,
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				f.manager.EXPECT().CompleteRun(gomock.Any(), args.event.ExptID, args.event.ExptRunID, args.event.ExptRunMode, args.event.SpaceID, args.event.Session, gomock.Any()).Return(errors.New("test error")).Times(1)
			},
			wantNextTick: false,
			wantErr:      true,
			assertErr:    func(t *testing.T, err error) { assert.Error(t, err) },
		},
		{
			name: "完成实验失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt:       mockExpt,
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *exptFailRetryExecFields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				f.manager.EXPECT().CompleteRun(gomock.Any(), args.event.ExptID, args.event.ExptRunID, args.event.ExptRunMode, args.event.SpaceID, args.event.Session, gomock.Any()).Return(nil).Times(1)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), args.event.ExptID, args.event.SpaceID, args.event.Session, gomock.Any()).Return(errors.New("complete expt error")).Times(1)
			},
			wantNextTick: false,
			wantErr:      true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "complete expt error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &exptFailRetryExecFields{
				manager:            svcmocks.NewMockIExptManager(ctrl),
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:      mock_repo.NewMockIExptStatsRepo(ctrl),
				idgenerator:        idgenmocks.NewMockIIDGenerator(ctrl),
				exptRepo:           mock_repo.NewMockIExperimentRepo(ctrl),
				idem:               idemmocks.NewMockIdempotentService(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
				publisher:          eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			e := &ExptFailRetryExec{
				manager:            f.manager,
				exptItemResultRepo: f.exptItemResultRepo,
				exptTurnResultRepo: f.exptTurnResultRepo,
				exptStatsRepo:      f.exptStatsRepo,
				idgenerator:        f.idgenerator,
				exptRepo:           f.exptRepo,
				idem:               f.idem,
				configer:           f.configer,
				publisher:          f.publisher,
			}

			nextTick, err := e.ExptEnd(tt.args.ctx, tt.args.event, tt.args.expt, tt.args.toSubmit, tt.args.incomplete)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
			assert.Equal(t, tt.wantNextTick, nextTick)
		})
	}
}

func TestExptAppendExec_Mode(t *testing.T) {
	type fields struct {
		manager            *svcmocks.MockIExptManager
		exptRepo           *mock_repo.MockIExperimentRepo
		exptStatsRepo      *mock_repo.MockIExptStatsRepo
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo

		exptTurnResultRepo       *mock_repo.MockIExptTurnResultRepo
		idgenerator              *idgenmocks.MockIIDGenerator
		evaluationSetItemService *svcmocks.MockEvaluationSetItemService
		idem                     *idemmocks.MockIdempotentService
		configer                 *configmocks.MockIConfiger
		publisher                *eventmocks.MockExptEventPublisher
	}
	tests := []struct {
		name   string
		fields fields
		want   entity.ExptRunMode
	}{
		{
			name:   "正常流程",
			fields: fields{},
			want:   entity.EvaluationModeAppend,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := &fields{
				manager:                  svcmocks.NewMockIExptManager(ctrl),
				exptRepo:                 mock_repo.NewMockIExperimentRepo(ctrl),
				exptStatsRepo:            mock_repo.NewMockIExptStatsRepo(ctrl),
				exptItemResultRepo:       mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:       mock_repo.NewMockIExptTurnResultRepo(ctrl),
				idgenerator:              idgenmocks.NewMockIIDGenerator(ctrl),
				evaluationSetItemService: svcmocks.NewMockEvaluationSetItemService(ctrl),
				idem:                     idemmocks.NewMockIdempotentService(ctrl),
				configer:                 configmocks.NewMockIConfiger(ctrl),
				publisher:                eventmocks.NewMockExptEventPublisher(ctrl),
			}
			e := &ExptAppendExec{
				manager:                  f.manager,
				exptRepo:                 f.exptRepo,
				exptStatsRepo:            f.exptStatsRepo,
				exptItemResultRepo:       f.exptItemResultRepo,
				exptTurnResultRepo:       f.exptTurnResultRepo,
				idgenerator:              f.idgenerator,
				evaluationSetItemService: f.evaluationSetItemService,
				idem:                     f.idem,
				configer:                 f.configer,
				publisher:                f.publisher,
			}
			if got := e.Mode(); got != tt.want {
				t.Errorf("ExptAppendExec.Mode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExptAppendExec_ExptStart(t *testing.T) {
	testUserID := "test_user_id_123"
	type fields struct {
		manager                  *svcmocks.MockIExptManager
		exptRepo                 *mock_repo.MockIExperimentRepo
		exptStatsRepo            *mock_repo.MockIExptStatsRepo
		exptItemResultRepo       *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo       *mock_repo.MockIExptTurnResultRepo
		idgenerator              *idgenmocks.MockIIDGenerator
		evaluationSetItemService *svcmocks.MockEvaluationSetItemService
		idem                     *idemmocks.MockIdempotentService
		configer                 *configmocks.MockIConfiger
		publisher                *eventmocks.MockExptEventPublisher
	}
	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		expt  *entity.Experiment
	}
	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "正常流程",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Draining},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {},
			wantErr:     false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := &fields{
				manager:                  svcmocks.NewMockIExptManager(ctrl),
				exptRepo:                 mock_repo.NewMockIExperimentRepo(ctrl),
				exptStatsRepo:            mock_repo.NewMockIExptStatsRepo(ctrl),
				exptItemResultRepo:       mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:       mock_repo.NewMockIExptTurnResultRepo(ctrl),
				idgenerator:              idgenmocks.NewMockIIDGenerator(ctrl),
				evaluationSetItemService: svcmocks.NewMockEvaluationSetItemService(ctrl),
				idem:                     idemmocks.NewMockIdempotentService(ctrl),
				configer:                 configmocks.NewMockIConfiger(ctrl),
				publisher:                eventmocks.NewMockExptEventPublisher(ctrl),
			}
			e := &ExptAppendExec{
				manager:                  f.manager,
				exptRepo:                 f.exptRepo,
				exptStatsRepo:            f.exptStatsRepo,
				exptItemResultRepo:       f.exptItemResultRepo,
				exptTurnResultRepo:       f.exptTurnResultRepo,
				idgenerator:              f.idgenerator,
				evaluationSetItemService: f.evaluationSetItemService,
				idem:                     f.idem,
				configer:                 f.configer,
				publisher:                f.publisher,
			}
			err := e.ExptStart(tt.args.ctx, tt.args.event, tt.args.expt)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptAppendExec_ScanEvalItems(t *testing.T) {
	testUserID := "test_user_id_123"
	mockExpt := &entity.Experiment{
		ID:                  1,
		SpaceID:             3,
		CreatedBy:           "created_by",
		Name:                "created_by",
		Description:         "description",
		EvalSetVersionID:    1,
		EvalSetID:           1,
		TargetType:          1,
		TargetVersionID:     1,
		TargetID:            1,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 1}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1, IngressConf: &entity.TargetIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}},
			}},
			EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: ptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{
				{
					EvaluatorVersionID: 1,
					IngressConf:        &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field_name", FromField: "from_field"}}}},
				},
			}},
		}},
		Target: &entity.EvalTarget{ID: 1, SpaceID: 3, SourceTargetID: "source_target_id", EvalTargetType: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1, OutputSchema: []*entity.ArgsSchema{{Key: ptr.Of("key")}}}, BaseInfo: &entity.BaseInfo{}},
		EvalSet: &entity.EvaluationSet{
			ID: 1, SpaceID: 3, Name: "name", Description: "description", Status: 0, Spec: nil, Features: nil, ItemCount: 0, ChangeUncommitted: false,
			EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1, AppID: 0, SpaceID: 3, EvaluationSetID: 1, Version: "version", VersionNum: 0, Description: "description", EvaluationSetSchema: nil, ItemCount: 0, BaseInfo: nil},
			LatestVersion:        "", NextVersionNum: 0, BaseInfo: nil, BizCategory: strconv.Itoa(1),
		},
		Evaluators:      []*entity.Evaluator{{}},
		Status:          0,
		StatusMessage:   "",
		LatestRunID:     0,
		CreditCost:      0,
		StartAt:         nil,
		EndAt:           nil,
		ExptType:        1,
		MaxAliveTime:    0,
		SourceType:      0,
		SourceID:        "",
		Stats:           nil,
		AggregateResult: nil,
	}

	type fields struct {
		manager                  *svcmocks.MockIExptManager
		exptRepo                 *mock_repo.MockIExperimentRepo
		exptStatsRepo            *mock_repo.MockIExptStatsRepo
		exptItemResultRepo       *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo       *mock_repo.MockIExptTurnResultRepo
		idgenerator              *idgenmocks.MockIIDGenerator
		evaluationSetItemService *svcmocks.MockEvaluationSetItemService
		idem                     *idemmocks.MockIdempotentService
		configer                 *configmocks.MockIConfiger
		publisher                *eventmocks.MockExptEventPublisher
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		expt  *entity.Experiment
	}

	tests := []struct {
		name           string
		prepareMock    func(f *fields, ctrl *gomock.Controller, args args)
		args           args
		wantToSubmit   []*entity.ExptEvalItem
		wantIncomplete []*entity.ExptEvalItem
		wantComplete   []*entity.ExptEvalItem
		wantErr        bool
		assertErr      func(t *testing.T, err error)
	}{
		{
			name: "正常流程-全部成功",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 3}}).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 1, Status: int32(entity.ItemRunState_Processing)},
				}, int64(1), nil).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 2, Status: int32(entity.ItemRunState_Queueing)},
				}, int64(1), nil).Times(1)
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResultRunLog{
					{ItemID: 3, Status: int32(entity.ItemRunState_Success)},
				}, int64(1), nil).Times(1)
			},
			wantToSubmit: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 2, State: entity.ItemRunState_Queueing},
			},
			wantIncomplete: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 1, State: entity.ItemRunState_Processing},
			},
			wantComplete: []*entity.ExptEvalItem{
				{ExptID: 1, EvalSetVersionID: 1, ItemID: 3, State: entity.ItemRunState_Success},
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "扫描失败",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
				expt: mockExpt,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.exptItemResultRepo.EXPECT().ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("scan error")).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "scan error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:                  svcmocks.NewMockIExptManager(ctrl),
				exptRepo:                 mock_repo.NewMockIExperimentRepo(ctrl),
				exptStatsRepo:            mock_repo.NewMockIExptStatsRepo(ctrl),
				exptItemResultRepo:       mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:       mock_repo.NewMockIExptTurnResultRepo(ctrl),
				idgenerator:              idgenmocks.NewMockIIDGenerator(ctrl),
				evaluationSetItemService: svcmocks.NewMockEvaluationSetItemService(ctrl),
				idem:                     idemmocks.NewMockIdempotentService(ctrl),
				configer:                 configmocks.NewMockIConfiger(ctrl),
				publisher:                eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			e := &ExptAppendExec{
				manager:                  f.manager,
				exptRepo:                 f.exptRepo,
				exptStatsRepo:            f.exptStatsRepo,
				exptItemResultRepo:       f.exptItemResultRepo,
				exptTurnResultRepo:       f.exptTurnResultRepo,
				idgenerator:              f.idgenerator,
				evaluationSetItemService: f.evaluationSetItemService,
				idem:                     f.idem,
				configer:                 f.configer,
				publisher:                f.publisher,
			}

			toSubmit, incomplete, complete, err := e.ScanEvalItems(tt.args.ctx, tt.args.event, tt.args.expt)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
			if !tt.wantErr {
				assert.Equal(t, tt.wantToSubmit, toSubmit)
				assert.Equal(t, tt.wantIncomplete, incomplete)
				assert.Equal(t, tt.wantComplete, complete)
			}
		})
	}
}

func TestExptAppendExec_ExptEnd(t *testing.T) {
	testUserID := "test_user_id_123"

	type fields struct {
		manager            *svcmocks.MockIExptManager
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo
		idem               *idemmocks.MockIdempotentService
		configer           *configmocks.MockIConfiger
	}

	type args struct {
		ctx        context.Context
		event      *entity.ExptScheduleEvent
		expt       *entity.Experiment
		toSubmit   int
		incomplete int
	}

	tests := []struct {
		name         string
		prepareMock  func(f *fields, ctrl *gomock.Controller, args args)
		args         args
		wantNextTick bool
		wantErr      bool
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "正常流程-全部完成",
			args: args{
				ctx:        session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				expt:       &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Draining},
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				f.manager.EXPECT().CompleteRun(gomock.Any(), args.event.ExptID, args.event.ExptRunID, args.event.ExptRunMode, args.event.SpaceID, args.event.Session, gomock.Any()).Return(nil).Times(1)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), args.event.ExptID, args.event.SpaceID, args.event.Session, gomock.Any()).Return(nil).Times(1)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), args.event.SpaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 100}).Times(1)
				f.idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantNextTick: false,
			wantErr:      false,
			assertErr:    func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "正常流程-未完成",
			args: args{
				ctx:        session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				expt:       &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Draining},
				toSubmit:   1,
				incomplete: 1,
			},
			prepareMock:  func(f *fields, ctrl *gomock.Controller, args args) {},
			wantNextTick: true,
			wantErr:      false,
			assertErr:    func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "幂等检查失败",
			args: args{
				ctx:        session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				expt:       &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Draining},
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil).Times(1)
			},
			wantNextTick: false,
			wantErr:      false,
			assertErr:    func(t *testing.T, err error) { assert.NoError(t, err) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:            svcmocks.NewMockIExptManager(ctrl),
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				idem:               idemmocks.NewMockIdempotentService(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			svc := &ExptAppendExec{
				manager:            f.manager,
				exptItemResultRepo: f.exptItemResultRepo,
				idem:               f.idem,
				configer:           f.configer,
			}

			gotNextTick, err := svc.ExptEnd(tt.args.ctx, tt.args.event, tt.args.expt, tt.args.toSubmit, tt.args.incomplete)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
			assert.Equal(t, tt.wantNextTick, gotNextTick)
		})
	}
}

func TestExptAppendExec_ScheduleStart(t *testing.T) {
	testUserID := "test_user_id_123"

	type fields struct {
		manager            *svcmocks.MockIExptManager
		exptRepo           *mock_repo.MockIExperimentRepo
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo
		idem               *idemmocks.MockIdempotentService
		configer           *configmocks.MockIConfiger
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		expt  *entity.Experiment
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "正常流程",
			args: args{
				ctx:   session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				expt:  &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Processing, MaxAliveTime: 1000, StartAt: ptr.Of(time.Now().Add(-2 * time.Second))},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.exptRepo.EXPECT().Update(gomock.Any(), &entity.Experiment{
					ID:      args.event.ExptID,
					SpaceID: args.event.SpaceID,
					Status:  entity.ExptStatus_Draining,
				}).Return(nil).Times(1)
			},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "正常流程-已完成",
			args: args{
				ctx:   session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				expt:  &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Pending, MaxAliveTime: 5000, StartAt: ptr.Of(time.Now().Add(-2 * time.Second))},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:            svcmocks.NewMockIExptManager(ctrl),
				exptRepo:           mock_repo.NewMockIExperimentRepo(ctrl),
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				idem:               idemmocks.NewMockIdempotentService(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			svc := &ExptAppendExec{
				manager:            f.manager,
				exptRepo:           f.exptRepo,
				exptItemResultRepo: f.exptItemResultRepo,
				idem:               f.idem,
				configer:           f.configer,
			}

			err := svc.ScheduleStart(tt.args.ctx, tt.args.event, tt.args.expt)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptAppendExec_ScheduleEnd(t *testing.T) {
	testUserID := "test_user_id_123"

	type fields struct {
		manager            *svcmocks.MockIExptManager
		exptRepo           *mock_repo.MockIExperimentRepo
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo
		idem               *idemmocks.MockIdempotentService
		configer           *configmocks.MockIConfiger
	}

	type args struct {
		ctx        context.Context
		event      *entity.ExptScheduleEvent
		expt       *entity.Experiment
		toSubmit   int
		incomplete int
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "正常流程-无数据未完成",
			args: args{
				ctx:        session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				expt:       &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Processing},
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.manager.EXPECT().PendRun(gomock.Any(), args.event.ExptID, args.event.ExptRunID, args.event.SpaceID, args.event.Session).Return(nil).Times(1)
				f.manager.EXPECT().PendExpt(gomock.Any(), args.event.ExptID, args.event.SpaceID, args.event.Session).Return(nil).Times(1)
			},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "正常流程-已完成",
			args: args{
				ctx:        session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				expt:       &entity.Experiment{ID: 1, SpaceID: 3, Status: entity.ExptStatus_Success},
				toSubmit:   0,
				incomplete: 0,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {},
			wantErr:     false,
			assertErr:   func(t *testing.T, err error) { assert.NoError(t, err) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:            svcmocks.NewMockIExptManager(ctrl),
				exptRepo:           mock_repo.NewMockIExperimentRepo(ctrl),
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				idem:               idemmocks.NewMockIdempotentService(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			svc := &ExptAppendExec{
				manager:            f.manager,
				exptRepo:           f.exptRepo,
				exptItemResultRepo: f.exptItemResultRepo,
				idem:               f.idem,
				configer:           f.configer,
			}

			err := svc.ScheduleEnd(tt.args.ctx, tt.args.event, tt.args.expt, tt.args.toSubmit, tt.args.incomplete)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptAppendExec_NextTick(t *testing.T) {
	testUserID := "test_user_id_123"
	type fields struct {
		manager                  *svcmocks.MockIExptManager
		exptRepo                 *mock_repo.MockIExperimentRepo
		exptStatsRepo            *mock_repo.MockIExptStatsRepo
		exptItemResultRepo       *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo       *mock_repo.MockIExptTurnResultRepo
		idgenerator              *idgenmocks.MockIIDGenerator
		evaluationSetItemService *svcmocks.MockEvaluationSetItemService
		idem                     *idemmocks.MockIdempotentService
		configer                 *configmocks.MockIConfiger
		publisher                *eventmocks.MockExptEventPublisher
	}
	type args struct {
		ctx      context.Context
		event    *entity.ExptScheduleEvent
		nextTick bool
	}
	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "正常流程-需要下一次调度",
			args: args{
				ctx:      session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event:    &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				nextTick: true,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				// 显式指定调用次数为 1 次
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), args.event.SpaceID).Return(&entity.ExptExecConf{DaemonIntervalSecond: 5}).Times(1)
				f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), args.event, gomock.Any()).Return(nil).Times(1)
			},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		{
			name: "正常流程-不需要下一次调度",
			args: args{
				ctx:      session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event:    &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptRunMode: 1, Session: &entity.Session{UserID: testUserID}},
				nextTick: false,
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				// 不需要下一次调度时，不应该调用 GetExptExecConf 和 PublishExptScheduleEvent
				// 这里不设置预期调用
			},
			wantErr:   false,
			assertErr: func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		// ... 其他测试用例 ...
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := &fields{
				manager:                  svcmocks.NewMockIExptManager(ctrl),
				exptRepo:                 mock_repo.NewMockIExperimentRepo(ctrl),
				exptStatsRepo:            mock_repo.NewMockIExptStatsRepo(ctrl),
				exptItemResultRepo:       mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:       mock_repo.NewMockIExptTurnResultRepo(ctrl),
				idgenerator:              idgenmocks.NewMockIIDGenerator(ctrl),
				evaluationSetItemService: svcmocks.NewMockEvaluationSetItemService(ctrl),
				idem:                     idemmocks.NewMockIdempotentService(ctrl),
				configer:                 configmocks.NewMockIConfiger(ctrl),
				publisher:                eventmocks.NewMockExptEventPublisher(ctrl),
			}
			e := &ExptAppendExec{
				manager:                  f.manager,
				exptRepo:                 f.exptRepo,
				exptStatsRepo:            f.exptStatsRepo,
				exptItemResultRepo:       f.exptItemResultRepo,
				exptTurnResultRepo:       f.exptTurnResultRepo,
				idgenerator:              f.idgenerator,
				evaluationSetItemService: f.evaluationSetItemService,
				idem:                     f.idem,
				configer:                 f.configer,
				publisher:                f.publisher,
			}
			// 准备 mock
			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}
			err := e.NextTick(tt.args.ctx, tt.args.event, tt.args.nextTick)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestNewSchedulerModeFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	manager := svcmocks.NewMockIExptManager(ctrl)
	exptItemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	exptStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	exptTurnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	idgenerator := idgenmocks.NewMockIIDGenerator(ctrl)
	evaluationSetItemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	evaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)
	resultService := svcmocks.NewMockExptResultService(ctrl)

	factory := NewSchedulerModeFactory(
		manager,
		exptItemResultRepo,
		exptStatsRepo,
		exptTurnResultRepo,
		idgenerator,
		evaluationSetItemService,
		exptRepo,
		idem,
		configer,
		publisher,
		evaluatorRecordService,
		resultService,
	)

	tests := []struct {
		name      string
		mode      entity.ExptRunMode
		wantType  interface{}
		wantError bool
	}{
		{
			name:      "submit模式",
			mode:      entity.EvaluationModeSubmit,
			wantType:  &ExptSubmitExec{},
			wantError: false,
		},
		{
			name:      "failRetry模式",
			mode:      entity.EvaluationModeFailRetry,
			wantType:  &ExptFailRetryExec{},
			wantError: false,
		},
		{
			name:      "append模式",
			mode:      entity.EvaluationModeAppend,
			wantType:  &ExptAppendExec{},
			wantError: false,
		},
		{
			name:      "未知模式",
			mode:      999,
			wantType:  nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := factory.NewSchedulerMode(tt.mode)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, mode)
			} else {
				assert.NoError(t, err)
				assert.IsType(t, tt.wantType, mode)
			}
		})
	}
}

func TestNewExptSubmitMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	manager := svcmocks.NewMockIExptManager(ctrl)
	exptItemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	exptStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	exptTurnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	idgenerator := idgenmocks.NewMockIIDGenerator(ctrl)
	evaluationSetItemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	evaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)

	exec := NewExptSubmitMode(manager, exptItemResultRepo, exptStatsRepo, exptTurnResultRepo, idgenerator, evaluationSetItemService, exptRepo, idem, configer, publisher, evaluatorRecordService, resultSvc)
	assert.NotNil(t, exec)
	assert.Equal(t, manager, exec.manager)
	assert.Equal(t, exptItemResultRepo, exec.exptItemResultRepo)
	assert.Equal(t, exptStatsRepo, exec.exptStatsRepo)
	assert.Equal(t, exptTurnResultRepo, exec.exptTurnResultRepo)
	assert.Equal(t, idgenerator, exec.idgenerator)
	assert.Equal(t, evaluationSetItemService, exec.evaluationSetItemService)
	assert.Equal(t, exptRepo, exec.exptRepo)
	assert.Equal(t, idem, exec.idem)
	assert.Equal(t, configer, exec.configer)
	assert.Equal(t, publisher, exec.publisher)
}

func TestNewExptFailRetryMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	manager := svcmocks.NewMockIExptManager(ctrl)
	exptItemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	exptStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	exptTurnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	idgenerator := idgenmocks.NewMockIIDGenerator(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	evaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)

	exec := NewExptFailRetryMode(manager, exptItemResultRepo, exptStatsRepo, exptTurnResultRepo, idgenerator, exptRepo, idem, configer, publisher, evaluatorRecordService)
	assert.NotNil(t, exec)
	assert.Equal(t, manager, exec.manager)
	assert.Equal(t, exptItemResultRepo, exec.exptItemResultRepo)
	assert.Equal(t, exptStatsRepo, exec.exptStatsRepo)
	assert.Equal(t, exptTurnResultRepo, exec.exptTurnResultRepo)
	assert.Equal(t, idgenerator, exec.idgenerator)
	assert.Equal(t, exptRepo, exec.exptRepo)
	assert.Equal(t, idem, exec.idem)
	assert.Equal(t, configer, exec.configer)
	assert.Equal(t, publisher, exec.publisher)
}

func TestNewExptAppendMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	manager := svcmocks.NewMockIExptManager(ctrl)
	exptItemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	exptStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	exptTurnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	idgenerator := idgenmocks.NewMockIIDGenerator(ctrl)
	evaluationSetItemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	evaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)

	exec := NewExptAppendMode(manager, exptItemResultRepo, exptStatsRepo, exptTurnResultRepo, idgenerator, evaluationSetItemService, exptRepo, idem, configer, publisher, evaluatorRecordService)
	assert.NotNil(t, exec)
	assert.Equal(t, manager, exec.manager)
	assert.Equal(t, exptItemResultRepo, exec.exptItemResultRepo)
	assert.Equal(t, exptStatsRepo, exec.exptStatsRepo)
	assert.Equal(t, exptTurnResultRepo, exec.exptTurnResultRepo)
	assert.Equal(t, idgenerator, exec.idgenerator)
	assert.Equal(t, evaluationSetItemService, exec.evaluationSetItemService)
	assert.Equal(t, exptRepo, exec.exptRepo)
	assert.Equal(t, idem, exec.idem)
	assert.Equal(t, configer, exec.configer)
	assert.Equal(t, publisher, exec.publisher)
}

func TestExptSubmitExec_PublishResult(t *testing.T) {
	e := &ExptSubmitExec{}
	err := e.PublishResult(context.Background(), nil, nil)
	assert.NoError(t, err)
}

func TestExptFailRetryExec_PublishResult(t *testing.T) {
	type fields struct {
		manager                *svcmocks.MockIExptManager
		exptItemResultRepo     *mock_repo.MockIExptItemResultRepo
		idem                   *idemmocks.MockIdempotentService
		configer               *configmocks.MockIConfiger
		publisher              *eventmocks.MockExptEventPublisher
		evaluatorRecordService *svcmocks.MockEvaluatorRecordService
	}
	type args struct {
		ctx               context.Context
		turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef
		event             *entity.ExptScheduleEvent
	}
	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
	}{
		{
			name: "离线实验不发布",
			args: args{
				ctx: context.Background(),
				event: &entity.ExptScheduleEvent{
					ExptType: entity.ExptType_Offline,
				},
			},
			wantErr: false,
		},
		{
			name: "非离线实验-委托给baseExec",
			args: args{
				ctx: context.Background(),
				event: &entity.ExptScheduleEvent{
					ExptType: entity.ExptType_Online,
				},
				turnEvaluatorRefs: []*entity.ExptTurnEvaluatorResultRef{},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				// No mocks needed for empty refs
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := &fields{
				manager:                svcmocks.NewMockIExptManager(ctrl),
				exptItemResultRepo:     mock_repo.NewMockIExptItemResultRepo(ctrl),
				idem:                   idemmocks.NewMockIdempotentService(ctrl),
				configer:               configmocks.NewMockIConfiger(ctrl),
				publisher:              eventmocks.NewMockExptEventPublisher(ctrl),
				evaluatorRecordService: svcmocks.NewMockEvaluatorRecordService(ctrl),
			}
			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}
			e := &ExptFailRetryExec{
				manager:                f.manager,
				exptItemResultRepo:     f.exptItemResultRepo,
				idem:                   f.idem,
				configer:               f.configer,
				publisher:              f.publisher,
				evaluatorRecordService: f.evaluatorRecordService,
			}
			err := e.PublishResult(tt.args.ctx, tt.args.turnEvaluatorRefs, tt.args.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
	TestExptBaseExec_publishResult(t)
}

func TestExptBaseExec_publishResult(t *testing.T) {
	type fields struct {
		manager                *svcmocks.MockIExptManager
		idem                   *idemmocks.MockIdempotentService
		configer               *configmocks.MockIConfiger
		exptItemResultRepo     *mock_repo.MockIExptItemResultRepo
		evaluatorRecordService *svcmocks.MockEvaluatorRecordService
		publisher              *eventmocks.MockExptEventPublisher
	}
	type args struct {
		ctx               context.Context
		turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef
		event             *entity.ExptScheduleEvent
	}
	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "空refs直接返回",
			args: args{
				ctx:               context.Background(),
				turnEvaluatorRefs: []*entity.ExptTurnEvaluatorResultRef{},
				event:             &entity.ExptScheduleEvent{},
			},
			wantErr: false,
		},
		{
			name: "获取评估记录失败",
			args: args{
				ctx: context.Background(),
				turnEvaluatorRefs: []*entity.ExptTurnEvaluatorResultRef{
					{ExptID: 1, EvaluatorResultID: 101},
				},
				event: &entity.ExptScheduleEvent{},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.evaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{101}, true).Return(nil, errors.New("db error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "db error")
			},
		},
		{
			name: "发布事件失败",
			args: args{
				ctx: context.Background(),
				turnEvaluatorRefs: []*entity.ExptTurnEvaluatorResultRef{
					{ExptID: 1, EvaluatorResultID: 101},
				},
				event: &entity.ExptScheduleEvent{},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.evaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{101}, true).Return([]*entity.EvaluatorRecord{
					{ID: 101, Status: entity.EvaluatorRunStatusSuccess},
				}, nil)
				f.publisher.EXPECT().PublishExptOnlineEvalResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "publish error")
			},
		},
		{
			name: "正常流程-多种状态",
			args: args{
				ctx: context.Background(),
				turnEvaluatorRefs: []*entity.ExptTurnEvaluatorResultRef{
					{ExptID: 1, EvaluatorResultID: 101},
					{ExptID: 1, EvaluatorResultID: 102},
					{ExptID: 1, EvaluatorResultID: 103},
				},
				event: &entity.ExptScheduleEvent{},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.evaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{101, 102, 103}, true).Return([]*entity.EvaluatorRecord{
					{
						ID:     101,
						Status: entity.EvaluatorRunStatusSuccess,
						EvaluatorOutputData: &entity.EvaluatorOutputData{
							EvaluatorResult: &entity.EvaluatorResult{Score: ptr.Of(1.0), Reasoning: "good"},
						},
					},
					{
						ID:     102,
						Status: entity.EvaluatorRunStatusFail,
						EvaluatorOutputData: &entity.EvaluatorOutputData{
							EvaluatorRunError: &entity.EvaluatorRunError{Code: 123, Message: "failed"},
						},
					},
					{
						ID:     103,
						Status: 3, // custom status to test fallthrough
					},
				}, nil)
				f.publisher.EXPECT().PublishExptOnlineEvalResult(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, event *entity.OnlineExptEvalResultEvent, ttl *time.Duration) error {
						assert.Equal(t, int64(1), event.ExptId)
						assert.Len(t, event.TurnEvalResults, 3)
						// check result 101
						assert.Equal(t, int64(101), event.TurnEvalResults[0].EvaluatorRecordId)
						assert.Equal(t, float64(1.0), event.TurnEvalResults[0].Score)
						assert.Equal(t, "good", event.TurnEvalResults[0].Reasoning)
						// check result 102
						assert.Equal(t, int64(102), event.TurnEvalResults[1].EvaluatorRecordId)
						assert.Equal(t, int32(123), event.TurnEvalResults[1].EvaluatorRunError.Code)
						assert.Equal(t, "failed", event.TurnEvalResults[1].EvaluatorRunError.Message)
						// check result 103
						assert.Equal(t, int64(103), event.TurnEvalResults[2].EvaluatorRecordId)
						assert.Equal(t, int32(3), event.TurnEvalResults[2].Status)
						assert.Nil(t, event.TurnEvalResults[2].EvaluatorRunError)
						return nil
					})
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := &fields{
				manager:                svcmocks.NewMockIExptManager(ctrl),
				idem:                   idemmocks.NewMockIdempotentService(ctrl),
				configer:               configmocks.NewMockIConfiger(ctrl),
				exptItemResultRepo:     mock_repo.NewMockIExptItemResultRepo(ctrl),
				evaluatorRecordService: svcmocks.NewMockEvaluatorRecordService(ctrl),
				publisher:              eventmocks.NewMockExptEventPublisher(ctrl),
			}
			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}
			e := newExptBaseExec(f.manager, f.idem, f.configer, f.exptItemResultRepo, f.publisher, f.evaluatorRecordService)
			err := e.publishResult(tt.args.ctx, tt.args.turnEvaluatorRefs, tt.args.event)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.assertErr != nil {
					tt.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
