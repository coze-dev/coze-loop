// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// FailRetry 构造器新增可选 resultSvc: 传入时应保存, 不传时字段为 nil。
func TestNewExptFailRetryMode_WithResultSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	exec := NewExptFailRetryMode(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, resultSvc)
	assert.Same(t, resultSvc, exec.resultSvc)

	execNo := NewExptFailRetryMode(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	assert.Nil(t, execNo.resultSvc)
}

// RetryAll 构造器新增 exptItemRefRepo/resultSvc 必填 + notifier 可选。
func TestNewExptRetryAllExec_WithNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	itemRefRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)

	exec := NewExptRetryAllExec(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, itemRefRepo, resultSvc, notifier)
	assert.Same(t, notifier, exec.sandboxAgentNotifier)
	assert.Same(t, resultSvc, exec.resultSvc)
	assert.Same(t, itemRefRepo, exec.exptItemRefRepo)

	execNoNotif := NewExptRetryAllExec(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, itemRefRepo, resultSvc)
	assert.Nil(t, execNoNotif.sandboxAgentNotifier)
	assert.Same(t, resultSvc, execNoNotif.resultSvc)
}

// RetryItems 构造器新增 resultSvc 必填 + notifier 可选。
func TestNewExptRetryItemsExec_WithNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	itemRefRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
	runLogRepo := mock_repo.NewMockIExptRunLogRepo(ctrl)

	exec := NewExptRetryItemsExec(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, runLogRepo, itemRefRepo, resultSvc, notifier)
	assert.Same(t, notifier, exec.sandboxAgentNotifier)
	assert.Same(t, resultSvc, exec.resultSvc)
	assert.Same(t, itemRefRepo, exec.exptItemRefRepo)
	assert.Same(t, runLogRepo, exec.exptRunLogRepo)
}

// NewSchedulerModeFactory notifier 可选: 传入时保存, 不传时字段为 nil。
func TestNewSchedulerModeFactory_WithNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)

	f := NewSchedulerModeFactory(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, notifier).(*DefaultSchedulerModeFactory)
	assert.Same(t, notifier, f.sandboxAgentNotifier)

	f2 := NewSchedulerModeFactory(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).(*DefaultSchedulerModeFactory)
	assert.Nil(t, f2.sandboxAgentNotifier)
}

func TestNewExptSchedulerSvc_InjectsFailRetryFactoryDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	factory := &DefaultSchedulerModeFactory{}

	NewExptSchedulerSvc(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		metric, nil, nil, nil, factory, targetSvc, nil, nil, nil,
		component.NewNoopCentralReservationGuard(),
	)

	assert.Same(t, targetSvc, factory.evalTargetService)
	assert.Same(t, metric, factory.metric)
}

// factory.NewSchedulerMode RetryAll 分派: 应把 notifier + resultSvc 注入到 exec。
func TestFactoryDispatch_RetryAllInjectsNotifierAndResultSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)

	f := &DefaultSchedulerModeFactory{
		resultSvc:            resultSvc,
		sandboxAgentNotifier: notifier,
	}
	mode, err := f.NewSchedulerMode(entity.EvaluationModeRetryAll)
	assert.NoError(t, err)
	exec, ok := mode.(*ExptRetryAllExec)
	assert.True(t, ok)
	assert.Same(t, notifier, exec.sandboxAgentNotifier)
	assert.Same(t, resultSvc, exec.resultSvc)
}

// factory.NewSchedulerMode RetryItems 分派: 同上。
func TestFactoryDispatch_RetryItemsInjectsNotifierAndResultSvc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)

	f := &DefaultSchedulerModeFactory{
		resultSvc:            resultSvc,
		sandboxAgentNotifier: notifier,
	}
	mode, err := f.NewSchedulerMode(entity.EvaluationModeRetryItems)
	assert.NoError(t, err)
	exec, ok := mode.(*ExptRetryItemsExec)
	assert.True(t, ok)
	assert.Same(t, notifier, exec.sandboxAgentNotifier)
	assert.Same(t, resultSvc, exec.resultSvc)
}

