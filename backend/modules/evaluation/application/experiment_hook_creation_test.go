// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapidomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type hookCreationIDs struct {
	idgen.IIDGenerator
	next atomic.Int64
}

// The real Dataset Service is a singleton; its test transport must not capture a test controller.
type hookCreationDatasetRPC struct{ rpc.IDatasetRPCAdapter }

func (hookCreationDatasetRPC) GetDataset(_ context.Context, space *int64, id int64, _ *bool, _ *entity.SharedResourceOption) (*entity.EvaluationSet, error) {
	if gptr.Indirect(space) != 7 || id != 11 {
		return nil, errors.New("unexpected dataset owner")
	}
	return &entity.EvaluationSet{ID: id, SpaceID: 7}, nil
}
func (hookCreationDatasetRPC) BatchGetDatasets(_ context.Context, space *int64, ids []int64, _ *bool, _ *entity.SharedResourceOption) ([]*entity.EvaluationSet, error) {
	if gptr.Indirect(space) != 7 {
		return nil, errors.New("unexpected dataset owner")
	}
	var out []*entity.EvaluationSet
	for _, id := range ids {
		if id != 0 && id != 11 {
			return nil, errors.New("unexpected dataset id")
		}
		if id > 0 {
			out = append(out, &entity.EvaluationSet{ID: id, SpaceID: 7})
		}
	}
	return out, nil
}

func (g *hookCreationIDs) GenID(context.Context) (int64, error) { return 100 + g.next.Add(1), nil }
func (g *hookCreationIDs) GenMultiIDs(ctx context.Context, n int) ([]int64, error) {
	ids := make([]int64, n)
	for i := range ids {
		ids[i], _ = g.GenID(ctx)
	}
	return ids, nil
}

type hookCreationStore struct {
	*hookApplicationConfigStore
	experiment *entity.Experiment
	template   *entity.ExptTemplate
	input      repo.HookConfigCreateInput
	refs       []*entity.ExptEvaluatorRef
}

func (s *hookCreationStore) CreateExperimentWithHookConfig(_ context.Context, x *entity.Experiment, refs []*entity.ExptEvaluatorRef, in repo.HookConfigCreateInput) error {
	s.experiment = x
	s.refs = refs
	s.input = in
	return nil
}
func (s *hookCreationStore) CreateTemplateWithHookConfig(_ context.Context, x *entity.ExptTemplate, _ []*entity.ExptTemplateEvaluatorRef, in repo.HookConfigCreateInput) error {
	s.template = x
	s.input = in
	return nil
}

func newHookCreationApplication(t *testing.T) (*experimentApplication, *hookCreationStore, *repomocks.MockIExperimentRepo, *rpcmocks.MockIAuthProvider) {
	t.Helper()
	ctrl := gomock.NewController(t)
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	templates := repomocks.NewMockIExptTemplateRepo(ctrl)
	stats := repomocks.NewMockIExptStatsRepo(ctrl)
	filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	tracker := lwtmocks.NewMockILatestWriteTracker(ctrl)
	auditor := auditmocks.NewMockIAuditService(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	users := rpcmocks.NewMockIUserProvider(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Any()).Return(nil).AnyTimes()
	tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	tracker.EXPECT().SetWriteFlag(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	auditor.EXPECT().Audit(gomock.Any(), gomock.Any()).Return(audit.AuditRecord{}, nil).AnyTimes()
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(7)).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{MaxItemConcurNum: 10}}).AnyTimes()
	users.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	experiments.EXPECT().GetByName(gomock.Any(), gomock.Any(), int64(7)).Return(nil, false, nil).AnyTimes()
	templates.EXPECT().GetByName(gomock.Any(), gomock.Any(), int64(7), gomock.Any()).Return(nil, false, nil).AnyTimes()
	stats.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	filters.EXPECT().InsertExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ids := &hookCreationIDs{}
	sets := service.NewEvaluationSetServiceImpl(hookCreationDatasetRPC{})
	results := service.NewExptResultService(nil, nil, nil, stats, experiments, nil, tracker, ids, filters, nil, nil, nil, nil, sets, nil, nil, nil, nil, nil, nil, nil)
	manager := service.NewExptManager(results, experiments, nil, stats, nil, nil, nil, config, nil, nil, nil, nil, auditor, ids, nil, tracker, nil, sets, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	templateManager := service.NewExptTemplateManager(templates, ids, nil, nil, sets, nil, tracker, nil, nil, experiments, nil, nil)
	store := &hookCreationStore{hookApplicationConfigStore: &hookApplicationConfigStore{}}
	app := &experimentApplication{manager: manager, templateManager: templateManager, idgen: ids, resultSvc: results, auth: auth, userInfoService: userinfo.NewUserInfoServiceImpl(users), hooks: &ExperimentHookApplicationDependencies{Configs: store, Summaries: hookApplicationSummaries{}, Runtime: hookApplicationRuntime{enabled: true}, ExecutionScope: "test-scope", ConfigKeyID: "operator-key"}}
	return app, store, experiments, auth
}

