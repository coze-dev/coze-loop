// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func newSandboxAgentSvc(t *testing.T) (*SandboxAgentSourceEvalTargetServiceImpl, *idgenmocks.MockIIDGenerator, *rpcmocks.MockISandboxSchedulerAdapter, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
	svc := NewSandboxAgentSourceEvalTargetServiceImpl(mockIdgen, mockSched, nil).(*SandboxAgentSourceEvalTargetServiceImpl)
	return svc, mockIdgen, mockSched, ctrl
}

func TestSandboxAgentSourceEvalTargetServiceImpl_EvalType(t *testing.T) {
	svc, _, _, ctrl := newSandboxAgentSvc(t)
	defer ctrl.Finish()
	assert.Equal(t, entity.EvalTargetTypeSandboxAgent, svc.EvalType())
}

func TestSandboxAgentSourceEvalTargetServiceImpl_RuntimeParam(t *testing.T) {
	svc, _, _, ctrl := newSandboxAgentSvc(t)
	defer ctrl.Finish()
	rp := svc.RuntimeParam()
	assert.NotNil(t, rp)
	assert.IsType(t, &entity.GenericJSONRuntimeParam{}, rp)
	assert.Equal(t, "{}", rp.GetJSONDemo())
}

func TestSandboxAgentSourceEvalTargetServiceImpl_ValidateInput(t *testing.T) {
	svc, _, _, ctrl := newSandboxAgentSvc(t)
	defer ctrl.Finish()

	t.Run("input 为 nil 直接通过", func(t *testing.T) {
		assert.NoError(t, svc.ValidateInput(context.Background(), 1, nil, nil))
	})

	t.Run("input 非 nil 走 ValidateInputSchema", func(t *testing.T) {
		// 空 schema + 空 input 应通过
		input := &entity.EvalTargetInputData{InputFields: map[string]*entity.Content{}}
		err := svc.ValidateInput(context.Background(), 1, nil, input)
		assert.NoError(t, err)
	})
}

func TestSandboxAgentSourceEvalTargetServiceImpl_Execute(t *testing.T) {
	svc, _, _, ctrl := newSandboxAgentSvc(t)
	defer ctrl.Finish()

	// SandboxAgent 仅支持 async 执行, Execute 必须报错
	output, status, err := svc.Execute(context.Background(), 1, &entity.ExecuteEvalTargetParam{})
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Equal(t, entity.EvalTargetRunStatusFail, status)
}

func TestSandboxAgentSourceEvalTargetServiceImpl_AsyncExecute(t *testing.T) {
	t.Run("GenID 成功返回 invokeID 与 callee", func(t *testing.T) {
		svc, mockIdgen, _, ctrl := newSandboxAgentSvc(t)
		defer ctrl.Finish()

		mockIdgen.EXPECT().GenID(gomock.Any()).Return(int64(12345), nil)

		id, callee, ext, err := svc.AsyncExecute(context.Background(), 1, &entity.ExecuteEvalTargetParam{})
		assert.NoError(t, err)
		assert.Equal(t, int64(12345), id)
		assert.Equal(t, "sandbox_agent", callee)
		assert.Nil(t, ext)
	})

	t.Run("GenID 失败返回错误", func(t *testing.T) {
		svc, mockIdgen, _, ctrl := newSandboxAgentSvc(t)
		defer ctrl.Finish()

		mockIdgen.EXPECT().GenID(gomock.Any()).Return(int64(0), errors.New("gen id fail"))

		id, callee, ext, err := svc.AsyncExecute(context.Background(), 1, &entity.ExecuteEvalTargetParam{})
		assert.Error(t, err)
		assert.Equal(t, int64(0), id)
		assert.Equal(t, "", callee)
		assert.Nil(t, ext)
	})
}

