// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookOnlineFrozenGlobalConfiguration(t *testing.T) {
	key := entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33}
	expt := &entity.Experiment{ID: 22, SpaceID: 11, ExptType: entity.ExptType_Online, EvalConf: &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{MaxTurns: 4}}}
	log := &entity.ExptRunLog{ID: 33, SpaceID: 11, ExptID: 22, ExptRunID: 33, Mode: int32(entity.EvaluationModeAppend), CreatedBy: "original-user"}
	execution, err := newHookExecutionSnapshot(expt, log, "local")
	require.NoError(t, err)
	require.NotNil(t, execution, "Online must freeze execution settings even before its first Invoke")
	seed := &entity.HookScheduleSeed{Version: 1, Key: key, ExecutionScope: "local", Mode: entity.EvaluationModeAppend, CreatedAt: 1700000001, Session: &entity.Session{UserID: "original-user", AppID: 8}, Ext: map[string]string{entity.RetryYieldExtKey: "false"}}
	snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "local", CreatedAt: time.Unix(1700000001, 0), Config: managerEnabledConfig(true, true), Execution: execution, Schedule: seed,
		Context: &spi.HookRunContext{WorkspaceID: gptr.Of("11"), ExperimentID: gptr.Of("22"), RunID: gptr.Of("33"), RunMode: gptr.Of("append"), Initiator: &spi.HookInitiator{UserID: gptr.Of("original-user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("online"), Type: gptr.Of("online")}, EvalSets: []*spi.HookEvalSetRef{}}})
	require.NoError(t, err)
	expt.EvalConf.RunModeConfig.MaxTurns = 99
	require.Equal(t, 4, snapshot.Input().Execution.RunModeConfig.MaxTurns)
	require.Empty(t, snapshot.Input().Context.EvalSets)
	event, err := snapshot.Input().Schedule.Event(key, "local", entity.EvaluationModeAppend, "original-user")
	require.NoError(t, err)
	require.Equal(t, entity.ExptType_Online, event.ExptType)
	require.Equal(t, int64(1700000001), event.CreatedAt)
	require.Equal(t, &entity.Session{UserID: "original-user", AppID: 8}, event.Session)
	_, err = snapshot.Input().Schedule.Event(key, "other", entity.EvaluationModeAppend, "original-user")
	require.Error(t, err)
}
