// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestResultSetSubmissionAndRetryRejectAbsentDeploymentGate(t *testing.T) {
	manager := &ExptMangerImpl{}
	conf := &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet}}
	_, err := manager.CreateExpt(context.Background(), &entity.CreateExptParam{WorkspaceID: 1, ExptConf: conf}, nil)
	require.ErrorContains(t, err, "managed application")
	err = manager.CheckRun(context.Background(), &entity.Experiment{EvalConf: conf}, 1, nil)
	require.ErrorContains(t, err, "disabled")
}

type resultSetCreationConfig struct{ component.IConfiger }

func (resultSetCreationConfig) CheckResultSetRuntime(_ context.Context, _ int64, version string) error {
	if version != "result-set-test" {
		return fmt.Errorf("unsupported runtime")
	}
	return nil
}
func (resultSetCreationConfig) ResolveResultSetRuntime(_ context.Context, _ int64, requested string) (string, error) {
	if requested == "" {
		return "result-set-test", nil
	}
	return requested, nil
}

func TestResultSetFreshCreationFreezesSystemVersion(t *testing.T) {
	boundary := errors.New("resource boundary reached")
	target := svcmocks.NewMockIEvalTargetService(gomock.NewController(t))
	target.EXPECT().CreateEvalTarget(gomock.Any(), int64(1001), "101", gomock.Any(), entity.EvalTargetTypeSandboxAgent, gomock.Any()).Return(int64(0), int64(0), boundary)
	manager := &ExptMangerImpl{configer: resultSetApplicationConfig{}, evalTargetService: target}
	param := &entity.CreateExptParam{WorkspaceID: 1001, CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("101"), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)}, EvalSetID: 10, EvalSetVersionID: 11, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}}, ExptConf: &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet}}}
	_, err := manager.CreateExpt(context.Background(), param, nil)
	require.ErrorIs(t, err, boundary)
	versions, err := param.ExptConf.ResultSetBinaryVersions()
	require.NoError(t, err)
	require.NotEmpty(t, versions)
	for _, version := range versions {
		require.Equal(t, "result-set-test", version)
	}
}

func TestResultSetExplicitPinsNeverFallBackToSystemDefault(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		t.Run(fmt.Sprint(conflicting), func(t *testing.T) {
			conf := &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet}, EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}}}
			require.NoError(t, conf.FreezeResultSetBinaryVersion("unsupported"))
			if conflicting {
				conf.EvalSetConfigs[0].TargetConfs[0].RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = `{"binary_version":"result-set-test"}`
			}
			manager := &ExptMangerImpl{configer: resultSetApplicationConfig{}}
			err := manager.prepareResultSetCreation(context.Background(), &entity.CreateExptParam{WorkspaceID: 1001, CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("101"), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)}, ExptConf: conf, EvalSetConfigs: conf.EvalSetConfigs, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig})
			if conflicting {
				require.ErrorContains(t, err, "conflict")
			} else {
				require.ErrorContains(t, err, "unsupported")
			}
		})
	}
}

type resultSetApplicationConfig struct{ resultSetCreationConfig }

func (resultSetApplicationConfig) IsResultSetApplication(_ context.Context, id string) bool {
	return id == "101"
}

func TestResultSetApplicationNormalizesDefaultFormAndFreezesPin(t *testing.T) {
	manager := &ExptMangerImpl{configer: resultSetApplicationConfig{}}
	param := &entity.CreateExptParam{
		WorkspaceID:           1001,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("101"), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)},
		EvalSetSourceType:     entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSetConfigs:        []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}},
		ExptConf:              &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeSingleTurn, MaxTurns: 1, MaxRunMinutes: 30}},
	}
	require.NoError(t, manager.prepareResultSetCreation(context.Background(), param))
	require.True(t, param.ExptConf.IsResultSet(), "a trusted application must select the stored result-set mode")
	require.Zero(t, param.ExptConf.RunModeConfig.MaxTurns, "the form default must not override source rounds")
	require.Equal(t, 30, param.ExptConf.RunModeConfig.MaxRunMinutes)
	versions, err := param.ExptConf.ResultSetBinaryVersions()
	require.NoError(t, err)
	require.Equal(t, []string{"result-set-test", "result-set-test"}, versions)
	require.NoError(t, param.PrepareResultSetTarget())
	require.Equal(t, "101", *param.CreateEvalTargetParam.SourceTargetID)
	require.Nil(t, param.CreateEvalTargetParam.SandboxAgent, "the real application must be fetched")
}