func creationHookConf() *domain.LifecycleHookConf {
	return &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":9007199254740993}`)}}
}
func hookCreationContext() context.Context {
	return session.WithCtxUser(context.Background(), &session.User{ID: "trusted-user"})
}

func TestLifecycleHookCreationHandlerAtomicAndRequestScoped(t *testing.T) {
	app, store, experiments, _ := newHookCreationApplication(t)
	base := app.manager
	req := &expt.CreateExperimentRequest{WorkspaceID: 7, Name: gptr.Of("creation"), Desc: gptr.Of("keep-description"), EvalSetID: gptr.Of(int64(11)), EvalSetVersionID: gptr.Of(int64(11)), ExptType: gptr.Of(domain.ExptType_Online), LifecycleHookConf: creationHookConf(), Session: &common.Session{UserID: gptr.Of(int64(999))}}
	out, err := app.CreateExperiment(hookCreationContext(), req)
	require.NoError(t, err)
	require.NotNil(t, store.experiment)
	require.Equal(t, out.Experiment.GetID(), store.experiment.ID)
	require.Equal(t, "trusted-user", store.experiment.CreatedBy)
	require.Equal(t, "keep-description", out.Experiment.GetDesc())
	require.Equal(t, `{"keep":9007199254740993}`, out.Experiment.LifecycleHookConf.Before.GetParametersJSON())
	require.False(t, *store.input.Config.Before.Enabled)
	require.Equal(t, int64(7), store.input.WorkspaceID)
	require.Equal(t, "test-scope", store.input.ExecutionScope)
	require.Equal(t, "operator-key", store.input.KeyID)
	require.Same(t, base, app.manager)
	experiments.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	legacy := *req
	legacy.LifecycleHookConf = nil
	old, err := app.CreateExperiment(hookCreationContext(), &legacy)
	require.NoError(t, err)
	require.Nil(t, old.Experiment.LifecycleHookConf)
}

