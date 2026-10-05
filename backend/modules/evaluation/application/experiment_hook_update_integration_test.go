// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapidomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

func TestLifecycleHookManagementMixedUpdateRealHandler(t *testing.T) {
	store := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1", Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":1}`)}}}}
	app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, Name: "old", Description: "old-desc"}, store, false, nil)
	base := app.manager
	out, err := app.UpdateExperiment(hookCreationContext(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, Name: gptr.Of("renamed"), Desc: gptr.Of("new-desc"), LifecycleHookConf: creationHookConf(), NotificationConf: &domain.ExptNotificationConf{}})
	require.NoError(t, err)
	require.Equal(t, "renamed", out.Experiment.GetName())
	require.Equal(t, "new-desc", out.Experiment.GetDesc())
	require.Equal(t, `{"keep":1}`, out.Experiment.LifecycleHookConf.After.GetParametersJSON())
	require.Equal(t, 1, store.writes)
	require.NotNil(t, store.experiment.NotificationConf)
	require.Same(t, base, app.manager)
}

func newHookManagementTemplateApplication(t *testing.T) (*experimentApplication, *hookApplicationConfigStore, *entity.ExptTemplate) {
	t.Helper()
	app, _, _, _ := newHookCreationApplication(t)
	ctrl := gomock.NewController(t)
	templates := repomocks.NewMockIExptTemplateRepo(ctrl)
	tracker := lwtmocks.NewMockILatestWriteTracker(ctrl)
	original := &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 42, WorkspaceID: 7, Name: "old", Desc: "keep"}, TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 11, EvalSetVersionID: 11}}
	store := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1", Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"after":1}`)}}}, template: original}
	templates.EXPECT().MGetByID(gomock.Any(), []int64{42}, int64(7)).DoAndReturn(func(context.Context, []int64, int64) ([]*entity.ExptTemplate, error) {
		return []*entity.ExptTemplate{store.template}, nil
	}).AnyTimes()
	templates.EXPECT().GetByID(gomock.Any(), int64(42), gomock.Any()).DoAndReturn(func(context.Context, int64, *int64) (*entity.ExptTemplate, error) { return store.template, nil }).AnyTimes()
	templates.EXPECT().GetByName(gomock.Any(), gomock.Any(), int64(7), gomock.Any()).Return(nil, false, nil).AnyTimes()
	templates.EXPECT().UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, x *entity.ExptTemplate, _ []*entity.ExptTemplateEvaluatorRef) error {
		store.template = x
		return nil
	}).AnyTimes()
	tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(42)).Return(false).AnyTimes()
	tracker.EXPECT().SetWriteFlag(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	app.auth.(*rpcmocks.MockIAuthProvider).EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	app.templateManager = service.NewExptTemplateManager(templates, &hookCreationIDs{}, nil, nil, service.NewEvaluationSetServiceImpl(hookCreationDatasetRPC{}), nil, tracker, nil, nil, nil, nil, nil)
	app.hooks.Configs = store
	return app, store, original
}

func TestLifecycleHookManagementScheduleNotBypassedByOmittedHook(t *testing.T) {
	app, store, _ := newHookManagementTemplateApplication(t)
	out, err := app.UpdateExperimentTemplate(hookCreationContext(), &expt.UpdateExperimentTemplateRequest{WorkspaceID: 7, TemplateID: 42, ExptInfo: &domain.ExptInfo{CronActivate: gptr.Of(true)}})
	require.Nil(t, out)
	require.ErrorIs(t, err, repo.ErrHookConfigScheduleBindingRequired)
	require.Nil(t, store.template.ExptInfo)
	require.Zero(t, store.writes)
}

func TestLifecycleHookManagementTemplateUpdateRealHandler(t *testing.T) {
	app, store, _ := newHookManagementTemplateApplication(t)
	base := app.templateManager
	out, err := app.UpdateExperimentTemplate(hookCreationContext(), &expt.UpdateExperimentTemplateRequest{WorkspaceID: 7, TemplateID: 42, Meta: &domain.ExptTemplateMeta{Name: gptr.Of("renamed")}, LifecycleHookConf: creationHookConf()})
	require.NoError(t, err)
	require.Equal(t, "renamed", out.ExperimentTemplate.Meta.GetName())
	require.Equal(t, "keep", out.ExperimentTemplate.Meta.GetDesc())
	require.Equal(t, `{"after":1}`, out.ExperimentTemplate.LifecycleHookConf.After.GetParametersJSON())
	require.Equal(t, 1, store.writes)
	require.Same(t, base, app.templateManager)
}

