// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type boundConsumerTargetCapture struct {
	IEvalTargetService
	execution          *entity.ExecuteTargetCtx
	input              *entity.EvalTargetInputData
	space, id, version int64
}

func (s *boundConsumerTargetCapture) ExecuteTarget(_ context.Context, space, id, version int64, execution *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
	s.space, s.id, s.version = space, id, version
	s.execution, s.input = execution, input
	return &entity.EvalTargetRecord{ID: 123}, nil
}

func (s *boundConsumerTargetCapture) AsyncExecuteTarget(ctx context.Context, space, id, version int64, execution *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
	record, err := s.ExecuteTarget(ctx, space, id, version, execution, input)
	return record, "fixture", err
}

func TestHookBoundConsumerOriginalSideConfigDownstreamMySQL(t *testing.T) {
	for _, tc := range []struct {
		name         string
		verification *entity.VerificationConfig
		trajectory   *bool
		keys         map[string]string
	}{
		{"verification", &entity.VerificationConfig{Mode: entity.VerificationModeF2P}, gptr.Of(true), map[string]string{"skill:1": "stable/original-key"}},
		{"run-mode", nil, gptr.Of(false), map[string]string{"skill:1": "stable/original-key"}},
		{"defaults", nil, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := boundConsumerFixtureConfig(t, func(conf *entity.EvaluationConfiguration) {
				conf.VerificationConfig, conf.EnableExtractTrajectory, conf.SkillTOSKeys = tc.verification, tc.trajectory, tc.keys
				if tc.verification != nil {
					conf.RunModeConfig = nil
				}
			})
			ctx := context.Background()
			current, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
			require.NoError(t, err)
			current.EvalConf.VerificationConfig = &entity.VerificationConfig{Mode: entity.VerificationModeOracleOnly}
			current.EvalConf.EnableExtractTrajectory = gptr.Of(!gptr.Indirect(tc.trajectory))
			current.EvalConf.SkillTOSKeys = map[string]string{"skill:1": "stable/changed-key", "new:2": "stable/new-key"}
			raw, err := json.Marshal(current.EvalConf)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", raw).Error)
			c.evaTargetService.(*boundConsumerTarget).value.EvalTargetVersion.SandboxAgent = &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeSingle}
			got, err := c.BuildExptRecordEvalCtx(ctx, boundConsumerEvent(f))
			require.NoError(t, err)
			capture := new(boundConsumerTargetCapture)
			ctrl := gomock.NewController(t)
			metric := metricsmocks.NewMockExptMetric(ctrl)
			metric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), gomock.Any()).AnyTimes()
			async := repomocks.NewMockIEvalAsyncRepo(ctrl)
			async.EXPECT().SetEvalAsyncCtx(gomock.Any(), "123", gomock.Any()).Return(nil).AnyTimes()
			turn := &DefaultExptTurnEvaluationImpl{metric: metric, evalTargetService: capture, evalAsyncRepo: async}
			_, err = turn.callTarget(ctx, &entity.ExptTurnEvalCtx{ExptItemEvalCtx: got, Turn: got.EvalSetItem.Turns[0], Ext: map[string]string{}}, nil, resolveLoadSpaceID(f.space, got.TargetSourceSpaceID()))
			require.NoError(t, err, "current verification must not pollute an original run-mode execution")
			require.Equal(t, f.space+90, capture.space)
			require.Equal(t, int64(91), capture.id)
			require.Equal(t, int64(92), capture.version)
			assert.Equal(t, tc.verification, capture.execution.VerificationConfig)
			assert.Equal(t, tc.trajectory, capture.execution.EnableExtractTrajectory)
			if tc.verification == nil {
				var runMode entity.RunModeConfig
				require.NoError(t, json.Unmarshal([]byte(capture.input.Ext[consts.TargetExecuteExtRunModeConfigKey]), &runMode))
				require.Equal(t, 7, runMode.MaxRunMinutes)
			} else {
				require.NotContains(t, capture.input.Ext, consts.TargetExecuteExtRunModeConfigKey)
			}
			ext := turn.buildEvaluatorInputDataExt(map[string]string{"original": "event"}, nil, got.Expt.EvalConf)
			if tc.keys == nil {
				require.NotContains(t, ext, consts.FieldAdapterBuiltinFieldNameSkillTOSKeys)
			} else {
				require.JSONEq(t, `{"skill:1":"stable/original-key"}`, ext[consts.FieldAdapterBuiltinFieldNameSkillTOSKeys])
			}
			if got.Expt.EvalConf.VerificationConfig != nil {
				got.Expt.EvalConf.VerificationConfig.Mode = entity.VerificationModeNopOnly
			}
			if got.Expt.EvalConf.EnableExtractTrajectory != nil {
				*got.Expt.EvalConf.EnableExtractTrajectory = !*got.Expt.EvalConf.EnableExtractTrajectory
			}
			if got.Expt.EvalConf.SkillTOSKeys != nil {
				got.Expt.EvalConf.SkillTOSKeys["skill:1"] = "mutated-return"
			}
			again, err := c.BuildExptRecordEvalCtx(ctx, boundConsumerEvent(f))
			require.NoError(t, err)
			require.Equal(t, tc.verification, again.Expt.EvalConf.VerificationConfig)
			require.Equal(t, tc.trajectory, again.Expt.EvalConf.EnableExtractTrajectory)
			require.Equal(t, tc.keys, again.Expt.EvalConf.SkillTOSKeys)
		})
	}
}