func TestLifecycleHookCreationCloneRebindsConfig(t *testing.T) {
	app, store, experiments, _ := newHookCreationApplication(t)
	source := &entity.Experiment{ID: 42, SpaceID: 7, Name: "clone-source", Description: "keep", EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 8, EvaluatorVersionID: 9}}}
	experiments.EXPECT().MGetByID(gomock.Any(), []int64{42}, int64(7)).Return([]*entity.Experiment{source}, nil)
	experiments.EXPECT().GetByID(gomock.Any(), int64(42), int64(7)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) { copy := *source; return &copy, nil })
	store.record.Config = &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"source":42}`)}}
	out, err := app.CloneExperiment(hookCreationContext(), &expt.CloneExperimentRequest{WorkspaceID: gptr.Of(int64(7)), ExptID: gptr.Of(int64(42))})
	require.NoError(t, err)
	require.NotEqual(t, int64(42), out.Experiment.GetID())
	require.Equal(t, out.Experiment.GetID(), store.experiment.ID)
	require.Equal(t, `{"source":42}`, out.Experiment.LifecycleHookConf.Before.GetParametersJSON())
	require.Equal(t, "keep", out.Experiment.GetDesc())
	require.Len(t, store.refs, 1)
	require.Equal(t, out.Experiment.GetID(), store.refs[0].ExptID)
	require.Positive(t, store.refs[0].ID)
	require.Equal(t, hookcomponent.ConfigOwner{WorkspaceID: 7, ObjectID: 42, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "test-scope"}, store.owner)
	require.Equal(t, int64(42), source.ID)
}

func TestLifecycleHookCreationTemplateHandlerAtomic(t *testing.T) {
	app, store, _, _ := newHookCreationApplication(t)
	base := app.templateManager
	out, err := app.CreateExperimentTemplate(hookCreationContext(), &expt.CreateExperimentTemplateRequest{WorkspaceID: 7, Meta: &domain.ExptTemplateMeta{Name: gptr.Of("template")}, LifecycleHookConf: creationHookConf()})
	require.NoError(t, err)
	require.NotNil(t, store.template)
	require.Equal(t, out.ExperimentTemplate.Meta.GetID(), store.template.GetID())
	require.NotNil(t, out.ExperimentTemplate.LifecycleHookConf)
	require.False(t, out.ExperimentTemplate.LifecycleHookConf.Before.GetEnabled())
	require.Equal(t, "trusted-user", store.template.GetCreatedBy())
	require.Same(t, base, app.templateManager)
}

func TestLifecycleHookCreationRejectsBeforeLegacySideEffects(t *testing.T) {
	for _, name := range []string{"no_services", "no_creator", "disabled", "scheduled"} {
		t.Run(name, func(t *testing.T) {
			app, store, _, _ := newHookCreationApplication(t)
			c := creationHookConf()
			req := &expt.CreateExperimentRequest{WorkspaceID: 7, Name: gptr.Of("creation"), EvalSetID: gptr.Of(int64(11)), EvalSetVersionID: gptr.Of(int64(11)), ExptType: gptr.Of(domain.ExptType_Online), LifecycleHookConf: c}
			want := "hook configuration storage failed"
			switch name {
			case "no_services":
				app.hooks = nil
			case "no_creator":
				app.hooks.Configs = store.hookApplicationConfigStore
			case "disabled":
				c.Before.Enabled = gptr.Of(true)
				c.Before.InvokeHTTPInfo = &domain.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}
				app.hooks.Runtime = hookApplicationRuntime{}
				want = "HOOK_FEATURE_DISABLED"
			case "scheduled":
				req.TriggerType = gptr.Of(" schedule ")
				want = "HOOK_SCHEDULE_IDENTITY_UNAVAILABLE"
			}
			out, err := app.CreateExperiment(hookCreationContext(), req)
			require.Nil(t, out)
			require.ErrorContains(t, err, want)
			require.Nil(t, store.experiment)
			require.Equal(t, int64(0), app.idgen.(*hookCreationIDs).next.Load())
		})
	}
}

func TestLifecycleHookCreationOpenAPITemplateAndScheduleBoundary(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		t.Run(fmt.Sprint(scheduled), func(t *testing.T) {
			app, store, _, auth := newHookCreationApplication(t)
			metric := metricmocks.NewMockOpenAPIEvaluationMetrics(gomock.NewController(t))
			metric.EXPECT().EmitOpenAPIMetric(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			api := &EvalOpenAPIApplication{experimentApp: app, exptTemplateManager: app.templateManager, auth: auth, metric: metric}
			req := &openapi.CreateExptTemplateOApiRequest{WorkspaceID: gptr.Of(int64(7)), Meta: &openapidomain.ExptTemplateMeta{Name: gptr.Of("template")}, LifecycleHookConf: &openapidomain.LifecycleHookConf{Before: &openapidomain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":1}`)}}}
			if scheduled {
				_, err := app.CreateExperimentTemplate(hookCreationContext(), &expt.CreateExperimentTemplateRequest{WorkspaceID: 7, Meta: &domain.ExptTemplateMeta{Name: gptr.Of("scheduled")}, ExptInfo: &domain.ExptInfo{CronActivate: gptr.Of(true)}, LifecycleHookConf: creationHookConf()})
				require.ErrorContains(t, err, "HOOK_SCHEDULE_IDENTITY_UNAVAILABLE")
				require.Nil(t, store.template)
				return
			}
			out, err := api.CreateExptTemplateOApi(hookCreationContext(), req)
			require.NoError(t, err)
			require.NotNil(t, store.template)
			require.Equal(t, `{"keep":1}`, out.Data.ExperimentTemplate.LifecycleHookConf.Before.GetParametersJSON())
		})
	}
}

type hookSubmitIDsFailure struct {
	idgen.IIDGenerator
	err error
}

func (g hookSubmitIDsFailure) GenID(context.Context) (int64, error) { return 0, g.err }