type hookManagementBatchStore struct {
	hookApplicationConfigStore
	owners [][]hookcomponent.ConfigOwner
	mode   string
}

func (s *hookManagementBatchStore) MGetConfigs(_ context.Context, owners []hookcomponent.ConfigOwner) ([]repo.HookConfigReadResult, error) {
	s.owners = append(s.owners, append([]hookcomponent.ConfigOwner(nil), owners...))
	results := make([]repo.HookConfigReadResult, len(owners))
	for i, owner := range owners {
		results[i] = repo.HookConfigReadResult{Owner: owner, Record: &entity.HookConfigRecord{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{ParametersJSON: gptr.Of(fmt.Sprintf(`{"id":%d}`, owner.ObjectID))}}}}
	}
	switch s.mode {
	case "outer":
		return nil, entity.ErrHookSummaryUnavailable
	case "short":
		return results[:1], nil
	case "wrong_owner":
		results[1].Owner.ExecutionScope = "foreign"
	case "wrong_order":
		results[0], results[1] = results[1], results[0]
	case "item_error":
		results[1].Err = entity.ErrHookStoreMissing
	case "nil_record":
		results[1].Record = nil
	case "nil_config":
		results[0].Record.Config = nil
	}
	return results, nil
}

func TestLifecycleHookManagementBatchReadRealHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	templates := repomocks.NewMockIExptTemplateRepo(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	store := &hookManagementBatchStore{}
	ids := make([]int64, 101)
	rows := make([]*entity.ExptTemplate, 101)
	for i := range ids {
		ids[i] = int64(200 - i)
		rows[i] = &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: ids[i], WorkspaceID: 7}}
	}
	templates.EXPECT().MGetByID(gomock.Any(), ids, int64(7)).Return(rows, nil)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Any()).Return(nil)
	app := &experimentApplication{auth: auth, templateManager: service.NewExptTemplateManager(templates, nil, nil, nil, service.NewEvaluationSetServiceImpl(hookCreationDatasetRPC{}), nil, nil, nil, nil, nil, nil, nil), hooks: &ExperimentHookApplicationDependencies{Configs: store, ExecutionScope: "test-scope"}}
	out, err := app.BatchGetExperimentTemplate(hookCreationContext(), &expt.BatchGetExperimentTemplateRequest{WorkspaceID: 7, TemplateIds: ids})
	require.NoError(t, err)
	require.Len(t, store.owners, 2)
	require.Len(t, store.owners[0], 100)
	require.Len(t, store.owners[1], 1)
	require.Zero(t, store.reads)
	require.Equal(t, `{"id":200}`, out.ExperimentTemplates[0].LifecycleHookConf.Before.GetParametersJSON())
	require.Equal(t, `{"id":100}`, out.ExperimentTemplates[100].LifecycleHookConf.Before.GetParametersJSON())
}

