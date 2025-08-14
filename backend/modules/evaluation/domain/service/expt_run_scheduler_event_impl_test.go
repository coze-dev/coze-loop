// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptSchedulerImpl_Schedule(t *testing.T) {
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
		manager              *svcmocks.MockIExptManager
		resultSvc            *svcmocks.MockExptResultService
		exptRepo             *mock_repo.MockIExperimentRepo
		exptItemResultRepo   *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo   *mock_repo.MockIExptTurnResultRepo
		exptStatsRepo        *mock_repo.MockIExptStatsRepo
		configer             *configmocks.MockIConfiger
		idGen                *idgenmocks.MockIIDGenerator
		publisher            *eventmocks.MockExptEventPublisher
		idem                 *idemmocks.MockIdempotentService
		evalSetItemSvc       *svcmocks.MockEvaluationSetItemService
		mutex                *lockmocks.MockILocker
		schedulerModeFactory *svcmocks.MockSchedulerModeFactory
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args) // 修改点：添加 ctrl 参数
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
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // 修改点：添加 ctrl 参数
				f.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), args.event.Session).Return(mockExpt, nil).Times(1)
				f.manager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), args.event.Session).Return(&entity.ExptRunLog{}, nil).Times(1)
				f.mutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, args.ctx, func() {}, nil).Times(1)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: math.MaxInt}).AnyTimes()
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
				f.idGen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1, 2, 3}, nil).AnyTimes()
				f.publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				mode := entitymocks.NewMockExptSchedulerMode(ctrl)
				mode.EXPECT().ExptStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ExptEnd(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil).Times(1)
				mode.EXPECT().NextTick(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ScheduleStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ScanEvalItems(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, nil).Times(1)
				mode.EXPECT().ScheduleEnd(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.schedulerModeFactory.EXPECT().
					NewSchedulerMode(gomock.Any()).
					Return(mode, nil).Times(1)
				// 由于 mode 是内部 new 的，实际测试时需用 interface 替换或注入
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "实验报错",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:      1,
					ExptRunID:   2,
					SpaceID:     3,
					ExptRunMode: 1,
					Session:     &entity.Session{UserID: testUserID},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // 修改点：添加 ctrl 参数
				f.manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), args.event.Session).Return(mockExpt, nil).Times(1)
				f.manager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), args.event.Session).Return(&entity.ExptRunLog{}, nil).Times(1)
				f.mutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, args.ctx, func() {}, nil).Times(1)
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: math.MaxInt}).AnyTimes()
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
				f.idGen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{1, 2, 3}, nil).AnyTimes()
				f.manager.EXPECT().CompleteRun(gomock.Any(), int64(1), int64(2), gomock.Any(), gomock.Any(), args.event.Session, gomock.Any()).Return(nil).Times(1)
				f.manager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode := entitymocks.NewMockExptSchedulerMode(ctrl)
				mode.EXPECT().ExptStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ScheduleStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				mode.EXPECT().ScanEvalItems(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, []*entity.ExptEvalItem{}, nil).Times(1)
				mode.EXPECT().ScheduleEnd(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("test error")).Times(1)
				f.publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.schedulerModeFactory.EXPECT().
					NewSchedulerMode(gomock.Any()).
					Return(mode, nil).Times(1)
				// 由于 mode 是内部 new 的，实际测试时需用 interface 替换或注入
			},
			wantErr: false,
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
				manager:              svcmocks.NewMockIExptManager(ctrl),
				exptRepo:             mock_repo.NewMockIExperimentRepo(ctrl),
				exptItemResultRepo:   mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo:   mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:        mock_repo.NewMockIExptStatsRepo(ctrl),
				configer:             configmocks.NewMockIConfiger(ctrl),
				idGen:                idgenmocks.NewMockIIDGenerator(ctrl),
				publisher:            eventmocks.NewMockExptEventPublisher(ctrl),
				idem:                 idemmocks.NewMockIdempotentService(ctrl),
				evalSetItemSvc:       svcmocks.NewMockEvaluationSetItemService(ctrl),
				mutex:                lockmocks.NewMockILocker(ctrl),
				schedulerModeFactory: svcmocks.NewMockSchedulerModeFactory(ctrl),
				resultSvc:            svcmocks.NewMockExptResultService(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args) // 修改点：传递 ctrl
			}

			svc := &ExptSchedulerImpl{
				Manager:                  f.manager,
				ExptRepo:                 f.exptRepo,
				ExptItemResultRepo:       f.exptItemResultRepo,
				ExptTurnResultRepo:       f.exptTurnResultRepo,
				ExptStatsRepo:            f.exptStatsRepo,
				Configer:                 f.configer,
				IDGen:                    f.idGen,
				Publisher:                f.publisher,
				Idem:                     f.idem,
				evaluationSetItemService: f.evalSetItemSvc,
				Mutex:                    f.mutex,
				schedulerModeFactory:     f.schedulerModeFactory,
				ResultSvc:                f.resultSvc,
			}
			svc.Endpoints = SchedulerChain(
				svc.HandleEventErr,
				svc.SysOps,
				svc.HandleEventCheck,
				svc.HandleEventLock,
				svc.HandleEventEndpoint,
			)(func(_ context.Context, _ *entity.ExptScheduleEvent) error { return nil })

			err := svc.Schedule(tt.args.ctx, tt.args.event)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptSchedulerImpl_RecordEvalItemRunLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testUserID := "test_user_id_123"

	type fields struct {
		ResultSvc *svcmocks.MockExptResultService
		Publisher *eventmocks.MockExptEventPublisher
	}

	type args struct {
		ctx           context.Context
		event         *entity.ExptScheduleEvent
		completeItems []*entity.ExptEvalItem
	}

	mockMode := entitymocks.NewMockExptSchedulerMode(ctrl)

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args) // 修改点：添加 ctrl 参数
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
				completeItems: []*entity.ExptEvalItem{
					{ItemID: 1, State: entity.ItemRunState_Success},
					{ItemID: 2, State: entity.ItemRunState_Fail},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // 修改点：添加 ctrl 参数
				f.ResultSvc.EXPECT().RecordItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				mockMode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.ResultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.Publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fields{
				ResultSvc: svcmocks.NewMockExptResultService(ctrl),
				Publisher: eventmocks.NewMockExptEventPublisher(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args) // 修改点：传递 ctrl
			}

			svc := &ExptSchedulerImpl{
				ResultSvc: f.ResultSvc,
				Publisher: f.Publisher,
			}

			err := svc.recordEvalItemRunLogs(tt.args.ctx, tt.args.event, tt.args.completeItems, mockMode)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestExptSchedulerImpl_SubmitItemEval(t *testing.T) {
	testUserID := "test_user_id_123"

	type fields struct {
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo *mock_repo.MockIExptTurnResultRepo
		exptStatsRepo      *mock_repo.MockIExptStatsRepo
		configer           *configmocks.MockIConfiger
		publisher          *eventmocks.MockExptEventPublisher
		metric             *metricsmocks.MockExptMetric
		resultSvc          *svcmocks.MockExptResultService
	}

	type args struct {
		ctx       context.Context
		event     *entity.ExptScheduleEvent
		toSubmits []*entity.ExptEvalItem
		expt      *entity.Experiment
	}

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args) // 修改点：添加 ctrl 参数
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
				toSubmits: []*entity.ExptEvalItem{
					{ItemID: 1, State: entity.ItemRunState_Success},
					{ItemID: 2, State: entity.ItemRunState_Fail},
					{ItemID: 3, State: entity.ItemRunState_Queueing},
					{ItemID: 4, State: entity.ItemRunState_Processing},
				},
				expt: &entity.Experiment{
					ID:       1,
					SpaceID:  1,
					ExptType: entity.ExptType_Offline,
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) { // 修改点：添加 ctrl 参数
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptItemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{}, nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().UpdateTurnResultsWithItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.exptTurnResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{}, nil).AnyTimes()
				f.publisher.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.configer.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{
					ExptItemEvalConf: &entity.ExptItemEvalConf{
						ConcurNum:      1,
						IntervalSecond: 1,
					},
				}).AnyTimes()
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
				f.exptStatsRepo.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				f.metric.EXPECT().EmitItemExecEval(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				f.resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			wantErr: false,
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
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
				exptStatsRepo:      mock_repo.NewMockIExptStatsRepo(ctrl),
				configer:           configmocks.NewMockIConfiger(ctrl),
				publisher:          eventmocks.NewMockExptEventPublisher(ctrl),
				metric:             metricsmocks.NewMockExptMetric(ctrl),
				resultSvc:          svcmocks.NewMockExptResultService(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args) // 修改点：传递 ctrl
			}

			svc := &ExptSchedulerImpl{
				ExptItemResultRepo: f.exptItemResultRepo,
				ExptTurnResultRepo: f.exptTurnResultRepo,
				ExptStatsRepo:      f.exptStatsRepo,
				Configer:           f.configer,
				Publisher:          f.publisher,
				Metric:             f.metric,
				ResultSvc:          f.resultSvc,
			}

			err := svc.handleToSubmits(tt.args.ctx, tt.args.event, tt.args.toSubmits)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}
		})
	}
}

func TestNewExptSchedulerSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	manager := svcmocks.NewMockIExptManager(ctrl)
	exptRepo := mock_repo.NewMockIExperimentRepo(ctrl)
	exptItemResultRepo := mock_repo.NewMockIExptItemResultRepo(ctrl)
	exptTurnResultRepo := mock_repo.NewMockIExptTurnResultRepo(ctrl)
	exptStatsRepo := mock_repo.NewMockIExptStatsRepo(ctrl)
	exptRunLogRepo := mock_repo.NewMockIExptRunLogRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	quotaRepo := mock_repo.NewMockQuotaRepo(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	auditClient := auditmocks.NewMockIAuditService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	idGen := idgenmocks.NewMockIIDGenerator(ctrl)
	evalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	schedulerModeFactory := svcmocks.NewMockSchedulerModeFactory(ctrl)

	svc := NewExptSchedulerSvc(
		manager,
		exptRepo,
		exptItemResultRepo,
		exptTurnResultRepo,
		exptStatsRepo,
		exptRunLogRepo,
		idem,
		configer,
		quotaRepo,
		mutex,
		publisher,
		auditClient,
		metric,
		resultSvc,
		idGen,
		evalSetItemSvc,
		schedulerModeFactory,
	)
	assert.NotNil(t, svc)
	assert.Implements(t, (*ExptSchedulerEvent)(nil), svc)
	impl, ok := svc.(*ExptSchedulerImpl)
	assert.True(t, ok)
	assert.Equal(t, manager, impl.Manager)
	assert.Equal(t, exptRepo, impl.ExptRepo)
	assert.Equal(t, exptItemResultRepo, impl.ExptItemResultRepo)
	assert.Equal(t, exptTurnResultRepo, impl.ExptTurnResultRepo)
	assert.Equal(t, exptStatsRepo, impl.ExptStatsRepo)
	assert.Equal(t, exptRunLogRepo, impl.ExptRunLogRepo)
	assert.Equal(t, idem, impl.Idem)
	assert.Equal(t, configer, impl.Configer)
	assert.Equal(t, quotaRepo, impl.QuotaRepo)
	assert.Equal(t, mutex, impl.Mutex)
	assert.Equal(t, publisher, impl.Publisher)
	assert.Equal(t, auditClient, impl.AuditClient)
	assert.Equal(t, metric, impl.Metric)
	assert.Equal(t, resultSvc, impl.ResultSvc)
	assert.Equal(t, idGen, impl.IDGen)
	assert.Equal(t, evalSetItemSvc, impl.evaluationSetItemService)
	assert.Equal(t, schedulerModeFactory, impl.schedulerModeFactory)
}

