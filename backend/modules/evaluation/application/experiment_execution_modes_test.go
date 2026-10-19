// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
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
	assert.Equal(t, sandboxTaskConcurrencyForMode(gptr.Of(7), entity.SandboxCountModeMacVMPlusSandbox).sandbox, sandboxTaskConcurrencyForMode(gptr.Of(7), mode).sandbox)
}

func TestReinitWindowsCreatesBothResources(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := rm.NewMockISandboxSchedulerAdapter(ctrl)
	got := map[string]rpc.SandboxResourceType{}
	adapter.EXPECT().Init(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
		got[req.TaskID] = req.ResourceType
		assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, req.Tenant)
		return &rpc.SandboxInitResponse{}, nil
	})
	err := reinitSandboxTaskForExperiment(context.Background(), "retry", executionModeExperiment("windows_plus_sandbox"), nil, 123, nil, adapter)
	assert.NoError(t, err)
	assert.Equal(t, map[string]rpc.SandboxResourceType{"456": 0, "456-windows": 3}, got)
}

func TestAsyncDebugWindowsCreatesResourcesBeforeProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rm.NewMockIAuthProvider(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	scheduler := rm.NewMockISandboxSchedulerAdapter(ctrl)
	scheduler.EXPECT().Init(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
		assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, req.Tenant)
		return &rpc.SandboxInitResponse{}, nil
	})
	targetSvc := sm.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().AsyncDebugTarget(gomock.Any(), gomock.Any()).Return(nil, "", errors.New("provider reached"))
	app := &EvalOpenAPIApplication{auth: auth, metric: &fakeOpenAPIMetric{}, sandboxSchedulerAdapter: scheduler, targetSvc: targetSvc}
	_, err := app.AsyncDebugEvalTargetOApi(context.Background(), &openapi.AsyncDebugEvalTargetOApiRequest{
		WorkspaceID: gptr.Of(int64(123)), EvalTargetType: gptr.Of(ot.EvalTargetTypeSandboxAgent), Param: gptr.Of("{}"),
		SandboxAgent: &ot.SandboxAgent{SandboxCountMode: gptr.Of(ot.SandboxCountMode("windows_plus_sandbox"))},
	})
	assert.ErrorContains(t, err, "provider reached")
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

func TestSubmitWindowsInitializesResourcesBeforeRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rm.NewMockIAuthProvider(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr := sm.NewMockIExptManager(ctrl)
	expt := executionModeExperiment(entity.SandboxCountModeWindowsPlusSandbox)
	expt.Target.EvalTargetType = entity.EvalTargetTypeSandboxAgent
	mgr.EXPECT().CreateExpt(gomock.Any(), gomock.Any(), gomock.Any()).Return(expt, nil)
	scheduler := rm.NewMockISandboxSchedulerAdapter(ctrl)
	scheduler.EXPECT().Init(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
		assert.Equal(t, "456", req.TaskID)
		assert.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, req.Tenant)
		return nil, errors.New("scheduler reached")
	})
	app := &experimentApplication{auth: auth, manager: mgr, idgen: im.NewMockIIDGenerator(ctrl), sandboxSchedulerAdapter: scheduler}
	_, err := app.SubmitExperiment(context.Background(), &ep.SubmitExperimentRequest{
		WorkspaceID: 123, Name: gptr.Of("windows"), TargetFieldMapping: &de.TargetFieldMapping{},
		CreateEvalTargetParam: &et.CreateEvalTargetParam{EvalTargetType: gptr.Of(dt.EvalTargetType_SandboxAgent)},
	})
	assert.ErrorContains(t, err, "scheduler reached")
}