func TestLifecycleHookManagementBatchIntegrityAndAuthorization(t *testing.T) {
	for _, mode := range []string{"outer", "short", "wrong_owner", "wrong_order", "item_error", "nil_record", "nil_config", "denied", "duplicates", "no_batch"} {
		t.Run(mode, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			store := &hookManagementBatchStore{mode: mode}
			var denied error
			if mode == "denied" {
				denied = errors.New("denied")
			}
			auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Any()).Return(denied)
			var configs repo.IHookConfigRepo = store
			if mode == "no_batch" {
				configs = struct{ repo.IHookConfigRepo }{store}
			}
			app := &experimentApplication{auth: auth, hooks: &ExperimentHookApplicationDependencies{Configs: configs, ExecutionScope: "test-scope"}}
			rows := []*entity.ExptTemplate{{Meta: &entity.ExptTemplateMeta{ID: 42, WorkspaceID: 7}}, {Meta: &entity.ExptTemplateMeta{ID: 43, WorkspaceID: 7}}}
			if mode == "duplicates" {
				rows = append(rows, rows[0])
			}
			dtos := convertor.ToExptTemplateDTOs(rows)
			err := app.readTemplateHooks(hookCreationContext(), rows, dtos, 7)
			require.Zero(t, store.reads)
			if mode == "nil_config" {
				require.NoError(t, err)
				require.Nil(t, dtos[0].LifecycleHookConf)
				require.NotNil(t, dtos[1].LifecycleHookConf)
				return
			}
			if mode == "duplicates" {
				require.NoError(t, err)
				require.Len(t, store.owners, 1)
				require.Len(t, store.owners[0], 2)
				require.Equal(t, `{"id":42}`, dtos[2].LifecycleHookConf.Before.GetParametersJSON())
				*dtos[0].LifecycleHookConf.Before.ParametersJSON = "changed"
				require.Equal(t, `{"id":42}`, dtos[2].LifecycleHookConf.Before.GetParametersJSON())
				return
			}
			if mode == "denied" {
				require.ErrorIs(t, err, denied)
				require.Empty(t, store.owners)
			} else if mode == "outer" {
				require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
			} else if mode == "item_error" {
				require.ErrorIs(t, err, entity.ErrHookStoreMissing)
			} else {
				require.ErrorIs(t, err, entity.ErrHookConfigStorage)
			}
			for _, dto := range dtos {
				require.Nil(t, dto.LifecycleHookConf)
			}
		})
	}
}

func TestLifecycleHookManagementKeepsNameAndAuditValidation(t *testing.T) {
	for _, name := range []string{"name_exists", "audit_rejected"} {
		t.Run(name, func(t *testing.T) {
			store := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1"}, nameTaken: name == "name_exists", rejectAudit: name == "audit_rejected"}
			app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, Name: "old"}, store, false, nil)
			out, err := app.UpdateExperiment(hookCreationContext(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, Name: gptr.Of("renamed"), LifecycleHookConf: creationHookConf()})
			require.Nil(t, out)
			require.Error(t, err)
			require.Zero(t, store.writes)
			require.Equal(t, "old", store.experiment.Name)
		})
	}
}

func TestLifecycleHookManagementOpenAPITemplateUpdateAndGuards(t *testing.T) {
	for _, mode := range []string{"success", "scheduled", "binding_race", "CAS", "permission"} {
		t.Run(mode, func(t *testing.T) {
			app, store, original := newHookManagementTemplateApplication(t)
			ctrl := gomock.NewController(t)
			metric := metricmocks.NewMockOpenAPIEvaluationMetrics(ctrl)
			metric.EXPECT().EmitOpenAPIMetric(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			if mode == "scheduled" {
				original.ExptInfo = &entity.ExptInfo{CronActivate: true}
			}
			if mode == "binding_race" {
				store.writeErr = repo.ErrHookConfigScheduleBindingRequired
			}
			if mode == "CAS" {
				store.writeErr = entity.ErrHookStoreConflict
			}
			var denied error
			if mode == "permission" {
				denied = errors.New("denied")
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(denied)
				app.auth = auth
			}
			api := &EvalOpenAPIApplication{auth: app.auth, experimentApp: app, exptTemplateManager: app.templateManager, metric: metric}
			out, err := api.UpdateExptTemplateOApi(hookCreationContext(), &openapi.UpdateExptTemplateOApiRequest{WorkspaceID: gptr.Of(int64(7)), TemplateID: gptr.Of(int64(42)), Meta: &openapidomain.ExptTemplateMeta{Name: gptr.Of("renamed")}, LifecycleHookConf: &openapidomain.LifecycleHookConf{Before: &openapidomain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"updated":1}`)}}})
			if mode == "success" {
				require.NoError(t, err)
				require.Equal(t, "renamed", out.Data.ExperimentTemplate.Meta.GetName())
				require.Equal(t, `{"after":1}`, out.Data.ExperimentTemplate.LifecycleHookConf.After.GetParametersJSON())
				require.Equal(t, 1, store.writes)
				return
			}
			require.Nil(t, out)
			require.Equal(t, "old", store.template.Meta.Name)
			if denied != nil {
				require.ErrorIs(t, err, denied)
				require.Zero(t, store.reads)
				require.Zero(t, store.writes)
			} else if mode == "CAS" {
				require.ErrorIs(t, err, entity.ErrHookStoreConflict)
			} else {
				require.ErrorIs(t, err, repo.ErrHookConfigScheduleBindingRequired)
			}
		})
	}
}
