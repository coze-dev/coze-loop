// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"github.com/bytedance/gg/gptr"
	im "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	dt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	de "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	ot "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	et "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	ep "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"testing"
)

func executionModeExperiment(mode entity.SandboxCountMode) *entity.Experiment {
	return &entity.Experiment{ID: 456, TargetType: entity.EvalTargetTypeSandboxAgent,
		Target: &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{
			EvalTargetType: entity.EvalTargetTypeSandboxAgent, SandboxAgent: &entity.SandboxAgent{SandboxCountMode: mode}}}}
}

func TestARMModeUsesGUITenant(t *testing.T) {
	mode := entity.SandboxCountMode("arm_mac_vm_plus_sandbox")
	expt := executionModeExperiment(mode)
	dto := &de.Experiment{EvalTarget: &dt.EvalTarget{EvalTargetVersion: &dt.EvalTargetVersion{
		EvalTargetContent: &dt.EvalTargetContent{SandboxAgent: &dt.SandboxAgent{SandboxCountMode: gptr.Of(dt.SandboxCountMode(mode))}}}}}
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForExperimentEntity(expt))
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForExperimentDTO(dto))
	assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, sandboxTenantForAsyncDebug(mode))
	assert.Equal(t, mode, sandboxCountModeForExperimentEntity(expt))
	assert.Equal(t, mode, sandboxCountModeForExperimentDTO(dto))
	assert.Equal(t, sandboxTaskConcurrencyForMode(gptr.Of(7), entity.SandboxCountModeMacVMPlusSandbox), sandboxTaskConcurrencyForMode(gptr.Of(7), mode))
}

func TestReinitWindowsRejectsBeforeResources(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := rm.NewMockISandboxSchedulerAdapter(ctrl)
	err := reinitSandboxTaskForExperiment(context.Background(), "retry", executionModeExperiment("windows_plus_sandbox"), nil, 123, nil, adapter)
	assert.ErrorContains(t, err, "unsupported execution mode")
}

func TestAsyncDebugWindowsRejectsBeforeResources(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rm.NewMockIAuthProvider(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	app := &EvalOpenAPIApplication{auth: auth, metric: &fakeOpenAPIMetric{}, sandboxSchedulerAdapter: rm.NewMockISandboxSchedulerAdapter(ctrl), targetSvc: sm.NewMockIEvalTargetService(ctrl)}
	_, err := app.AsyncDebugEvalTargetOApi(context.Background(), &openapi.AsyncDebugEvalTargetOApiRequest{
		WorkspaceID: gptr.Of(int64(123)), EvalTargetType: gptr.Of(ot.EvalTargetTypeSandboxAgent), Param: gptr.Of("{}"),
		SandboxAgent: &ot.SandboxAgent{SandboxCountMode: gptr.Of(ot.SandboxCountMode("windows_plus_sandbox"))},
	})
	assert.ErrorContains(t, err, "unsupported execution mode")
}

func TestARMReinitCreatesBothTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := rm.NewMockISandboxSchedulerAdapter(ctrl)
	got := map[string]rpc.SandboxResourceType{}
	adapter.EXPECT().Init(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
		got[req.TaskID] = req.ResourceType
		assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneralGUI, req.Tenant)
		assert.Equal(t, int32(35), req.Concurrency)
		return &rpc.SandboxInitResponse{}, nil
	})
	expt := executionModeExperiment(entity.SandboxCountModeArmMacVMPlusSandbox)
	err := reinitSandboxTaskForExperiment(context.Background(), "retry", expt, gptr.Of(7), 123, nil, adapter)
	assert.NoError(t, err)
	assert.Equal(t, map[string]rpc.SandboxResourceType{"456": rpc.SandboxResourceTypeSandbox, "456-macvm": rpc.SandboxResourceTypeMacVM}, got)
	assert.Equal(t, entity.SandboxCountModeArmMacVMPlusSandbox, expt.Target.EvalTargetVersion.SandboxAgent.SandboxCountMode)
}

func TestSubmitWindowsRejectsBeforeResources(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rm.NewMockIAuthProvider(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr := sm.NewMockIExptManager(ctrl)
	expt := executionModeExperiment(entity.SandboxCountModeWindowsPlusSandbox)
	expt.Target.EvalTargetType = entity.EvalTargetTypeSandboxAgent
	mgr.EXPECT().CreateExpt(gomock.Any(), gomock.Any(), gomock.Any()).Return(expt, nil)
	app := &experimentApplication{auth: auth, manager: mgr, idgen: im.NewMockIIDGenerator(ctrl), sandboxSchedulerAdapter: rm.NewMockISandboxSchedulerAdapter(ctrl)}
	_, err := app.SubmitExperiment(context.Background(), &ep.SubmitExperimentRequest{
		WorkspaceID: 123, Name: gptr.Of("windows"), TargetFieldMapping: &de.TargetFieldMapping{},
		CreateEvalTargetParam: &et.CreateEvalTargetParam{EvalTargetType: gptr.Of(dt.EvalTargetType_SandboxAgent)},
	})
	assert.ErrorContains(t, err, "unsupported execution mode")
}