func TestHookExecutionSideConfigRequestAndLimitMySQL(t *testing.T) {
	f := foundationManagerFixture(t, true, false)
	ctx := context.Background()
	expt, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	expt.EvalConf.RunModeConfig = nil
	raw, err := json.Marshal(expt.EvalConf)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", raw).Error)
	foundationCreate(t, f, entity.EvaluationModeSubmit)
	run := finalizationRead(t, f)
	_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Hash: entity.NewHookPlanDigest().Hash})
	require.NoError(t, err)
	run = finalizationRead(t, f)
	codec := f.manager.hooks.Codec.(*hookinfra.StorageCodec)
	config, hash, err := codec.DecodePhase(ctx, f.key, "local", run.Snapshot, entity.HookPhaseBefore)
	require.NoError(t, err)
	scope := entity.HookAttemptScope{Key: f.key, OperationID: run.State.Before.ID, Phase: entity.HookPhaseBefore, ExecutionScope: "local", SnapshotHash: hash, ExpectedVersion: run.Operations[0].Version}
	claimed, err := f.repo.ClaimAttempt(ctx, entity.HookClaimAttemptInput{HookAttemptScope: scope, Owner: "side-config-test", AttemptID: finalizationTestIDs.Add(1), Config: config})
	require.NoError(t, err)
	require.NotNil(t, claimed.Claim)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Where("operation_id=?", scope.OperationID).Delete(&model.ExptLifecycleHookAttempt{}).Error)
	})
	run = finalizationRead(t, f)
	request, oldHash, err := codec.BuildClaimedRequest(ctx, "local", run, claimed.Claim)
	require.NoError(t, err)
	oldBody, err := hookinfra.BuildRequest(request)
	require.NoError(t, err)
	snapshot, err := codec.DecodeSnapshot(ctx, f.key, "local", run.Snapshot)
	require.NoError(t, err)
	in := snapshot.Input()
	in.Execution.VerificationConfig = &entity.VerificationConfig{Mode: entity.VerificationModeF2P}
	in.Execution.EnableExtractTrajectory = gptr.Of(false)
	in.Execution.SkillTOSKeys = map[string]string{"skill:1": "stable/private-key"}
	owned, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	in.Execution.VerificationConfig.Mode = entity.VerificationModeOracleOnly
	*in.Execution.EnableExtractTrajectory = true
	in.Execution.SkillTOSKeys["skill:1"] = "changed"
	changed, err := codec.EncodeSnapshot(ctx, "key", owned)
	require.NoError(t, err)
	require.NotEqual(t, run.Snapshot.Hash, changed.Hash)
	decoded, err := codec.DecodeSnapshot(ctx, f.key, "local", changed)
	require.NoError(t, err)
	require.Equal(t, entity.VerificationModeF2P, decoded.Input().Execution.VerificationConfig.Mode)
	require.Equal(t, gptr.Of(false), decoded.Input().Execution.EnableExtractTrajectory)
	require.Equal(t, "stable/private-key", decoded.Input().Execution.SkillTOSKeys["skill:1"])
	run.Snapshot = changed
	request, newHash, err := codec.BuildClaimedRequest(ctx, "local", run, claimed.Claim)
	require.NoError(t, err)
	newBody, err := hookinfra.BuildRequest(request)
	require.NoError(t, err)
	require.Equal(t, oldHash, newHash)
	require.Equal(t, oldBody, newBody)
	require.NotContains(t, string(newBody), "stable/private-key")
	large := decoded.Input()
	large.Execution.SkillTOSKeys["skill:1"] = strings.Repeat("x", 256*1024)
	oversize, err := entity.NewHookRunSnapshot(large)
	require.NoError(t, err)
	_, err = codec.EncodeSnapshot(ctx, "key", oversize)
	require.Error(t, err, "private side configuration shares the existing plaintext budget")
}

