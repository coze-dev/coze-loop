// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestExptTemplateManagerImpl_VerificationTargetSnapshot(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	agent := &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeMacVMPlusSSH}
	targetSvc.EXPECT().CreateEvalTarget(ctx, int64(42), entity.BuiltinVerificationTargetID, "", entity.EvalTargetTypeSandboxAgent, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, _, _ string, _ entity.EvalTargetType, opts ...entity.Option) (int64, int64, error) {
			opt := new(entity.Opt)
			for _, f := range opts {
				f(opt)
			}
			require.Same(t, agent, opt.SandboxAgent)
			return 8, 9, nil
		})
	mgr := &ExptTemplateManagerImpl{evalTargetService: targetSvc}
	id, version, targetType, err := mgr.resolveTargetForCreate(ctx, &entity.CreateExptTemplateParam{SpaceID: 42, CreateEvalTargetParam: &entity.CreateEvalTargetParam{
		SourceTargetID: gptr.Of(entity.BuiltinVerificationTargetID), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent), SandboxAgent: agent,
	}})
	require.NoError(t, err)
	require.Equal(t, int64(8), id)
	require.Equal(t, int64(9), version)
	require.Equal(t, entity.EvalTargetTypeSandboxAgent, targetType)
}

func newVerificationTemplateManager(t *testing.T) (*ExptTemplateManagerImpl, *repomocks.MockIExptTemplateRepo, *svcmocks.MockIEvalTargetService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := repomocks.NewMockIExptTemplateRepo(ctrl)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	evalSet := svcmocks.NewMockIEvaluationSetService(ctrl)
	version := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	evaluator := svcmocks.NewMockEvaluatorService(ctrl)
	lwt := lwtmocks.NewMockILatestWriteTracker(ctrl)
	// Tuple enrichment is independent of the configuration being persisted.
	target.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any(), true).Return(nil, nil).AnyTimes()
	evalSet.EXPECT().BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil).Return(nil, nil).AnyTimes()
	version.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), nil).Return(nil, nil).AnyTimes()
	evaluator.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()
	lwt.EXPECT().SetWriteFlag(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	return &ExptTemplateManagerImpl{
		templateRepo: repo, evalTargetService: target,
		evaluationSetService: evalSet, evaluationSetVersionService: version,
		evaluatorService: evaluator, lwt: lwt, idgen: idgenmocks.NewMockIIDGenerator(ctrl),
	}, repo, target
}

func TestExptTemplateManagerImpl_CreateVerification(t *testing.T) {
	for _, mode := range []entity.VerificationMode{entity.VerificationModeNopOnly, entity.VerificationModeOracleOnly, entity.VerificationModeF2P} {
		t.Run(string(mode), func(t *testing.T) {
			mgr, repo, target := newVerificationTemplateManager(t)
			param := newBasicCreateParam()
			config := &entity.VerificationConfig{Mode: mode}
			agent := &entity.SandboxAgent{Name: "client name", SandboxCountMode: entity.SandboxCountModeMacVMPlusSSH}
			param.TemplateConf = &entity.ExptTemplateConfiguration{VerificationConfig: config}
			param.CreateEvalTargetParam = &entity.CreateEvalTargetParam{SandboxAgent: agent}
			repo.EXPECT().GetByName(gomock.Any(), param.Name, param.SpaceID, gomock.Any()).Return(nil, false, nil)
			mgr.idgen.(*idgenmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(10001), nil)
			target.EXPECT().CreateEvalTarget(gomock.Any(), param.SpaceID, entity.BuiltinVerificationTargetID, "", entity.EvalTargetTypeSandboxAgent, gomock.Any()).DoAndReturn(
				func(_ context.Context, _ int64, _, _ string, _ entity.EvalTargetType, opts ...entity.Option) (int64, int64, error) {
					opt := new(entity.Opt)
					for _, f := range opts {
						f(opt)
					}
					require.NotNil(t, opt.SandboxAgent)
					require.Equal(t, entity.SandboxCountModeMacVMPlusSSH, opt.SandboxAgent.SandboxCountMode)
					require.Equal(t, "Dataset verification", opt.SandboxAgent.Name)
					require.Equal(t, entity.SandboxAgentTypeSingleRunCLI, opt.SandboxAgent.Type)
					return 8, 9, nil
				})
			repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, saved *entity.ExptTemplate, _ []*entity.ExptTemplateEvaluatorRef) error {
					require.Equal(t, config, saved.TemplateConf.VerificationConfig)
					require.Equal(t, int64(8), saved.GetTargetID())
					require.Equal(t, int64(9), saved.GetTargetVersionID())
					require.Equal(t, entity.EvalTargetTypeSandboxAgent, saved.GetTargetType())
					return nil
				})
			got, err := mgr.Create(context.Background(), param, &entity.Session{UserID: "u1"})
			require.NoError(t, err)
			require.Equal(t, config, got.TemplateConf.VerificationConfig)
			require.Equal(t, "client name", agent.Name, "server identity must not mutate the caller's snapshot")
		})
	}
}

