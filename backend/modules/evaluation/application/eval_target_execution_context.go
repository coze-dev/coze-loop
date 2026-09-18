// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func (e *EvalOpenAPIApplication) GetEvalTargetExecutionContextOApi(ctx context.Context, req *openapi.GetEvalTargetExecutionContextOApiRequest) (*openapi.GetEvalTargetExecutionContextOApiResponse, error) {
	if req == nil || req.GetWorkspaceID() <= 0 || req.GetEvalTargetRecordID() <= 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	if err := e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID: strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:  req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{
			Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Space),
		}},
	}); err != nil {
		return nil, err
	}
	execution, err := e.targetSvc.GetExecutionContext(ctx, req.GetWorkspaceID(), req.GetEvalTargetRecordID())
	if err != nil {
		return nil, err
	}
	return &openapi.GetEvalTargetExecutionContextOApiResponse{
		Data: &openapi.GetEvalTargetExecutionContextOpenAPIData{
			WorkspaceID:           gptr.Of(execution.WorkspaceID),
			EvalTargetRecordID:    gptr.Of(execution.EvalTargetRecordID),
			ExperimentID:          gptr.Of(execution.ExperimentID),
			ExperimentRunID:       gptr.Of(execution.ExperimentRunID),
			ExperimentWorkspaceID: gptr.Of(execution.ExperimentWorkspaceID),
			InitiatorUserID:       gptr.Of(execution.InitiatorUserID),
		},
		BaseResp: base.NewBaseResp(),
	}, nil
}
