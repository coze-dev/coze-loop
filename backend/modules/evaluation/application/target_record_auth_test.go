// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	evaltargetapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTargetRecordAuthorizationUsesExecutionOwner(t *testing.T) {
	record := &entity.EvalTargetRecord{TargetID: 20}
	record.BindExecutionSpace(10, 99)
	permission := targetRecordAuthorization(record)
	require.Equal(t, int64(99), permission.SpaceID)
	require.Equal(t, "99", permission.ObjectID)
	require.Equal(t, consts.ActionReadExpt, gptr.Indirect(permission.ActionObjects[0].Action))
	require.Equal(t, rpc.AuthEntityType_Space, gptr.Indirect(permission.ActionObjects[0].EntityType))
	record.BindExecutionSpace(10, 0)
	permission = targetRecordAuthorization(record)
	require.Equal(t, int64(10), permission.SpaceID)
	require.Equal(t, "20", permission.ObjectID)
	require.Equal(t, consts.Read, gptr.Indirect(permission.ActionObjects[0].Action))
	require.Equal(t, rpc.AuthEntityType_EvaluationTarget, gptr.Indirect(permission.ActionObjects[0].EntityType))
}

func TestExperimentRecordReadAuthorization(t *testing.T) {
	for _, mode := range []string{"allowed", "denied", "wrong-workspace", "foreign-record"} {
		t.Run(mode, func(t *testing.T) {
			controller := gomock.NewController(t)
			reader := servicemocks.NewMockIEvalTargetService(controller)
			authorizer := rpcmocks.NewMockIAuthProvider(controller)
			application := &EvalTargetApplicationImpl{evalTargetService: reader, auth: authorizer}
			record := &entity.EvalTargetRecord{ID: 5, TargetID: 8, BaseInfo: &entity.BaseInfo{}}
			record.BindExecutionSpace(10, 99)
			request := &evaltargetapi.GetEvalTargetRecordRequest{WorkspaceID: 99, EvalTargetRecordID: 5}
			switch mode {
			case "wrong-workspace":
				request.WorkspaceID = 10
				reader.EXPECT().GetRecordByID(gomock.Any(), int64(10), int64(5)).Return(nil, nil)
			case "foreign-record":
				request.WorkspaceID = 10
				reader.EXPECT().GetRecordByID(gomock.Any(), int64(10), int64(5)).Return(record, nil)
			default:
				reader.EXPECT().GetRecordByID(gomock.Any(), int64(99), int64(5)).Return(record, nil)
				var permissionError error
				if mode == "denied" {
					permissionError = errorx.NewByCode(errno.CommonNoPermissionCode)
				}
				authorizer.EXPECT().Authorization(gomock.Any(), targetRecordAuthorization(record)).Return(permissionError)
			}
			response, err := application.GetEvalTargetRecord(context.Background(), request)
			if mode == "denied" || mode == "foreign-record" {
				require.Error(t, err)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			if mode == "wrong-workspace" {
				require.Nil(t, response.EvalTargetRecord)
			} else {
				require.NotNil(t, response.EvalTargetRecord)
			}
		})
	}
}

func TestExperimentRecordBatchAuthorization(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "allowed", true: "denied"}[denied], func(t *testing.T) {
			controller := gomock.NewController(t)
			reader := servicemocks.NewMockIEvalTargetService(controller)
			authorizer := rpcmocks.NewMockIAuthProvider(controller)
			application := &EvalTargetApplicationImpl{evalTargetService: reader, auth: authorizer}
			record := &entity.EvalTargetRecord{ID: 5, TargetID: 8, BaseInfo: &entity.BaseInfo{}}
			record.BindExecutionSpace(10, 99)
			reader.EXPECT().BatchGetRecordByIDs(gomock.Any(), int64(99), []int64{5}).Return([]*entity.EvalTargetRecord{record}, nil)
			var permissionError error
			if denied {
				permissionError = errorx.NewByCode(errno.CommonNoPermissionCode)
			}
			authorizer.EXPECT().Authorization(gomock.Any(), targetRecordAuthorization(record)).Return(permissionError)
			response, err := application.BatchGetEvalTargetRecords(context.Background(), &evaltargetapi.BatchGetEvalTargetRecordsRequest{WorkspaceID: 99, EvalTargetRecordIds: []int64{5}})
			if denied {
				require.Error(t, err)
				require.Nil(t, response)
			} else {
				require.NoError(t, err)
				require.Len(t, response.EvalTargetRecords, 1)
			}
		})
	}
}