func TestResultSetApplicationDoesNotTrustNameOrFreeJSON(t *testing.T) {
	manager := &ExptMangerImpl{configer: resultSetApplicationConfig{}}
	param := &entity.CreateExptParam{WorkspaceID: 1001, CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("102"), SandboxAgent: &entity.SandboxAgent{Name: "Result set evaluation"}}, ExptConf: &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeSingleTurn}}}
	require.NoError(t, manager.prepareResultSetCreation(context.Background(), param))
	require.False(t, param.ExptConf.IsResultSet())
}

func TestResultSetApplicationSharedSourceIsAuthorizedBeforeConsumerSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := svcmocks.NewMockResourceAccessAuthorizer(ctrl)
	auth.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
		require.Equal(t, int64(101), req.ResourceID)
		require.Equal(t, int64(1001), req.CallerSpaceID)
		require.Equal(t, int64(2002), *req.SharedOption.SourceSpaceID)
		return &entity.ResourceAccessContext{ResourceSpaceID: 2002, AccessMode: entity.AccessModeShared, AccessLevel: entity.SharedAccessLevelExecute}, nil
	})
	boundary := errors.New("real application lookup boundary")
	targets := svcmocks.NewMockIEvalTargetService(ctrl)
	targets.EXPECT().CreateEvalTarget(gomock.Any(), int64(1001), "101", gomock.Any(), entity.EvalTargetTypeSandboxAgent, gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _, _ string, _ entity.EvalTargetType, opts ...entity.Option) (int64, int64, error) {
		var opt entity.Opt
		for _, f := range opts {
			f(&opt)
		}
		require.Equal(t, int64(2002), opt.ResultSetSourceSpaceID)
		require.Nil(t, opt.SandboxAgent)
		return 0, 0, boundary
	})
	manager := &ExptMangerImpl{configer: resultSetApplicationConfig{}, resourceAccessAuthorizer: auth, evalTargetService: targets}
	param := &entity.CreateExptParam{WorkspaceID: 1001, EvalSetID: 10, EvalSetVersionID: 11, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}}, ExptConf: &entity.EvaluationConfiguration{}, CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("101"), SourceTargetVersion: gptr.Of("result-set-app:2002:99"), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)}}
	_, err := manager.CreateExpt(context.Background(), param, nil)
	require.ErrorIs(t, err, boundary)
}

func TestResultSetApplicationSharedDenialCannotCreateSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	denied := errors.New("source application not shared")
	auth := svcmocks.NewMockResourceAccessAuthorizer(ctrl)
	auth.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).Return(nil, denied)
	manager := &ExptMangerImpl{configer: resultSetApplicationConfig{}, resourceAccessAuthorizer: auth, evalTargetService: svcmocks.NewMockIEvalTargetService(ctrl)}
	param := &entity.CreateExptParam{WorkspaceID: 1001, EvalSetID: 10, EvalSetVersionID: 11, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}}, ExptConf: &entity.EvaluationConfiguration{}, TargetSharedOption: &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(int64(2002))}, CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("101"), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)}}
	_, err := manager.CreateExpt(context.Background(), param, nil)
	require.ErrorIs(t, err, denied)
}

func TestResultSetApplicationExplicitModeStillRejectsTurnOverrides(t *testing.T) {
	manager := &ExptMangerImpl{configer: resultSetApplicationConfig{}}
	param := &entity.CreateExptParam{
		WorkspaceID:           1001,
		EvalSetSourceType:     entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSetConfigs:        []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}},
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("101"), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)},
		ExptConf:              &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet, MaxTurns: 10}},
	}
	require.NoError(t, manager.prepareResultSetCreation(context.Background(), param))
	require.Equal(t, 10, param.ExptConf.RunModeConfig.MaxTurns)
	require.ErrorContains(t, param.PrepareResultSetTarget(), "only accepts max_run_minutes")
}
