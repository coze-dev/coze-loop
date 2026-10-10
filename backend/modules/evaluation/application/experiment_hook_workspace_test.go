// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func workspaceAdmissionForTest(allowed bool) workerConfigFunc {
	return func(context.Context) (entity.HookRuntimeConfig, error) {
		ids := []int64{8}
		if allowed {
			ids = []int64{7}
		}
		return entity.HookRuntimeConfig{AdmissionEnabled: true, WorkspaceAllowlistConfigured: true, WorkspaceAllowlist: ids}, nil
	}
}

func TestLifecycleHookWorkspaceCreationAndTemplateAdmission(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, template := range []bool{false, true} {
				t.Run(fmt.Sprintf("allowed=%t/enabled=%t/template=%t", allowed, enabled, template), func(t *testing.T) {
					app, store, _, _ := newHookCreationApplication(t)
					app.hooks.Runtime = workspaceAdmissionForTest(allowed)
					c := creationHookConf()
					c.Before.Enabled = gptr.Of(enabled)
					c.Before.InvokeHTTPInfo = &domain.HookHTTPInfo{URL: gptr.Of("https://new.example/hook")}
					var err error
					if template {
						_, err = app.CreateExperimentTemplate(hookCreationContext(), &expt.CreateExperimentTemplateRequest{WorkspaceID: 7, Meta: &domain.ExptTemplateMeta{Name: gptr.Of("workspace_template")}, LifecycleHookConf: c})
					} else {
						_, err = app.CreateExperiment(hookCreationContext(), &expt.CreateExperimentRequest{WorkspaceID: 7, Name: gptr.Of("workspace_experiment"), EvalSetID: gptr.Of(int64(11)), EvalSetVersionID: gptr.Of(int64(11)), ExptType: gptr.Of(domain.ExptType_Online), LifecycleHookConf: c})
					}
					if !allowed && enabled {
						require.ErrorContains(t, err, "HOOK_FEATURE_DISABLED")
						require.Nil(t, store.experiment)
						require.Nil(t, store.template)
						require.Zero(t, app.idgen.(*hookCreationIDs).next.Load())
					} else {
						require.NoError(t, err)
						require.Equal(t, int64(7), store.input.WorkspaceID)
					}
				})
			}
		}
	}
}

func TestLifecycleHookWorkspaceUpdateAdmission(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("allowed=%t/enabled=%t", allowed, enabled), func(t *testing.T) {
				storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1"}}
				app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, Name: "original"}, storage, true, nil)
				app.hooks.Runtime = workspaceAdmissionForTest(allowed)
				_, err := app.UpdateExperiment(context.Background(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(enabled), InvokeHTTPInfo: &domain.HookHTTPInfo{URL: gptr.Of("https://new.example/hook")}}}})
				if !allowed && enabled {
					require.ErrorContains(t, err, "HOOK_FEATURE_DISABLED")
					require.Zero(t, storage.writes)
				} else {
					require.NoError(t, err)
					require.Equal(t, 1, storage.writes)
				}
			})
		}
	}
}

func TestLifecycleHookDisabledConfigurationDoesNotReadRuntime(t *testing.T) {
	for _, nilRuntime := range []bool{false, true} {
		t.Run(fmt.Sprintf("nil_runtime=%t", nilRuntime), func(t *testing.T) {
			calls := 0
			unavailable := workerConfigFunc(func(context.Context) (entity.HookRuntimeConfig, error) {
				calls++
				return entity.HookRuntimeConfig{}, errors.New("runtime unavailable")
			})
			app, store, _, _ := newHookCreationApplication(t)
			app.hooks.Runtime = unavailable
			if nilRuntime {
				app.hooks.Runtime = nil
			}
			_, err := app.CreateExperiment(hookCreationContext(), &expt.CreateExperimentRequest{WorkspaceID: 7, Name: gptr.Of("disabled_experiment"), EvalSetID: gptr.Of(int64(11)), EvalSetVersionID: gptr.Of(int64(11)), ExptType: gptr.Of(domain.ExptType_Online), LifecycleHookConf: creationHookConf()})
			require.NoError(t, err)
			require.NotNil(t, store.experiment)
			_, err = app.CreateExperimentTemplate(hookCreationContext(), &expt.CreateExperimentTemplateRequest{WorkspaceID: 7, Meta: &domain.ExptTemplateMeta{Name: gptr.Of("disabled_template")}, LifecycleHookConf: creationHookConf()})
			require.NoError(t, err)
			require.NotNil(t, store.template)
			storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1", Config: &entity.LifecycleHookConf{
				Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/before")}},
				After:  &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/after")}},
			}}}
			updater := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, Name: "original"}, storage, true, nil)
			updater.hooks.Runtime = unavailable
			if nilRuntime {
				updater.hooks.Runtime = nil
			}
			out, err := updater.UpdateExperiment(context.Background(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, LifecycleHookConf: creationHookConf()})
			require.NoError(t, err)
			require.False(t, out.Experiment.LifecycleHookConf.Before.GetEnabled())
			require.True(t, out.Experiment.LifecycleHookConf.After.GetEnabled(), "untouched phase must be preserved")
			require.Equal(t, 1, storage.writes)
			require.Zero(t, calls, "disabled configuration must not require runtime availability")
		})
	}
}