func TestExptSchedulerImpl_HandleEventLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mutex := lockmocks.NewMockILocker(ctrl)
	svc := &ExptSchedulerImpl{
		Mutex: mutex,
	}

	type lockArgs struct {
		event   *entity.ExptScheduleEvent
		locked  bool
		lockErr error
	}

	tests := []struct {
		name    string
		args    lockArgs
		next    func(ctx context.Context, event *entity.ExptScheduleEvent) error
		wantErr bool
		wantNil bool // 是否期望返回 nil（即锁未获得时）
	}{
		{
			name: "正常加锁并调用next",
			args: lockArgs{
				event:   &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2},
				locked:  true,
				lockErr: nil,
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return nil
			},
			wantErr: false,
			wantNil: false,
		},
		{
			name: "加锁失败返回错误",
			args: lockArgs{
				event:   &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2},
				locked:  false,
				lockErr: errors.New("lock error"),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return nil
			},
			wantErr: true,
			wantNil: false,
		},
		{
			name: "未获得锁直接返回nil",
			args: lockArgs{
				event:   &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2},
				locked:  false,
				lockErr: nil,
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return errors.New("should not be called")
			},
			wantErr: false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unlockCalled := false
			mutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.args.locked, context.Background(), func() { unlockCalled = true }, tt.args.lockErr)
			handler := svc.HandleEventLock(tt.next)
			err := handler(context.Background(), tt.args.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantNil {
				assert.Nil(t, err)
			}
			if tt.args.locked && !tt.wantErr {
				assert.True(t, unlockCalled, "unlock should be called when locked")
			}
		})
	}
}

