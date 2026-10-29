// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"
	"time"

	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSandboxEarlyCallbackUsesPreRegisteredRedisContext(t *testing.T) {
	ctx := context.Background()
	repository := experimentrepo.NewEvalAsyncRepo(dao.NewEvalAsyncDAO(infraredis.NewTestRedis(t), nil))
	ctrl := gomock.NewController(t)
	target := servicemocks.NewMockIEvalTargetService(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	config := configmocks.NewMockIConfiger(ctrl)
	app := &EvalOpenAPIApplication{asyncRepo: repository, targetSvc: target, publisher: publisher, configer: config}
	request := newFailedInvokeResultReq(2, 707, "runtime preflight failed")
	// This is the actual application callback entry, not ReportInvokeRecords alone.
	_, err := app.ReportEvalTargetInvokeResult_(ctx, request)
	require.Error(t, err, "without pre-registration the former initialization race reproduces")
	event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 21, ExptRunID: 22, EvalSetItemID: 23}
	prepared := &entity.EvalAsyncCtx{RecordID: 707, Event: event, Callee: "sandbox_agent", AsyncUnixMS: time.Now().Add(-time.Second).UnixMilli()}
	require.NoError(t, service.PrepareSandboxAsyncContext(ctx, repository, prepared))
	target.EXPECT().GetRecordByID(gomock.Any(), int64(2), int64(707)).Return(&entity.EvalTargetRecord{ID: 707, TurnID: 0}, nil)
	target.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ReportTargetRecordParam) error {
		require.Equal(t, int64(2), p.SpaceID)
		require.Equal(t, int64(707), p.RecordID)
		require.Equal(t, prepared.AsyncUnixMS, p.AsyncUnixMS)
		require.True(t, p.SkipErrMsgConvert)
		require.Equal(t, entity.EvalTargetRunStatusFail, p.Status)
		return nil
	})
	config.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(&entity.TargetTrajectoryConf{})
	publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.ExptItemEvalEvent, _ *time.Duration, _ ...func(*entity.ExptItemEvalEvent)) error {
		require.Equal(t, int64(3), e.SpaceID)
		require.Equal(t, int64(21), e.ExptID)
		require.Equal(t, int64(22), e.ExptRunID)
		require.Equal(t, int64(23), e.EvalSetItemID)
		assertCallbackResumeSource(t, e, "target", 707, 0)
		require.NotSame(t, event, e)
		return nil
	})
	response, err := app.ReportEvalTargetInvokeResult_(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Zero(t, response.BaseResp.StatusCode)
	// Re-entry after the early result must not reset callback state or elapsed time.
	_, err = repository.MarkEvalAsyncResumeReady(ctx, "707")
	require.NoError(t, err)
	retry := *prepared
	retry.AsyncUnixMS = time.Now().UnixMilli()
	require.NoError(t, service.PrepareSandboxAsyncContext(ctx, repository, &retry))
	actual, err := repository.GetEvalAsyncCtx(ctx, "707")
	require.NoError(t, err)
	require.Equal(t, prepared.AsyncUnixMS, actual.AsyncUnixMS)
	require.True(t, actual.ResumeReady)
	require.Nil(t, actual.Event.ResumeSource)
	for _, field := range []string{"space", "experiment", "run", "item", "record", "callee"} {
		t.Run(field, func(t *testing.T) {
			wrong := *prepared
			owner := *prepared.Event
			wrong.Event = &owner
			switch field {
			case "space":
				owner.SpaceID++
			case "experiment":
				owner.ExptID++
			case "run":
				owner.ExptRunID++
			case "item":
				owner.EvalSetItemID++
			case "record":
				// Same Redis key but corrupt stored record identity must fail closed.
				corrupted := *prepared
				corrupted.RecordID = 999
				require.NoError(t, repository.SetEvalAsyncCtx(ctx, "707", &corrupted))
				defer func() { require.NoError(t, repository.SetEvalAsyncCtx(ctx, "707", prepared)) }()
			case "callee":
				wrong.Callee = "other"
			}
			require.ErrorContains(t, service.PrepareSandboxAsyncContext(ctx, repository, &wrong), "owner mismatch")
		})
	}
}
