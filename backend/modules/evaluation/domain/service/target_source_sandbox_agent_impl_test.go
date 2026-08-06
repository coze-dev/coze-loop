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
	metricscomp "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
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
	t.Run("GetLatestSourceVersion", func(t *testing.T) {
		version, err := svc.GetLatestSourceVersion(ctx, 1, "a")
		assert.NoError(t, err)
		assert.Nil(t, version)
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

// TestParseInt64OrZero 覆盖 helper：空串、非数字、正常数字。
func TestParseInt64OrZero(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(0), parseInt64OrZero(""))
	assert.Equal(t, int64(0), parseInt64OrZero("not-a-number"))
	assert.Equal(t, int64(123), parseInt64OrZero("123"))
	assert.Equal(t, int64(-1), parseInt64OrZero("-1"))
}

// TestSandboxAgentSourceEvalTargetServiceImpl_emitInvokeStarted 覆盖提交侧打点：
// 1) metrics/param 缺失短路；2) ItemMeta/EvalSetItemID 缺失时零值；3) 完整 tags 透传。
func TestSandboxAgentSourceEvalTargetServiceImpl_emitInvokeStarted(t *testing.T) {
	t.Parallel()

	t.Run("nil metrics skips", func(t *testing.T) {
		svc, _, _, ctrl := newSandboxAgentSvc(t)
		defer ctrl.Finish()
		svc.sandboxAgentMetrics = nil
		// 不 panic 即通过
		svc.emitInvokeStarted(1, &entity.ExecuteEvalTargetParam{})
	})

	t.Run("nil param skips", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mock := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &SandboxAgentSourceEvalTargetServiceImpl{sandboxAgentMetrics: mock}
		svc.emitInvokeStarted(1, nil)
	})

	t.Run("param without ItemMeta emits with zero optional tags", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mock := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &SandboxAgentSourceEvalTargetServiceImpl{sandboxAgentMetrics: mock}
		mock.EXPECT().EmitInvokeStarted(gomock.Any()).Do(func(tags metricscomp.SandboxAgentInvokeTags) {
			assert.Equal(t, "42", tags.InvokeID)
			assert.Equal(t, int64(1001), tags.ExperimentID)
			assert.Equal(t, int64(2001), tags.TargetID)
			assert.Zero(t, tags.DatasetID)
			assert.Zero(t, tags.DatasetVersion)
			assert.Equal(t, "", tags.ItemKey)
			assert.Equal(t, "", tags.DatasetKey)
			assert.Zero(t, tags.ItemID)
		})
		svc.emitInvokeStarted(42, &entity.ExecuteEvalTargetParam{ExptID: 1001, TargetID: 2001})
	})

	t.Run("full ItemMeta populates all tags", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mock := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
		svc := &SandboxAgentSourceEvalTargetServiceImpl{sandboxAgentMetrics: mock}
		itemID := int64(9001)
		mock.EXPECT().EmitInvokeStarted(gomock.Any()).Do(func(tags metricscomp.SandboxAgentInvokeTags) {
			assert.Equal(t, "77", tags.InvokeID)
			assert.Equal(t, int64(1001), tags.ExperimentID)
			assert.Equal(t, int64(2001), tags.TargetID)
			assert.Equal(t, int64(3001), tags.DatasetID)
			assert.Equal(t, int64(4001), tags.DatasetVersion)
			assert.Equal(t, "ik", tags.ItemKey)
			assert.Equal(t, "dk", tags.DatasetKey)
			assert.Equal(t, itemID, tags.ItemID)
		})
		svc.emitInvokeStarted(77, &entity.ExecuteEvalTargetParam{
			ExptID:        1001,
			TargetID:      2001,
			EvalSetItemID: &itemID,
			ItemMeta: &entity.EvalSetItemMeta{
				EvalSetID:        "3001",
				EvalSetVersionID: "4001",
				ItemKey:          "ik",
				DatasetKey:       "dk",
			},
		})
	})
}
