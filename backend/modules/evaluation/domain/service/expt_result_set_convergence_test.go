package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestConvergedResultSetRequiresManagedApplication(t *testing.T) {
	for _, source := range []string{"", "builtin:result-set-evaluation", "102"} {
		p := &entity.CreateExptParam{WorkspaceID: 1001, CreateEvalTargetParam: &entity.CreateEvalTargetParam{SourceTargetID: &source, EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)}, ExptConf: &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet}}}
		m := &ExptMangerImpl{configer: resultSetApplicationConfig{}}
		require.Error(t, m.prepareResultSetCreation(context.Background(), p), source)
	}
}

func TestConvergedResultSetBuiltinCopyCannotBecomeAgentRun(t *testing.T) {
	for _, reference := range []bool{false, true} {
		ctrl := gomock.NewController(t)
		targets := svcmocks.NewMockIEvalTargetService(ctrl)
		p := &entity.CreateExptParam{WorkspaceID: 1001, ExptConf: &entity.EvaluationConfiguration{}}
		if reference {
			p.TargetID, p.TargetVersionID = gptr.Of(int64(20)), 21
			targets.EXPECT().GetEvalTarget(gomock.Any(), int64(20)).Return(&entity.EvalTarget{SourceTargetID: "builtin:result-set-evaluation"}, nil)
		} else {
			p.CreateEvalTargetParam = &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of("builtin:result-set-evaluation"), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent)}
		}
		m := &ExptMangerImpl{configer: resultSetApplicationConfig{}, evalTargetService: targets}
		require.Error(t, m.prepareResultSetCreation(context.Background(), p))
	}
}