func TestHookExecutionSnapshotSideConfigCaptureReplayMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := foundationManagerFixture(t, false, true)
			ctx := context.Background()
			expt, err := f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
			require.NoError(t, err)
			expt.EvalConf.RunModeConfig = nil
			expt.EvalConf.VerificationConfig = &entity.VerificationConfig{Mode: entity.VerificationModeF2P}
			expt.EvalConf.EnableExtractTrajectory = gptr.Of(false)
			expt.EvalConf.SkillTOSKeys = map[string]string{"skill:1": "stable/original-key"}
			raw, err := json.Marshal(expt.EvalConf)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", raw).Error)
			foundationCreate(t, f, mode)
			stored := finalizationRead(t, f)
			snapshot, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", stored.Snapshot)
			require.NoError(t, err)
			assertCaptured := func(execution *entity.HookExecutionSnapshot) {
				t.Helper()
				encoded, err := json.Marshal(execution)
				require.NoError(t, err)
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(encoded, &fields))
				assert.JSONEq(t, `{"mode":"f2p"}`, string(fields["verification_config"]))
				assert.Equal(t, "false", string(fields["enable_extract_trajectory"]))
				assert.JSONEq(t, `{"skill:1":"stable/original-key"}`, string(fields["skill_tos_keys"]))
			}
			assertCaptured(snapshot.Input().Execution)
			if t.Failed() {
				return
			}
			owned := snapshot.Input()
			require.NoError(t, json.Unmarshal([]byte(`{"verification_config":{"mode":"oracle_only"},"enable_extract_trajectory":true,"skill_tos_keys":{"skill:1":"changed"}}`), owned.Execution))
			assertCaptured(snapshot.Input().Execution)
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", []byte(`{}`)).Error)
			require.NoError(t, f.manager.LogRunWithPlanSeed(ctx, f.expt, f.key.RunID, mode, f.space, "[801,802]", &entity.Session{UserID: "original-user"}))
			require.Equal(t, stored.Snapshot, finalizationRead(t, f).Snapshot)
			encoded, err := f.manager.hooks.Codec.EncodeSnapshot(ctx, "key", snapshot)
			require.NoError(t, err)
			again, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", encoded)
			require.NoError(t, err)
			assertCaptured(again.Input().Execution)
		})
	}
}
