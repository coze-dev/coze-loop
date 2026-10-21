package service

import (
	"context"
	"errors"
	"fmt"
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
	require.ErrorContains(t, err, "disabled")
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
	target.EXPECT().CreateEvalTarget(gomock.Any(), int64(1001), entity.BuiltinResultSetTargetID, gomock.Any(), entity.EvalTargetTypeSandboxAgent, gomock.Any()).Return(int64(0), int64(0), boundary)
	manager := &ExptMangerImpl{configer: resultSetCreationConfig{}, evalTargetService: target}
	param := &entity.CreateExptParam{WorkspaceID: 1001, EvalSetID: 10, EvalSetVersionID: 11, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}}, ExptConf: &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet}}}
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
			manager := &ExptMangerImpl{configer: resultSetCreationConfig{}}
			err := manager.prepareResultSetCreation(context.Background(), &entity.CreateExptParam{WorkspaceID: 1001, ExptConf: conf, EvalSetConfigs: conf.EvalSetConfigs})
			if conflicting {
				require.ErrorContains(t, err, "conflict")
			} else {
				require.ErrorContains(t, err, "unsupported")
			}
		})
	}
}