// factory.NewSchedulerMode FailRetry 分派: 透传 RetryFailure 启动规划所需依赖。
func TestFactoryDispatch_FailRetryInjectsDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	itemRefRepo := mock_repo.NewMockIExptItemRefRepo(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)

	f := &DefaultSchedulerModeFactory{
		resultSvc:            resultSvc,
		exptItemRefRepo:      itemRefRepo,
		evalTargetService:    targetSvc,
		metric:               metric,
		sandboxAgentNotifier: notifier, // factory 有, 但 FailRetry 不接收
	}
	mode, err := f.NewSchedulerMode(entity.EvaluationModeFailRetry)
	assert.NoError(t, err)
	exec, ok := mode.(*ExptFailRetryExec)
	assert.True(t, ok)
	assert.Same(t, resultSvc, exec.resultSvc)
	assert.Same(t, itemRefRepo, exec.exptItemRefRepo)
	assert.Same(t, targetSvc, exec.evalTargetService)
	assert.Same(t, metric, exec.metric)
}

// RetryAll ExptStart 幂等短路: idem.Exist 返回 true 时直接返回 nil, 不触碰 notifier/resultSvc。
func TestExptRetryAllExec_ExptStart_IdempotentShortCircuit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil)

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	// notifier + resultSvc 不应被调用。

	exec := &ExptRetryAllExec{
		idem:                 idem,
		resultSvc:            resultSvc,
		sandboxAgentNotifier: notifier,
	}
	err := exec.ExptStart(t.Context(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, &entity.Experiment{})
	assert.NoError(t, err)
}

// RetryItems ExptStart 幂等短路。
func TestExptRetryItemsExec_ExptStart_IdempotentShortCircuit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil)

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)

	exec := &ExptRetryItemsExec{
		idem:                 idem,
		resultSvc:            resultSvc,
		sandboxAgentNotifier: notifier,
	}
	err := exec.ExptStart(t.Context(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, &entity.Experiment{})
	assert.NoError(t, err)
}

// FailRetry ExptStart 幂等短路 (确认 FailRetry 也保持既有短路语义, 不引入 notifier)。
func TestExptFailRetryExec_ExptStart_IdempotentShortCircuit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)

	exec := &ExptFailRetryExec{idem: idem, resultSvc: resultSvc}
	err := exec.ExptStart(t.Context(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, &entity.Experiment{})
	assert.NoError(t, err)
}

// idem.Exist 返回错误时, 三个 executor 应传播错误。
func TestExptStartIdempotentErrPropagated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	newIdem := func(t *testing.T) *idemmocks.MockIdempotentService {
		m := idemmocks.NewMockIdempotentService(ctrl)
		m.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, assert.AnError)
		return m
	}

	t.Run("FailRetry", func(t *testing.T) {
		exec := &ExptFailRetryExec{idem: newIdem(t)}
		err := exec.ExptStart(t.Context(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, &entity.Experiment{})
		assert.Error(t, err)
	})
	t.Run("RetryAll", func(t *testing.T) {
		exec := &ExptRetryAllExec{idem: newIdem(t)}
		err := exec.ExptStart(t.Context(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, &entity.Experiment{})
		assert.Error(t, err)
	})
	t.Run("RetryItems", func(t *testing.T) {
		exec := &ExptRetryItemsExec{idem: newIdem(t)}
		err := exec.ExptStart(t.Context(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, &entity.Experiment{})
		assert.Error(t, err)
	})
}

// 保底: 构造出的三个 executor Mode() 返回正确的枚举。
func TestRetryExecModes(t *testing.T) {
	assert.Equal(t, entity.EvaluationModeFailRetry, (&ExptFailRetryExec{}).Mode())
	assert.Equal(t, entity.EvaluationModeRetryAll, (&ExptRetryAllExec{}).Mode())
	assert.Equal(t, entity.EvaluationModeRetryItems, (&ExptRetryItemsExec{}).Mode())
}

// 保底: 未使用的 import 变量, 避免"imported and not used"。
var (
	_ = idgenmocks.NewMockIIDGenerator
	_ = lockMocks.NewMockILocker
	_ = configmocks.NewMockIConfiger
	_ = eventmocks.NewMockExptEventPublisher
)
