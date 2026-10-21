package service

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
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
