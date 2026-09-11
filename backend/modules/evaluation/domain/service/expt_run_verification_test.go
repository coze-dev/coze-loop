// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestDefaultExptTurnEvaluationImpl_callTarget_Verification(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		mode                        entity.VerificationMode
		raw, wantRaw, wantErr       string
		legacy, runMode, noSnapshot bool
	}{
		{name: "typed nop with nil ext", mode: entity.VerificationModeNopOnly},
		{name: "typed oracle", mode: entity.VerificationModeOracleOnly, raw: `{"binary_version":"pinned"}`, wantRaw: `{"binary_version":"pinned"}`},
		{name: "typed f2p removes matching legacy field", mode: entity.VerificationModeF2P, raw: `{"verification":{"mode":"f2p"},"binary_version":"pinned"}`, wantRaw: `{"binary_version":"pinned"}`},
		{name: "legacy Verify target", legacy: true, raw: `{"verification":{"mode":"oracle_only"},"timeout":30}`, wantRaw: `{"timeout":30}`},
		{name: "conflicting legacy mode", mode: entity.VerificationModeF2P, raw: `{"verification":{"mode":"nop_only"}}`, wantErr: "conflicts"},
		{name: "reject non object runtime", mode: entity.VerificationModeF2P, raw: `[]`, wantErr: "JSON object"},
		{name: "reject run mode", mode: entity.VerificationModeOracleOnly, runMode: true, wantErr: "mutually exclusive"},
		{name: "reject missing snapshot", mode: entity.VerificationModeF2P, noSnapshot: true, wantErr: "SandboxAgent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
			metric := metricsmocks.NewMockExptMetric(ctrl)
			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			svc := &DefaultExptTurnEvaluationImpl{metric: metric, evalTargetService: targetSvc, evalAsyncRepo: asyncRepo}
			conf := &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{TargetVersionID: 9}}}
			if tc.mode != "" {
				conf.VerificationConfig = &entity.VerificationConfig{Mode: tc.mode}
			}
			if tc.runMode {
				conf.RunModeConfig = &entity.RunModeConfig{}
			}
			agent := &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeMacVMPlusSSH}
			if tc.legacy {
				agent.Name = "Verify"
			}
			if tc.noSnapshot {
				agent = nil
			}
			etec := &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event:       &entity.ExptItemEvalEvent{ExptID: 11, ExptRunID: 12, SpaceID: 42},
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 13},
					Expt: &entity.Experiment{EvalConf: conf, Target: &entity.EvalTarget{
						ID: 8, EvalTargetType: entity.EvalTargetTypeSandboxAgent,
						EvalTargetVersion: &entity.EvalTargetVersion{ID: 9, SandboxAgent: agent},
					}},
				},
				Turn: &entity.Turn{ID: 14},
			}
			if tc.raw != "" || tc.runMode {
				etec.Ext = map[string]string{consts.TargetExecuteExtRuntimeParamKey: tc.raw, "lane": "test"}
			}
			metric.EXPECT().EmitTurnExecTargetResult(int64(42), tc.wantErr != "")
			if tc.wantErr == "" {
				targetSvc.EXPECT().AsyncExecuteTarget(gomock.Any(), int64(100), int64(8), int64(9), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _, _, _ int64, exec *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
						require.NotNil(t, exec.VerificationConfig)
						wantMode := tc.mode
						if tc.legacy {
							wantMode = entity.VerificationModeOracleOnly
						}
						require.Equal(t, wantMode, exec.VerificationConfig.Mode)
						require.Equal(t, int64(42), exec.ExptSpaceID)
						require.Equal(t, int64(11), *exec.ExperimentID)
						require.NotNil(t, input.Ext)
						if tc.wantRaw == "" {
							require.Empty(t, input.Ext[consts.TargetExecuteExtRuntimeParamKey])
						} else {
							require.JSONEq(t, tc.wantRaw, input.Ext[consts.TargetExecuteExtRuntimeParamKey])
							require.Equal(t, "test", input.Ext["lane"])
						}
						return &entity.EvalTargetRecord{ID: 15}, "sandbox", nil
					})
				asyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), "15", gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, saved *entity.EvalAsyncCtx) error {
						require.Equal(t, "sandbox", saved.Callee)
						require.Same(t, etec.Event, saved.Event)
						return nil
					})
			}
			got, err := svc.callTarget(context.Background(), etec, nil, 100)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(15), got.ID)
			}
			if etec.Ext != nil {
				require.Equal(t, tc.raw, etec.Ext[consts.TargetExecuteExtRuntimeParamKey], "execution must not mutate the source runtime parameters")
			}
		})
	}
}

func TestExptMangerImpl_CreateExptRejectsInvalidVerification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mode   entity.VerificationMode
		online bool
	}{
		{name: "invalid mode", mode: "unknown"},
		{name: "online verification", mode: entity.VerificationModeF2P, online: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			param := &entity.CreateExptParam{ExptConf: &entity.EvaluationConfiguration{VerificationConfig: &entity.VerificationConfig{Mode: tc.mode}}}
			if tc.online {
				param.ExptType = entity.ExptType_Online
			}
			// No dependencies: validation must precede target creation or persistence.
			got, err := new(ExptMangerImpl).CreateExpt(context.Background(), param, &entity.Session{})
			require.Error(t, err)
			require.Nil(t, got)
			code, _, ok := errno.ParseStatusError(err)
			require.True(t, ok)
			require.EqualValues(t, errno.CommonInvalidParamCode, code)
		})
	}
}
