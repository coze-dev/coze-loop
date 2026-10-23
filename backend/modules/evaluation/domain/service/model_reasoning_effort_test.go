package service

import (
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestDraftServiceRejectsInvalidVersionSettingsBeforeRepositoryWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockIEvaluatorRepo(ctrl)
	repo.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).Times(0)
	svc := &EvaluatorServiceImpl{evaluatorRepo: repo}
	evaluator := &entity.Evaluator{EvaluatorType: entity.EvaluatorTypeAgent, BaseInfo: &entity.BaseInfo{}, AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{AgentConfig: &entity.AgentConfig{Runtime: gptr.Of("codex")}}}
	err := svc.UpdateEvaluatorDraft(t.Context(), evaluator)
	require.Error(t, err)
}
