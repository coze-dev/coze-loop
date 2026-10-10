// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	api "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
)

func TestLifecycleHookLaneCreationEntrypoints(t *testing.T) {
	forEachHookLane(t, func(t *testing.T, conf *entity.LifecycleHookConf, phase string, valid bool) {
		dto := experiment.LifecycleHookConfDO2DTO(conf)
		for _, entry := range []struct {
			name string
			run  func() error
		}{
			{"template_create", func() error {
				_, err := experiment.ConvertCreateExptTemplateReq(&api.CreateExperimentTemplateRequest{LifecycleHookConf: dto})
				return err
			}},
			{"template_update", func() error {
				_, err := experiment.ConvertUpdateExptTemplateReq(&api.UpdateExperimentTemplateRequest{LifecycleHookConf: dto})
				return err
			}},
			{"experiment_create", func() error {
				_, err := experiment.ConvertCreateReq(&api.CreateExperimentRequest{LifecycleHookConf: dto}, nil)
				return err
			}},
			{"inherited_config", func() error {
				_, err := entity.ResolveLifecycleHookConf(conf, &entity.LifecycleHookConf{})
				return err
			}},
		} {
			t.Run(entry.name, func(t *testing.T) {
				err := entry.run()
				if valid {
					require.NoError(t, err)
				} else {
					require.ErrorContains(t, err, "lifecycle_hook_conf."+phase+".lane")
				}
			})
		}
	})
}

func TestLifecycleHookLaneRunSnapshot(t *testing.T) {
	forEachHookLane(t, func(t *testing.T, conf *entity.LifecycleHookConf, phase string, valid bool) {
		active := &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://hook.example/other")}}
		if phase == "before" {
			conf.After = active
		} else {
			conf.Before = active
		}
		input := entity.HookRunSnapshotInput{
			Key: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, ExecutionScope: "ppe_platform",
			CreatedAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Config: conf,
			Context: &spi.HookRunContext{WorkspaceID: gptr.Of("1"), ExperimentID: gptr.Of("2"), RunID: gptr.Of("3"), RunMode: gptr.Of("submit"),
				Initiator:  &spi.HookInitiator{UserID: gptr.Of("user"), IdentityType: gptr.Of("fornax_user")},
				Experiment: &spi.HookExperimentRef{Name: gptr.Of("experiment"), Type: gptr.Of("online")}, EvalSets: []*spi.HookEvalSetRef{}},
		}
		got, err := entity.NewHookRunSnapshot(input)
		if !valid {
			require.Error(t, err)
			require.Nil(t, got)
			return
		}
		require.NoError(t, err)
		require.Equal(t, hookLaneStage(conf, phase).Lane, hookLaneStage(got.Input().Config, phase).Lane)
	})
}

func TestLifecycleHookLaneTransportAgreement(t *testing.T) {
	for _, environment := range []string{"Prod", "PPE", "BOE"} {
		for _, tc := range hookLaneCases() {
			t.Run(environment+"/"+tc.name, func(t *testing.T) {
				valid := tc.active
				if environment == "Prod" {
					valid = tc.prod
				}
				targetURL, err := url.Parse("https://hook.example/hook")
				require.NoError(t, err)
				target := hook.EndpointTarget{WorkspaceID: 1, URL: targetURL, Environment: environment, Lane: gptr.Indirect(tc.lane)}
				policy := hook.EndpointPolicy{WorkspaceID: 1, Host: "hook.example", Port: 443, Environment: environment, Lane: target.Lane}
				_, err = hook.MatchEndpoint(target, []hook.EndpointPolicy{policy})
				if valid {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		}
	}
}
