// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestWindowsSandboxTopology(t *testing.T) {
	for _, tc := range []struct {
		name, task string
		mode       entity.SandboxCountMode
		want       map[string]rpc.SandboxResourceType
	}{
		{"experiment Windows", "42", "windows_plus_sandbox", map[string]rpc.SandboxResourceType{"42": 0, "42-windows": 3}},
		{"Windows overrides legacy GUI tenant", "42", "windows_plus_sandbox", map[string]rpc.SandboxResourceType{"42": 0, "42-windows": 3}},
		{"debug Windows", "sandbox_debug", "windows_plus_sandbox", map[string]rpc.SandboxResourceType{"sandbox_debug": 0, "sandbox_debug-windows": 3}},
		{"general tenant dual stays Linux", "42", entity.SandboxCountModeDual, map[string]rpc.SandboxResourceType{"42": 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sched := rpcmocks.NewMockISandboxSchedulerAdapter(gomock.NewController(t))
			got := map[string]rpc.SandboxResourceType{}
			sched.EXPECT().Init(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
				got[req.TaskID] = req.ResourceType
				require.Equal(t, rpc.SandboxTenant(4), req.Tenant)
				wantConcurrency := int32(15)
				if tc.mode == entity.SandboxCountModeDual {
					wantConcurrency = 30
				}
				require.Equal(t, wantConcurrency, req.Concurrency)
				return &rpc.SandboxInitResponse{}, nil
			})
			tenant := rpc.SandboxTenantFornaxEvalGeneral
			if tc.name == "Windows overrides legacy GUI tenant" {
				tenant = rpc.SandboxTenantFornaxEvalGeneralGUI
			}
			err := initSandboxTasksByTaskID(context.Background(), sched, "test", tc.task, sandboxTaskConcurrencyForMode(gptr.Of(3), tc.mode), 7, tenant)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestWindowsSandboxTenantFromTarget(t *testing.T) {
	expt := &entity.Experiment{Target: &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{SandboxCountMode: "windows_plus_sandbox"}}}}
	require.Equal(t, rpc.SandboxTenant(4), sandboxTenantForExperimentEntity(expt))
	dto := &domain_expt.Experiment{EvalTarget: &domain_eval_target.EvalTarget{EvalTargetVersion: &domain_eval_target.EvalTargetVersion{EvalTargetContent: &domain_eval_target.EvalTargetContent{SandboxAgent: &domain_eval_target.SandboxAgent{SandboxCountMode: gptr.Of(domain_eval_target.SandboxCountMode("windows_plus_sandbox"))}}}}}
	require.Equal(t, rpc.SandboxTenant(4), sandboxTenantForExperimentDTO(dto))
}

func TestWindowsDebugDestroyUsesWindowsTask(t *testing.T) {
	sched := rpcmocks.NewMockISandboxSchedulerAdapter(gomock.NewController(t))
	got := map[string][]string{}
	sched.EXPECT().Destroy(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(ctx context.Context, req *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
		require.NoError(t, ctx.Err())
		got[req.TaskID] = req.ExecuteIDs
		return &rpc.SandboxDestroyResponse{}, nil
	})
	app := &EvalOpenAPIApplication{sandboxSchedulerAdapter: sched}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.destroyDebugSandboxExecutions(ctx, 7, 42, []string{"42-orch", "42-windows"})
	require.Equal(t, map[string][]string{"sandbox_debug": {"42-orch"}, "sandbox_debug-windows": {"42-windows"}}, got)
}

func TestWindowsReinitLoadsTargetTopology(t *testing.T) {
	ctrl := gomock.NewController(t)
	sched := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(8), int64(99), false).Return(&entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{SandboxCountMode: "windows_plus_sandbox"}}}, nil)
	got := map[string]rpc.SandboxInitRequest{}
	sched.EXPECT().Init(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
		got[req.TaskID] = *req
		return &rpc.SandboxInitResponse{}, nil
	})
	expt := &entity.Experiment{ID: 42, TargetType: entity.EvalTargetTypeSandboxAgent, TargetSpaceID: 8, TargetVersionID: 99}
	err := reinitSandboxTaskForExperiment(context.Background(), "retry experiment", expt, gptr.Of(3), 7, targetSvc, sched)
	require.NoError(t, err)
	require.Equal(t, map[string]rpc.SandboxInitRequest{
		"42":         {TaskID: "42", WorkspaceID: 7, Tenant: 4, ResourceType: 0, Concurrency: 15},
		"42-windows": {TaskID: "42-windows", WorkspaceID: 7, Tenant: 4, ResourceType: 3, Concurrency: 15},
	}, got)
}
