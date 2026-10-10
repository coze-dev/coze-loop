// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type hookWorkspaceRuntime struct {
	ids    []int64
	paused bool
}

func (r hookWorkspaceRuntime) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	return entity.HookRuntimeConfig{AdmissionEnabled: !r.paused, WorkspaceAllowlistConfigured: true, WorkspaceAllowlist: r.ids}, nil
}

func TestHookManagerWorkspaceAdmissionForNewAndRetryRun(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll} {
		for _, state := range []string{"allowed", "removed", "paused"} {
			t.Run(strconv.Itoa(int(mode))+"/"+state, func(t *testing.T) {
				allowed := state == "allowed"
				f := newHookManagerFixture(t, managerEnabledConfig(true, true))
				ids := []int64{11}
				if state != "removed" {
					ids = []int64{10}
				}
				f.manager.(*ExptMangerImpl).hooks.Runtime = hookWorkspaceRuntime{ids: ids, paused: state == "paused"}
				f.expectLock(mode)
				latest := int64(0)
				if mode != entity.EvaluationModeSubmit {
					latest = 29
					f.expt.LatestRunID = latest
					f.expt.EvalConf = &entity.EvaluationConfiguration{}
					f.expt.EvalSetID, f.expt.EvalSetVersionID = 71, 71
				}
				f.expectRead(latest, nil)
				f.expectConfig()
				f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
				if allowed {
					id := int64(100)
					f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) { id++; return id, nil }).MinTimes(1)
				} else {
					f.expectOwnerCleanup()
				}
				err := f.manager.LogRun(context.Background(), 20, 30, mode, 10, nil, &entity.Session{UserID: "user"})
				if allowed {
					require.NoError(t, err)
					require.NotNil(t, f.repo.input)
				} else {
					require.ErrorContains(t, err, "HOOK_ADMISSION_UNAVAILABLE")
					require.Nil(t, f.repo.input)
					require.Empty(t, f.wake.events)
				}
			})
		}
	}
}

func TestHookManagerWorkspaceRevocationDoesNotStopSameRunReplay(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(true, true))
	f.manager.(*ExptMangerImpl).hooks.Runtime = hookWorkspaceRuntime{}
	f.expectRead(30, managerStoredLog(1))
	f.expectStoredRun(2, 0, "local", true)
	require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "original-user"}))
	require.Nil(t, f.repo.input)
	require.Len(t, f.wake.events, 2)
}

type deniedWorkspaceIDs struct {
	idgen.IIDGenerator
	calls int
}

func (g *deniedWorkspaceIDs) GenMultiIDs(context.Context, int) ([]int64, error) {
	g.calls++
	return nil, errors.New("must not allocate IDs for denied workspace")
}

func TestHookScheduledWorkspaceAdmissionPrecedesSideEffects(t *testing.T) {
	ids := &deniedWorkspaceIDs{}
	p := &scheduledTemplatePreparation{manager: &ExptMangerImpl{idgenerator: ids, hooks: &ExptManagerHookDependencies{Runtime: hookWorkspaceRuntime{ids: []int64{11}}, ExecutionScope: "local"}}}
	b := &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: "binding", Version: 1, UserID: "user", IdentityType: "fornax_user", SpaceID: 10, TemplateID: 20, ExecutionScope: "local", Namespace: "ns", Group: "group", BizKey: "biz", JobID: "job", Enabled: true, BoundAt: time.Unix(100, 0), Callback: entity.ExptTemplateScheduleCallback{PSM: "test.eval", Method: "Submit", Cluster: "default"}}
	require.True(t, b.Active())
	tr := entity.ScheduledRunTrigger{ID: 1, BindingID: b.BindingID, BindingVersion: b.Version, InstanceID: "instance", SpaceID: 10, TemplateID: 20, ExperimentID: 30, RunID: 40, Status: entity.ScheduledRunTriggerPending, CreatedAt: time.Unix(101, 0)}
	_, err := p.Prepare(context.Background(), tr, &repo.ExptTemplateScheduleState{Binding: b, Config: &entity.HookConfigRecord{Config: managerEnabledConfig(true, true)}}, &ScheduledTemplateResources{Create: &entity.CreateExptParam{}, resolved: &exptCreationResources{}})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	require.Zero(t, ids.calls)
	p.manager.hooks.Runtime = hookWorkspaceRuntime{ids: []int64{10}, paused: true}
	_, err = p.Prepare(context.Background(), tr, &repo.ExptTemplateScheduleState{Binding: b, Config: &entity.HookConfigRecord{Config: managerEnabledConfig(true, true)}}, &ScheduledTemplateResources{Create: &entity.CreateExptParam{}, resolved: &exptCreationResources{}})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	require.Zero(t, ids.calls)
}
