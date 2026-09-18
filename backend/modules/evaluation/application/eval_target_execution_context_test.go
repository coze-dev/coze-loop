// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
)

type executionContextAuth struct {
	rpc.IAuthProvider
	authorized bool
	calls      *[]string
}

func (a executionContextAuth) Authorization(_ context.Context, p *rpc.AuthorizationParam) error {
	*a.calls = append(*a.calls, "auth")
	if !a.authorized || p.SpaceID != 1 {
		return errorx.NewByCode(errno.CommonNoPermissionCode)
	}
	return nil
}

type executionContextService struct {
	service.IEvalTargetService
	calls *[]string
}

func (s executionContextService) GetExecutionContext(_ context.Context, spaceID, recordID int64) (*entity.EvalTargetExecutionContext, error) {
	*s.calls = append(*s.calls, "read")
	return &entity.EvalTargetExecutionContext{WorkspaceID: spaceID, EvalTargetRecordID: recordID, ExperimentID: 10, ExperimentRunID: 21, ExperimentWorkspaceID: 2, InitiatorUserID: "initiator-b"}, nil
}

func TestExecutionContextOApiAuthorizesBeforeReading(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "denied", true: "allowed"}[allowed], func(t *testing.T) {
			calls := []string{}
			app := &EvalOpenAPIApplication{auth: executionContextAuth{authorized: allowed, calls: &calls}, targetSvc: executionContextService{calls: &calls}}
			got, err := app.GetEvalTargetExecutionContextOApi(context.Background(), &openapi.GetEvalTargetExecutionContextOApiRequest{WorkspaceID: 1, EvalTargetRecordID: 31})
			if !allowed {
				require.Nil(t, got)
				require.ErrorContains(t, err, "601200101")
				require.Equal(t, []string{"auth"}, calls)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			require.NotNil(t, got.Data)
			require.Equal(t, []string{"auth", "read"}, calls)
			require.Equal(t, int64(1), got.Data.GetWorkspaceID())
			require.Equal(t, int64(31), got.Data.GetEvalTargetRecordID())
			require.Equal(t, int64(10), got.Data.GetExperimentID())
			require.Equal(t, int64(21), got.Data.GetExperimentRunID())
			require.Equal(t, int64(2), got.Data.GetExperimentWorkspaceID())
			require.Equal(t, "initiator-b", got.Data.GetInitiatorUserID())
		})
	}
}

func TestExecutionContextOApiRejectsInvalidCoordinates(t *testing.T) {
	for _, req := range []*openapi.GetEvalTargetExecutionContextOApiRequest{nil, {}, {WorkspaceID: -1, EvalTargetRecordID: 31}, {WorkspaceID: 1, EvalTargetRecordID: -1}, {WorkspaceID: 1}} {
		got, err := (&EvalOpenAPIApplication{}).GetEvalTargetExecutionContextOApi(context.Background(), req)
		require.Nil(t, got)
		require.ErrorContains(t, err, "601200202")
	}
}