func TestSandboxAgentSourceEvalTargetServiceImpl_BuildBySource(t *testing.T) {
	t.Run("缺少 SandboxAgent 配置返回错误", func(t *testing.T) {
		svc, _, _, ctrl := newSandboxAgentSvc(t)
		defer ctrl.Finish()

		got, err := svc.BuildBySource(context.Background(), 1, "src-id", "v1")
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("有 SandboxAgent 配置构建 EvalTarget", func(t *testing.T) {
		svc, _, _, ctrl := newSandboxAgentSvc(t)
		defer ctrl.Finish()

		sa := &entity.SandboxAgent{
			Name:          "demo",
			ModelName:     "doubao",
			AgentSetupCmd: "setup.sh",
			AgentRunCmd:   "run.sh",
			Envs:          []*entity.SandboxEnvVar{{Key: "K", Value: "V"}},
		}
		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user-1"})

		got, err := svc.BuildBySource(ctx, 100, "src-id", "v1", entity.WithSandboxAgent(sa))
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, int64(100), got.SpaceID)
		assert.Equal(t, "src-id", got.SourceTargetID)
		assert.Equal(t, entity.EvalTargetTypeSandboxAgent, got.EvalTargetType)
		// version
		assert.NotNil(t, got.EvalTargetVersion)
		assert.Equal(t, int64(100), got.EvalTargetVersion.SpaceID)
		assert.Equal(t, "v1", got.EvalTargetVersion.SourceTargetVersion)
		assert.Equal(t, entity.EvalTargetTypeSandboxAgent, got.EvalTargetVersion.EvalTargetType)
		assert.Equal(t, sa, got.EvalTargetVersion.SandboxAgent)
		assert.Equal(t, gptr.Of("{}"), got.EvalTargetVersion.RuntimeParamDemo)
		// BaseInfo 用 ctx 中的 user
		assert.NotNil(t, got.BaseInfo)
		assert.NotNil(t, got.BaseInfo.CreatedBy)
		assert.Equal(t, "user-1", gptr.Indirect(got.BaseInfo.CreatedBy.UserID))
		assert.NotNil(t, got.EvalTargetVersion.BaseInfo)
		assert.Equal(t, "user-1", gptr.Indirect(got.EvalTargetVersion.BaseInfo.CreatedBy.UserID))
	})
}

// nop 方法只需保证 contract: 不 panic, 返回零值/nil error
func TestSandboxAgentSourceEvalTargetServiceImpl_NopMethods(t *testing.T) {
	svc, _, _, ctrl := newSandboxAgentSvc(t)
	defer ctrl.Finish()
	ctx := context.Background()

	t.Run("ListSource", func(t *testing.T) {
		dos, cursor, hasMore, err := svc.ListSource(ctx, &entity.ListSourceParam{})
		assert.NoError(t, err)
		assert.Nil(t, dos)
		assert.Equal(t, "", cursor)
		assert.False(t, hasMore)
	})
	t.Run("BatchGetSource", func(t *testing.T) {
		dos, err := svc.BatchGetSource(ctx, 1, []string{"a"})
		assert.NoError(t, err)
		assert.Nil(t, dos)
	})
	t.Run("ListSourceVersion", func(t *testing.T) {
		dos, cursor, hasMore, err := svc.ListSourceVersion(ctx, &entity.ListSourceVersionParam{})
		assert.NoError(t, err)
		assert.Nil(t, dos)
		assert.Equal(t, "", cursor)
		assert.False(t, hasMore)
	})
	t.Run("PackSourceInfo / PackSourceVersionInfo", func(t *testing.T) {
		assert.NoError(t, svc.PackSourceInfo(ctx, 1, nil))
		assert.NoError(t, svc.PackSourceVersionInfo(ctx, 1, nil))
	})
	t.Run("SearchCustomEvalTarget", func(t *testing.T) {
		dos, cursor, hasMore, err := svc.SearchCustomEvalTarget(ctx, &entity.SearchCustomEvalTargetParam{})
		assert.NoError(t, err)
		assert.Nil(t, dos)
		assert.Equal(t, "", cursor)
		assert.False(t, hasMore)
	})
}
