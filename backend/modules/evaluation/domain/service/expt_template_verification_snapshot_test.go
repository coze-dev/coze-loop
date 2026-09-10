// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestExptTemplateManagerImpl_VerificationTargetSnapshot(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	agent := &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeMacVMPlusSSH}
	targetSvc.EXPECT().CreateEvalTarget(ctx, int64(42), entity.BuiltinVerificationTargetID, "", entity.EvalTargetTypeSandboxAgent, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, _, _ string, _ entity.EvalTargetType, opts ...entity.Option) (int64, int64, error) {
			opt := new(entity.Opt)
			for _, f := range opts {
				f(opt)
			}
			require.Same(t, agent, opt.SandboxAgent)
			return 8, 9, nil
		})
	mgr := &ExptTemplateManagerImpl{evalTargetService: targetSvc}
	id, version, targetType, err := mgr.resolveTargetForCreate(ctx, &entity.CreateExptTemplateParam{SpaceID: 42, CreateEvalTargetParam: &entity.CreateEvalTargetParam{
		SourceTargetID: gptr.Of(entity.BuiltinVerificationTargetID), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent), SandboxAgent: agent,
	}})
	require.NoError(t, err)
	require.Equal(t, int64(8), id)
	require.Equal(t, int64(9), version)
	require.Equal(t, entity.EvalTargetTypeSandboxAgent, targetType)
}