func TestLifecycleHookCreationManualTemplatePersistsBeforeRunBoundary(t *testing.T) {
	for _, mode := range []string{"internal", "openapi", "scheduled", "untrusted"} {
		t.Run(mode, func(t *testing.T) {
			app, store, _, auth := newHookCreationApplication(t)
			ctrl := gomock.NewController(t)
			templates := repomocks.NewMockIExptTemplateRepo(ctrl)
			tracker := lwtmocks.NewMockILatestWriteTracker(ctrl)
			template := &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 42, WorkspaceID: 7, Name: "source", ExptType: entity.ExptType_Online}, TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 11, EvalSetVersionID: 11}}
			if mode == "scheduled" {
				template.ExptInfo = &entity.ExptInfo{CronActivate: true}
			}
			templates.EXPECT().MGetByID(gomock.Any(), []int64{42}, int64(7)).Return([]*entity.ExptTemplate{template}, nil)
			tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(42)).Return(false)
			app.templateManager = service.NewExptTemplateManager(templates, nil, nil, nil, service.NewEvaluationSetServiceImpl(hookCreationDatasetRPC{}), nil, tracker, nil, nil, nil, nil, nil)
			store.record.Config = &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"source":42}`)}, After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":"after"}`)}}
			stopped := errors.New("run ID storage unavailable")
			app.idgen = hookSubmitIDsFailure{err: stopped}
			ctx := hookCreationContext()
			if mode == "untrusted" {
				ctx = context.Background()
			}
			var err error
			if mode == "openapi" {
				metric := metricmocks.NewMockOpenAPIEvaluationMetrics(ctrl)
				metric.EXPECT().EmitOpenAPIMetric(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				api := &EvalOpenAPIApplication{experimentApp: app, exptTemplateManager: app.templateManager, manager: app.manager, auth: auth, metric: metric}
				_, err = api.SubmitExptFromTemplateOApi(ctx, &openapi.SubmitExptFromTemplateOApiRequest{WorkspaceID: gptr.Of(int64(7)), TemplateID: gptr.Of(int64(42)), Name: gptr.Of("manual"), LifecycleHookConf: &openapidomain.LifecycleHookConf{Before: &openapidomain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"override":true}`)}}})
			} else {
				_, err = app.SubmitExptFromTemplate(ctx, &expt.SubmitExptFromTemplateRequest{WorkspaceID: 7, TemplateID: 42, Name: gptr.Of("manual"), Session: &common.Session{UserID: gptr.Of(int64(999))}, LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"override":true}`)}}})
			}
			if mode == "scheduled" || mode == "untrusted" {
				require.ErrorContains(t, err, "HOOK_SCHEDULE_IDENTITY_UNAVAILABLE")
				require.Nil(t, store.experiment)
				return
			}
			require.ErrorIs(t, err, stopped)
			require.NotNil(t, store.experiment)
			require.Equal(t, `{"override":true}`, *store.input.Config.Before.ParametersJSON)
			require.Equal(t, `{"keep":"after"}`, *store.input.Config.After.ParametersJSON)
			require.Equal(t, "manual", store.experiment.TriggerType)
			require.Equal(t, "trusted-user", store.experiment.CreatedBy)
			require.Equal(t, int64(42), store.experiment.ExptTemplateMeta.ID)
		})
	}
}

func TestLifecycleHookCreationEnabledConfig(t *testing.T) {
	app, store, _, _ := newHookCreationApplication(t)
	c := creationHookConf()
	c.Before.Enabled = gptr.Of(true)
	c.Before.InvokeHTTPInfo = &domain.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}
	out, err := app.CreateExperiment(hookCreationContext(), &expt.CreateExperimentRequest{WorkspaceID: 7, Name: gptr.Of("enabled"), ExptType: gptr.Of(domain.ExptType_Online), EvalSetID: gptr.Of(int64(11)), EvalSetVersionID: gptr.Of(int64(11)), LifecycleHookConf: c})
	require.NoError(t, err)
	require.True(t, out.Experiment.LifecycleHookConf.Before.GetEnabled())
	require.Equal(t, int32(180), *store.input.Config.Before.TimeoutSeconds)
	require.Equal(t, "http", string(*store.input.Config.Before.AccessProtocol))
}
