// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// TestUpdateExptRunConf_ReInitsSandboxTaskOnConcurrencyChange 钉住「运行中调大并发后，沙箱任务的
// 名额必须同步」这条契约。
//
// 沙箱任务的并发上限只在 Init 时设定 —— SandboxScheduler 的 IDL 里 concurrency 只存在于
// InitRequest，没有 Update/Scale 方法。所以只改实验的 ItemConcurNum 而不 re-Init 会造成两侧分裂：
// evaluation 按新并发申请沙箱，agent_studio 名额还是旧值，超出部分全部 601300702
// concurrency limit reached，且该错误 fail-fast、不入队不重试 → item 永久失败。
//
// 实测：59 题实验运行中把并发从 5 调到 30，35 题以此挂掉。
func TestUpdateExptRunConf_ReInitsSandboxTaskOnConcurrencyChange(t *testing.T) {
	const (
		workspaceID = int64(123)
		exptID      = int64(456)
		userID      = "789"
		maxConcur   = 200
	)
	execConf := &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{MaxItemConcurNum: maxConcur}}

	// 双沙箱 SandboxAgent 实验：每 item 占 2 个 execution 名额，故 Init 要按 2 倍再乘余量系数。
	dualSandboxExpt := func() *entity.Experiment {
		return &entity.Experiment{
			ID: exptID, SpaceID: workspaceID, Status: entity.ExptStatus_Processing, CreatedBy: userID,
			TargetType: entity.EvalTargetTypeSandboxAgent,
			Target: &entity.EvalTarget{
				EvalTargetType: entity.EvalTargetTypeSandboxAgent,
				EvalTargetVersion: &entity.EvalTargetVersion{
					EvalTargetType: entity.EvalTargetTypeSandboxAgent,
					SandboxAgent:   &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeDual},
				},
			},
		}
	}

	t.Run("并发度变更 → 必须按新并发 re-Init 沙箱任务", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockManager := servicemocks.NewMockIExptManager(ctrl)
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockConfiger := componentMocks.NewMockIConfiger(ctrl)
		mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)

		expt := dualSandboxExpt()
		mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, &entity.Session{}).Return(expt, nil)
		mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
		mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), workspaceID).Return(execConf).AnyTimes()
		mockManager.EXPECT().UpdateRunConf(gomock.Any(), gomock.Any()).Return(nil)

		wantConcurrency := sandboxInitConcurrency(gptr.Of(30), true)
		var gotReq *rpc.SandboxInitRequest
		mockSched.EXPECT().Init(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
				gotReq = req
				return &rpc.SandboxInitResponse{}, nil
			}).Times(1)

		app := &experimentApplication{manager: mockManager, auth: mockAuth, configer: mockConfiger, sandboxSchedulerAdapter: mockSched}
		_, err := app.UpdateExptRunConf(context.Background(), &exptpb.UpdateExptRunConfRequest{
			ExptID: exptID, WorkspaceID: workspaceID, ItemConcurNum: gptr.Of(int32(30)),
		})
		assert.NoError(t, err)
		if assert.NotNil(t, gotReq, "并发度改了却没有 re-Init：沙箱名额会停在旧值, 超出部分全部 601300702") {
			assert.Equal(t, wantConcurrency, gotReq.Concurrency,
				"Init 必须按新并发下发（双沙箱 2 倍 + 余量系数）")
			// dualSandboxExpt() 不带 RunModeConfig → 旧链路租户；两个双沙箱租户都要并发 ×2。
			assert.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, gotReq.Tenant)
			assert.Equal(t, workspaceID, gotReq.WorkspaceID)
		}
	})

	t.Run("只改重试数 → 不该动沙箱任务", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockManager := servicemocks.NewMockIExptManager(ctrl)
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockConfiger := componentMocks.NewMockIConfiger(ctrl)
		mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)

		mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, &entity.Session{}).Return(dualSandboxExpt(), nil)
		mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
		mockManager.EXPECT().UpdateRunConf(gomock.Any(), gomock.Any()).Return(nil)
		// Init 一次都不该发生：名额与重试数无关，白发一次 Init 是多余的写操作。
		mockSched.EXPECT().Init(gomock.Any(), gomock.Any()).Times(0)

		app := &experimentApplication{manager: mockManager, auth: mockAuth, configer: mockConfiger, sandboxSchedulerAdapter: mockSched}
		_, err := app.UpdateExptRunConf(context.Background(), &exptpb.UpdateExptRunConfRequest{
			ExptID: exptID, WorkspaceID: workspaceID, ItemRetryNum: gptr.Of(int32(2)),
		})
		assert.NoError(t, err)
	})

	t.Run("非 SandboxAgent 实验 → 不该动沙箱任务", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockManager := servicemocks.NewMockIExptManager(ctrl)
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockConfiger := componentMocks.NewMockIConfiger(ctrl)
		mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)

		plain := &entity.Experiment{ID: exptID, SpaceID: workspaceID, Status: entity.ExptStatus_Processing, CreatedBy: userID}
		mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, &entity.Session{}).Return(plain, nil)
		mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
		mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), workspaceID).Return(execConf).AnyTimes()
		mockManager.EXPECT().UpdateRunConf(gomock.Any(), gomock.Any()).Return(nil)
		mockSched.EXPECT().Init(gomock.Any(), gomock.Any()).Times(0)

		app := &experimentApplication{manager: mockManager, auth: mockAuth, configer: mockConfiger, sandboxSchedulerAdapter: mockSched}
		_, err := app.UpdateExptRunConf(context.Background(), &exptpb.UpdateExptRunConfRequest{
			ExptID: exptID, WorkspaceID: workspaceID, ItemConcurNum: gptr.Of(int32(30)),
		})
		assert.NoError(t, err)
	})

	// Init 失败不该让整个更新失败: DB 里的 ItemConcurNum 已经改完, 报错会让调用方以为没生效而重试。
	// 名额没跟上只是新并发暂时吃不满(老名额仍可用, 实验不中断), 故降级为告警。
	t.Run("re-Init 失败 → 降级告警, 不让更新失败", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockManager := servicemocks.NewMockIExptManager(ctrl)
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockConfiger := componentMocks.NewMockIConfiger(ctrl)
		mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)

		mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, &entity.Session{}).Return(dualSandboxExpt(), nil)
		mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
		mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), workspaceID).Return(execConf).AnyTimes()
		mockManager.EXPECT().UpdateRunConf(gomock.Any(), gomock.Any()).Return(nil)
		mockSched.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil, errors.New("scheduler down")).Times(1)

		app := &experimentApplication{manager: mockManager, auth: mockAuth, configer: mockConfiger, sandboxSchedulerAdapter: mockSched}
		_, err := app.UpdateExptRunConf(context.Background(), &exptpb.UpdateExptRunConfRequest{
			ExptID: exptID, WorkspaceID: workspaceID, ItemConcurNum: gptr.Of(int32(30)),
		})
		assert.NoError(t, err, "Init 失败不该让 UpdateExptRunConf 失败")
	})
}