func TestExptTemplateManagerImpl_CreateVerificationRejectsInvalidTarget(t *testing.T) {
	for _, tc := range []struct {
		name             string
		mode             entity.VerificationMode
		online, existing bool
	}{
		{name: "invalid mode", mode: "invalid"},
		{name: "online", mode: entity.VerificationModeF2P, online: true},
		{name: "existing prompt target", mode: entity.VerificationModeOracleOnly, existing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, repo, target := newVerificationTemplateManager(t)
			param := newBasicCreateParam()
			param.TemplateConf = &entity.ExptTemplateConfiguration{VerificationConfig: &entity.VerificationConfig{Mode: tc.mode}}
			if tc.online {
				param.ExptType = entity.ExptType_Online
			}
			if tc.existing {
				param.TargetID, param.TargetVersionID = 8, 9
				repo.EXPECT().GetByName(gomock.Any(), param.Name, param.SpaceID, gomock.Any()).Return(nil, false, nil)
				mgr.idgen.(*idgenmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(10001), nil)
				target.EXPECT().GetEvalTarget(gomock.Any(), int64(8)).Return(&entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeLoopPrompt}, nil)
			}
			got, err := mgr.Create(context.Background(), param, &entity.Session{})
			require.Error(t, err)
			require.Nil(t, got)
			code, _, ok := errno.ParseStatusError(err)
			require.True(t, ok)
			require.EqualValues(t, errno.CommonInvalidParamCode, code)
			// No Create expectation: invalid configurations must never reach storage.
		})
	}
}

func TestExptTemplateManagerImpl_UpdateVerification(t *testing.T) {
	for _, tc := range []struct {
		name                                            string
		omitConf, override, newSnapshot, online, prompt bool
	}{
		{name: "omitted configuration", omitConf: true},
		{name: "old client changes concurrency"},
		{name: "explicit mode update", override: true},
		{name: "new sandbox snapshot", newSnapshot: true},
		{name: "reject online", online: true},
		{name: "reject prompt target", prompt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr, repo, target := newVerificationTemplateManager(t)
			config := &entity.VerificationConfig{Mode: entity.VerificationModeOracleOnly}
			existing := &entity.ExptTemplate{
				Meta:         &entity.ExptTemplateMeta{ID: 1, WorkspaceID: 100, Name: "tpl", ExptType: entity.ExptType_Offline},
				TripleConfig: &entity.ExptTemplateTuple{TargetID: 8, TargetVersionID: 9, TargetType: entity.EvalTargetTypeSandboxAgent},
				TemplateConf: &entity.ExptTemplateConfiguration{VerificationConfig: config, ItemConcurNum: gptr.Of(3), ExptSource: &entity.ExptSource{}},
			}
			param := &entity.UpdateExptTemplateParam{TemplateID: 1, SpaceID: 100}
			want := config
			if !tc.omitConf {
				param.TemplateConf = &entity.ExptTemplateConfiguration{ItemConcurNum: gptr.Of(5)}
			}
			if tc.override {
				want = &entity.VerificationConfig{Mode: entity.VerificationModeF2P}
				param.TemplateConf.VerificationConfig = want
			}
			if tc.online {
				param.ExptType = entity.ExptType_Online
			}
			if tc.prompt {
				existing.TripleConfig.TargetType = entity.EvalTargetTypeLoopPrompt
			}
			if tc.newSnapshot {
				agent := &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeMacVMPlusSSH}
				param.CreateEvalTargetParam = &entity.CreateEvalTargetParam{SourceTargetID: gptr.Of(entity.BuiltinVerificationTargetID), EvalTargetType: gptr.Of(entity.EvalTargetTypeSandboxAgent), SandboxAgent: agent}
				target.EXPECT().GetEvalTarget(gomock.Any(), int64(8)).Return(&entity.EvalTarget{SourceTargetID: entity.BuiltinVerificationTargetID}, nil)
				target.EXPECT().CreateEvalTarget(gomock.Any(), int64(100), entity.BuiltinVerificationTargetID, "", entity.EvalTargetTypeSandboxAgent, gomock.Any()).DoAndReturn(
					func(_ context.Context, _ int64, _, _ string, _ entity.EvalTargetType, opts ...entity.Option) (int64, int64, error) {
						opt := new(entity.Opt)
						for _, f := range opts {
							f(opt)
						}
						require.Same(t, agent, opt.SandboxAgent)
						return 8, 10, nil
					})
			}
			repo.EXPECT().GetByID(gomock.Any(), int64(1), gomock.Any()).Return(existing, nil)
			if !tc.online && !tc.prompt {
				var saved *entity.ExptTemplate
				repo.EXPECT().UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, tpl *entity.ExptTemplate, _ []*entity.ExptTemplateEvaluatorRef) error {
						saved = tpl
						require.Equal(t, want, tpl.TemplateConf.VerificationConfig)
						require.Same(t, existing.TemplateConf.ExptSource, tpl.TemplateConf.ExptSource)
						if !tc.omitConf {
							require.Equal(t, 5, *tpl.TemplateConf.ItemConcurNum)
						}
						if tc.newSnapshot {
							require.Equal(t, int64(10), tpl.GetTargetVersionID())
						}
						return nil
					})
				repo.EXPECT().GetByID(gomock.Any(), int64(1), gomock.Any()).DoAndReturn(func(context.Context, int64, *int64) (*entity.ExptTemplate, error) { return saved, nil })
			}
			got, err := mgr.Update(context.Background(), param, &entity.Session{UserID: "u1"})
			if tc.online || tc.prompt {
				require.ErrorContains(t, err, "offline SandboxAgent")
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, want, got.TemplateConf.VerificationConfig)
		})
	}
}
