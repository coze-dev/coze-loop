// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// SandboxAgentHourlyNotify: next 返回 error 时不进 notifier 路径, 直接透传错误。
func TestSandboxAgentHourlyNotify_NextErrorPropagated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	impl := &ExptSchedulerImpl{sandboxAgentNotifier: notifier}
	// notifier 不注册期望, 若被调用会 fail。

	mw := impl.SandboxAgentHourlyNotify(func(_ context.Context, _ *entity.ExptScheduleEvent) error {
		return errors.New("schedule boom")
	})
	err := mw(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2})
	assert.EqualError(t, err, "schedule boom")
}

// SandboxAgentHourlyNotify: notifier 为 nil 时 middleware no-op, 依然返回 next 的结果。
func TestSandboxAgentHourlyNotify_NilNotifier(t *testing.T) {
	impl := &ExptSchedulerImpl{}
	called := false
	mw := impl.SandboxAgentHourlyNotify(func(_ context.Context, _ *entity.ExptScheduleEvent) error {
		called = true
		return nil
	})
	err := mw(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2})
	assert.NoError(t, err)
	assert.True(t, called)
}

// SandboxAgentHourlyNotify: 主流程成功 + GetDetail 失败时, 静默不发通知, 返回 nil。
func TestSandboxAgentHourlyNotify_GetDetailFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	manager := svcmocks.NewMockIExptManager(ctrl)
	manager.EXPECT().
		GetDetail(gomock.Any(), int64(1), int64(2), gomock.Any()).
		Return(nil, errors.New("db down"))
	// notifier.NotifyProgressIfDue 不应被调用。

	impl := &ExptSchedulerImpl{Manager: manager, sandboxAgentNotifier: notifier}
	mw := impl.SandboxAgentHourlyNotify(func(_ context.Context, _ *entity.ExptScheduleEvent) error {
		return nil
	})
	err := mw(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2})
	assert.NoError(t, err)
}

// SandboxAgentHourlyNotify: 主流程成功 + GetDetail 成功时, 调用 NotifyProgressIfDue。
func TestSandboxAgentHourlyNotify_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	manager := svcmocks.NewMockIExptManager(ctrl)
	expt := &entity.Experiment{ID: 1, SpaceID: 2}
	manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(2), gomock.Any()).Return(expt, nil)
	notifier.EXPECT().
		NotifyProgressIfDue(gomock.Any(), expt).
		Return(nil).
		Times(1)

	impl := &ExptSchedulerImpl{Manager: manager, sandboxAgentNotifier: notifier}
	mw := impl.SandboxAgentHourlyNotify(func(_ context.Context, _ *entity.ExptScheduleEvent) error {
		return nil
	})
	assert.NoError(t, mw(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}))
}

// SandboxAgentHourlyNotify: NotifyProgressIfDue 返回错误时仅 log, middleware 返回 nil。
func TestSandboxAgentHourlyNotify_NotifierError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)
	manager := svcmocks.NewMockIExptManager(ctrl)
	manager.EXPECT().GetDetail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{}, nil)
	notifier.EXPECT().NotifyProgressIfDue(gomock.Any(), gomock.Any()).Return(errors.New("lark 500"))

	impl := &ExptSchedulerImpl{Manager: manager, sandboxAgentNotifier: notifier}
	mw := impl.SandboxAgentHourlyNotify(func(_ context.Context, _ *entity.ExptScheduleEvent) error {
		return nil
	})
	assert.NoError(t, mw(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}))
}

// NewExptSchedulerSvc variadic notifier: 传入时保存, 不传时字段为 nil。
func TestNewExptSchedulerSvc_VariadicNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	notifier := svcmocks.NewMockISandboxAgentNotifier(ctrl)

	svc := NewExptSchedulerSvc(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, notifier)
	impl, ok := svc.(*ExptSchedulerImpl)
	assert.True(t, ok)
	assert.Same(t, notifier, impl.sandboxAgentNotifier)

	svc2 := NewExptSchedulerSvc(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	impl2 := svc2.(*ExptSchedulerImpl)
	assert.Nil(t, impl2.sandboxAgentNotifier)
}