// TestInitSandboxTask 覆盖 initSandboxTask 的三条路径：
//   - nil adapter：直接返回 nil，不触发任何 RPC；
//   - GUI 租户（mac_vm_plus_sandbox）：Init 两个 task —— 基础 taskID(Sandbox) + taskID+"-macvm"(MacVM)，
//     两者共用同一租户/并发；
//   - GUI 租户下任一 Init 失败：整体返回 error。
func TestInitSandboxTask(t *testing.T) {
	const (
		exptID      int64 = 42
		workspaceID int64 = 7
		concurrency int32 = 5
	)

	t.Run("nil adapter 不触发 RPC", func(t *testing.T) {
		app := &experimentApplication{}
		err := app.initSandboxTask(context.Background(), "test", exptID, concurrency, workspaceID, rpc.SandboxTenantFornaxEvalGeneralGUI)
		assert.NoError(t, err)
	})

	t.Run("GUI 租户 Init 两个 task（sandbox + -macvm）", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)

		got := map[string]rpc.SandboxResourceType{}
		mockSched.EXPECT().Init(gomock.Any(), gomock.Any()).Times(2).
			DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
				got[req.TaskID] = req.ResourceType
				// 两个 task 必须共用 GUI 租户与同一并发
				assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, req.Tenant)
				assert.Equal(t, concurrency, req.Concurrency)
				assert.Equal(t, workspaceID, req.WorkspaceID)
				return &rpc.SandboxInitResponse{}, nil
			})

		app := &experimentApplication{sandboxSchedulerAdapter: mockSched}
		err := app.initSandboxTask(context.Background(), "test", exptID, concurrency, workspaceID, rpc.SandboxTenantFornaxEvalGeneralGUI)
		assert.NoError(t, err)
		assert.Equal(t, rpc.SandboxResourceTypeSandbox, got["42"], "基础 taskID 落 Sandbox")
		assert.Equal(t, rpc.SandboxResourceTypeMacVM, got["42-macvm"], "-macvm taskID 落 MacVM")
	})

	t.Run("GUI 租户任一 Init 失败即返回 error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)

		mockSched.EXPECT().Init(gomock.Any(), gomock.Any()).Return(nil, errors.New("scheduler down")).Times(1)

		app := &experimentApplication{sandboxSchedulerAdapter: mockSched}
		err := app.initSandboxTask(context.Background(), "test", exptID, concurrency, workspaceID, rpc.SandboxTenantFornaxEvalGeneralGUI)
		assert.Error(t, err)
	})
}
