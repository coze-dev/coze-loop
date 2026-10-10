// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity_test

import (
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type hookLaneCase struct {
	name                string
	lane                *string
	prod, active, draft bool
}

func hookLaneCases() []hookLaneCase {
	return []hookLaneCase{
		{"omitted", nil, true, false, true},
		{"empty", gptr.Of(""), true, false, true},
		{"no_prefix", gptr.Of("feature-135"), false, true, true},
		{"ascii_boundaries", gptr.Of("!~"), false, true, true},
		{"punctuation", gptr.Of("a/b:c@d?#%+_=.,;\\\"'[]{}()"), false, true, true},
		{"no_length_limit", gptr.Of(strings.Repeat("a", 1024)), false, true, true},
		{"leading_space", gptr.Of(" lane"), false, false, false},
		{"trailing_space", gptr.Of("lane "), false, false, false},
		{"internal_space", gptr.Of("lane name"), false, false, false},
		{"space_only", gptr.Of(" "), false, false, false},
		{"newline", gptr.Of("lane\nname"), false, false, false},
		{"carriage_return", gptr.Of("lane\rname"), false, false, false},
		{"tab", gptr.Of("lane\tname"), false, false, false},
		{"nul", gptr.Of("lane\x00name"), false, false, false},
		{"control_boundary", gptr.Of("lane\x1fname"), false, false, false},
		{"delete", gptr.Of("lane\x7fname"), false, false, false},
		{"non_ascii", gptr.Of("泳道"), false, false, false},
		{"nonbreaking_space", gptr.Of("lane\u00a0"), false, false, false},
		{"zero_width_space", gptr.Of("lane\u200b"), false, false, false},
		{"invalid_utf8", gptr.Of("lane\xff"), false, false, false},
	}
}

func forEachHookLane(t *testing.T, check func(*testing.T, *entity.LifecycleHookConf, string, bool)) {
	t.Helper()
	for _, phase := range []string{"before", "after"} {
		for _, environment := range []entity.HookEnvironment{entity.HookEnvironmentProd, entity.HookEnvironmentPPE, entity.HookEnvironmentBOE} {
			for _, enabled := range []bool{false, true} {
				state := "disabled"
				if enabled {
					state = "enabled"
				}
				for _, tc := range hookLaneCases() {
					t.Run(phase+"/"+string(environment)+"/"+state+"/"+tc.name, func(t *testing.T) {
						valid := tc.draft
						if environment == entity.HookEnvironmentProd {
							valid = tc.prod
						} else if enabled {
							valid = tc.active
						}
						stage := &entity.HookConfig{Enabled: gptr.Of(enabled), Environment: gptr.Of(environment), Lane: tc.lane,
							InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://hook.example/hook")}}
						conf := &entity.LifecycleHookConf{Before: stage}
						if phase == "after" {
							conf = &entity.LifecycleHookConf{After: stage}
						}
						check(t, conf, phase, valid)
					})
				}
			}
		}
	}
}

func hookLaneStage(conf *entity.LifecycleHookConf, phase string) *entity.HookConfig {
	if phase == "before" {
		return conf.Before
	}
	return conf.After
}

func TestLifecycleHookLaneNormalization(t *testing.T) {
	forEachHookLane(t, func(t *testing.T, conf *entity.LifecycleHookConf, phase string, valid bool) {
		stage := hookLaneStage(conf, phase)
		original := gptr.Indirect(stage.Lane)
		got, err := entity.ResolveLifecycleHookConf(nil, conf)
		require.Equal(t, original, gptr.Indirect(stage.Lane), "input must not be trimmed or mutated")
		if !valid {
			require.ErrorContains(t, err, "lifecycle_hook_conf."+phase+".lane")
			require.Nil(t, got)
			return
		}
		require.NoError(t, err)
		lane := hookLaneStage(got, phase).Lane
		require.Equal(t, stage.Lane, lane)
		if lane != nil {
			*lane = "changed"
			require.Equal(t, original, *stage.Lane, "normalized lane must own its pointer")
		}
	})
}