func TestExptSchedulerImpl_HandleEventCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	manager := svcmocks.NewMockIExptManager(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	svc := &ExptSchedulerImpl{
		Manager:  manager,
		Configer: configer,
	}

	type checkArgs struct {
		event      *entity.ExptScheduleEvent
		runLog     *entity.ExptRunLog
		runLogErr  error
		zombieSecs int64
		createdAt  int64
	}

	tests := []struct {
		name        string
		args        checkArgs
		next        func(ctx context.Context, event *entity.ExptScheduleEvent) error
		preparemock func()
		wantErr     bool
	}{
		{
			name: "正常流程，未完成，未超时，调用next",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, CreatedAt: time.Now().Unix()},
				runLog:     &entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)},
				runLogErr:  nil,
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error { return nil },
			preparemock: func() {
				configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: int(10000)}).Times(1)
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "runLog返回错误",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				runLog:     nil,
				runLogErr:  errors.New("db error"),
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error { return nil },
			preparemock: func() {
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db error")).Times(1)
			},
			wantErr: true,
		},
		{
			name: "实验已完成直接返回nil",
			args: checkArgs{
				event:      &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3},
				runLog:     &entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)},
				runLogErr:  nil,
				zombieSecs: 10000,
				createdAt:  time.Now().Unix(),
			},
			next: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return errors.New("should not be called")
			},
			preparemock: func() {
				manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil).Times(1)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.preparemock()
			handler := svc.HandleEventCheck(tt.next)
			err := handler(context.Background(), tt.args.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptSchedulerImpl_handleZombies(t *testing.T) {
	testUserID := "test_user_id_123"

	type fields struct {
		configer           *configmocks.MockIConfiger
		exptItemResultRepo *mock_repo.MockIExptItemResultRepo
		exptTurnResultRepo *mock_repo.MockIExptTurnResultRepo
	}

	type args struct {
		ctx   context.Context
		event *entity.ExptScheduleEvent
		items []*entity.ExptEvalItem
	}

	now := time.Now()
	zombieTime := now.Add(-10 * time.Minute)
	aliveTime := now.Add(-1 * time.Minute)

	tests := []struct {
		name        string
		prepareMock func(f *fields, ctrl *gomock.Controller, args args)
		args        args
		wantAlives  []*entity.ExptEvalItem
		wantZombies []*entity.ExptEvalItem
		wantErr     bool
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "Normal case - no zombie tasks",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    1,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
				{
					ExptID:    1,
					ItemID:    2,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{},
			wantErr:     false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Normal case - zombie tasks need to be handled",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
					{
						ExptID:    1,
						ItemID:    3,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1, 3},
					map[string]any{"status": int32(entity.ItemRunState_Fail)},
					int64(3),
				).Return(nil).Times(1)
				f.exptTurnResultRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(
					gomock.Any(),
					int64(3),
					int64(1),
					int64(2),
					[]int64{1, 3},
					entity.TurnRunState_Fail,
				).Return(nil).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    2,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    1,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
				{
					ExptID:    1,
					ItemID:    3,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Error case - UpdateItemRunLog failed",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1},
					map[string]any{"status": int32(entity.ItemRunState_Fail)},
					int64(3),
				).Return(errors.New("update item run log failed")).Times(1)
			},
			wantAlives:  nil,
			wantZombies: nil,
			wantErr:     true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "update item run log failed")
			},
		},
		{
			name: "Error case - UpdateTurnRunLog failed",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1},
					map[string]any{"status": int32(entity.ItemRunState_Fail)},
					int64(3),
				).Return(nil).Times(1)
				f.exptTurnResultRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(
					gomock.Any(),
					int64(3),
					int64(1),
					int64(2),
					[]int64{1},
					entity.TurnRunState_Fail,
				).Return(errors.New("update turn run log failed")).Times(1)
			},
			wantAlives:  nil,
			wantZombies: nil,
			wantErr:     true,
			assertErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "update turn run log failed")
			},
		},
		{
			name: "Edge case - all tasks are zombies",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &zombieTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
				f.exptItemResultRepo.EXPECT().UpdateItemRunLog(
					gomock.Any(),
					int64(1),
					int64(2),
					[]int64{1, 2},
					map[string]any{"status": int32(entity.ItemRunState_Fail)},
					int64(3),
				).Return(nil).Times(1)
				f.exptTurnResultRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(
					gomock.Any(),
					int64(3),
					int64(1),
					int64(2),
					[]int64{1, 2},
					entity.TurnRunState_Fail,
				).Return(nil).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{},
			wantZombies: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    1,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
				{
					ExptID:    1,
					ItemID:    2,
					State:     entity.ItemRunState_Fail,
					UpdatedAt: &zombieTime,
				},
			},
			wantErr: false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Edge case - task update time is nil",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: nil,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &time.Time{},
					},
					{
						ExptID:    1,
						ItemID:    3,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    3,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{},
			wantErr:     false,
			assertErr: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Edge case - tasks with non-Processing state",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				event: &entity.ExptScheduleEvent{
					ExptID:    1,
					ExptRunID: 2,
					SpaceID:   3,
					Session:   &entity.Session{UserID: testUserID},
				},
				items: []*entity.ExptEvalItem{
					{
						ExptID:    1,
						ItemID:    1,
						State:     entity.ItemRunState_Queueing,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    2,
						State:     entity.ItemRunState_Success,
						UpdatedAt: &zombieTime,
					},
					{
						ExptID:    1,
						ItemID:    3,
						State:     entity.ItemRunState_Processing,
						UpdatedAt: &aliveTime,
					},
				},
			},
			prepareMock: func(f *fields, ctrl *gomock.Controller, args args) {
				f.configer.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{
					SpaceExptExecConf: map[int64]*entity.ExptExecConf{
						3: {
							ExptItemEvalConf: &entity.ExptItemEvalConf{
								ZombieSecond: 300,
							},
						},
					},
				}).Times(1)
			},
			wantAlives: []*entity.ExptEvalItem{
				{
					ExptID:    1,
					ItemID:    3,
					State:     entity.ItemRunState_Processing,
					UpdatedAt: &aliveTime,
				},
			},
			wantZombies: []*entity.ExptEvalItem{},
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
				configer:           configmocks.NewMockIConfiger(ctrl),
				exptItemResultRepo: mock_repo.NewMockIExptItemResultRepo(ctrl),
				exptTurnResultRepo: mock_repo.NewMockIExptTurnResultRepo(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(f, ctrl, tt.args)
			}

			svc := &ExptSchedulerImpl{
				Configer:           f.configer,
				ExptItemResultRepo: f.exptItemResultRepo,
				ExptTurnResultRepo: f.exptTurnResultRepo,
			}

			alives, zombies, err := svc.handleZombies(tt.args.ctx, tt.args.event, tt.args.items)

			if tt.assertErr != nil {
				tt.assertErr(t, err)
			}

			if !tt.wantErr {
				assert.Equal(t, len(tt.wantAlives), len(alives), "alives count should match")
				assert.Equal(t, len(tt.wantZombies), len(zombies), "zombies count should match")

				for i, expectedAlive := range tt.wantAlives {
					if i < len(alives) {
						assert.Equal(t, expectedAlive.ItemID, alives[i].ItemID, "alive item ID should match")
						assert.Equal(t, expectedAlive.State, alives[i].State, "alive item state should match")
					}
				}

				for i, expectedZombie := range tt.wantZombies {
					if i < len(zombies) {
						assert.Equal(t, expectedZombie.ItemID, zombies[i].ItemID, "zombie item ID should match")
						assert.Equal(t, expectedZombie.State, zombies[i].State, "zombie item state should be Fail")
					}
				}
			}
		})
	}
}
