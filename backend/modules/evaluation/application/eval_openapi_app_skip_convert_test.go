// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	configermocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// sandbox_agent async 回调 (Callee == sandboxAgentAsyncCallee) 时,
// 传递到 targetSvc.ReportInvokeRecords 的 param.SkipErrMsgConvert 应为 true。
func TestReportEvalTargetInvokeResult_SandboxCallee_SetsSkipErrMsgConvert(t *testing.T) {
	t.Parallel()

	req := newFailedInvokeResultReq(77, 707, "sandbox invoke failed")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	configer := configermocks.NewMockIConfiger(ctrl)

	event := &entity.ExptItemEvalEvent{}
	actx := &entity.EvalAsyncCtx{
		AsyncUnixMS: time.Now().Add(-50 * time.Millisecond).UnixMilli(),
		Event:       event,
		Callee:      sandboxAgentAsyncCallee, // ★ 关键
	}
	asyncRepo.EXPECT().
		GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(req.GetInvokeID(), 10)).
		Return(actx, nil)
	targetSvc.EXPECT().
		ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).
		DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
			assert.True(t, param.SkipErrMsgConvert, "sandbox callee should skip err msg convert")
			return nil
		})
	configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(&entity.TargetTrajectoryConf{})
	publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	app := &EvalOpenAPIApplication{
		targetSvc: targetSvc,
		asyncRepo: asyncRepo,
		publisher: publisher,
		configer:  configer,
	}
	resp, err := app.ReportEvalTargetInvokeResult_(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// 非 sandbox callee (Callee != sandboxAgentAsyncCallee) 时, SkipErrMsgConvert 应为 false。
func TestReportEvalTargetInvokeResult_NonSandboxCallee_KeepsConvert(t *testing.T) {
	t.Parallel()

	req := newFailedInvokeResultReq(88, 808, "generic async invoke failed")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	configer := configermocks.NewMockIConfiger(ctrl)

	event := &entity.ExptItemEvalEvent{}
	actx := &entity.EvalAsyncCtx{
		AsyncUnixMS: time.Now().Add(-50 * time.Millisecond).UnixMilli(),
		Event:       event,
		Callee:      "some_other_callee",
	}
	asyncRepo.EXPECT().
		GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(req.GetInvokeID(), 10)).
		Return(actx, nil)
	targetSvc.EXPECT().
		ReportInvokeRecords(gomock.Any(), gomock.AssignableToTypeOf(&entity.ReportTargetRecordParam{})).
		DoAndReturn(func(_ context.Context, param *entity.ReportTargetRecordParam) error {
			assert.False(t, param.SkipErrMsgConvert, "non-sandbox callee should not skip err msg convert")
			return nil
		})
	configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(&entity.TargetTrajectoryConf{})
	publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	app := &EvalOpenAPIApplication{
		targetSvc: targetSvc,
		asyncRepo: asyncRepo,
		publisher: publisher,
		configer:  configer,
	}
	resp, err := app.ReportEvalTargetInvokeResult_(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}
