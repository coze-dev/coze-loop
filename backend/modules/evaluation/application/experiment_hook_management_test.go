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

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestLifecycleHookEmptyExperimentList(t *testing.T) {
	denied := errors.New("space permission denied")
	for _, hooksEnabled := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			items   []*entity.Experiment
			authErr error
		}{
			{name: "nil"},
			{name: "empty", items: []*entity.Experiment{}},
			{name: "space_denied", authErr: denied},
		} {
			t.Run(fmt.Sprintf("hooks=%t/%s", hooksEnabled, tc.name), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				manager := servicemocks.NewMockIExptManager(ctrl)
				spaceAuth := auth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID: "7", SpaceID: 7,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(tc.authErr)
				if tc.authErr == nil {
					manager.EXPECT().List(gomock.Any(), int32(1), int32(5), int64(7), gomock.Any(), gomock.Any(), gomock.Any()).After(spaceAuth).Return(tc.items, int64(0), nil)
				}
				// Match the commercial provider's empty-batch rejection, keeping application authorization real.
				auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Len(0)).Return(errors.New("permission check with null action objects")).AnyTimes()
				storage := &hookApplicationConfigStore{}
				summaries := &hookApplicationSummaryStore{}
				app := &experimentApplication{auth: auth, manager: manager}
				if hooksEnabled {
					app.hooks = &ExperimentHookApplicationDependencies{Configs: storage, Summaries: summaries, ExecutionScope: "test-scope"}
				}
				out, err := app.ListExperiments(context.Background(), &expt.ListExperimentsRequest{WorkspaceID: 7, PageNumber: gptr.Of(int32(1)), PageSize: gptr.Of(int32(5))})
				if tc.authErr != nil {
					require.ErrorIs(t, err, denied)
					require.Nil(t, out)
				} else {
					require.NoError(t, err)
					require.NotNil(t, out)
					require.Empty(t, out.Experiments)
					require.Zero(t, out.GetTotal())
					require.Zero(t, out.BaseResp.StatusCode)
				}
				require.Zero(t, storage.batchCalls)
				require.Zero(t, storage.reads)
				require.Zero(t, summaries.calls)
			})
		}
	}
}

func TestLifecycleHookEmptyTemplateList(t *testing.T) {
	denied := errors.New("space permission denied")
	for _, hooksEnabled := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			items   []*entity.ExptTemplate
			authErr error
		}{
			{name: "nil"},
			{name: "empty", items: []*entity.ExptTemplate{}},
			{name: "space_denied", authErr: denied},
		} {
			t.Run(fmt.Sprintf("hooks=%t/%s", hooksEnabled, tc.name), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				manager := servicemocks.NewMockIExptTemplateManager(ctrl)
				spaceAuth := auth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID: "7", SpaceID: 7,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(tc.authErr)
				if tc.authErr == nil {
					manager.EXPECT().List(gomock.Any(), int32(1), int32(5), int64(7), gomock.Any(), gomock.Any(), gomock.Any()).After(spaceAuth).Return(tc.items, int64(0), nil)
				}
				auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Len(0)).Return(errors.New("permission check with null action objects")).AnyTimes()
				storage := &hookApplicationConfigStore{}
				app := &experimentApplication{auth: auth, templateManager: manager}
				if hooksEnabled {
					app.hooks = &ExperimentHookApplicationDependencies{Configs: storage, ExecutionScope: "test-scope"}
				}
				out, err := app.ListExperimentTemplates(context.Background(), &expt.ListExperimentTemplatesRequest{WorkspaceID: 7, PageNumber: gptr.Of(int32(1)), PageSize: gptr.Of(int32(5))})
				if tc.authErr != nil {
					require.ErrorIs(t, err, denied)
					require.Nil(t, out)
				} else {
					require.NoError(t, err)
					require.NotNil(t, out)
					require.Empty(t, out.ExperimentTemplates)
					require.Zero(t, out.GetTotal())
					require.Zero(t, out.BaseResp.StatusCode)
				}
				require.Zero(t, storage.batchCalls)
				require.Zero(t, storage.reads)
			})
		}
	}
}

func TestLifecycleHookReadNonEmptyGuards(t *testing.T) {
	denied := errors.New("resource permission denied")
	for _, tc := range []struct {
		name       string
		id, space  int64
		nilElement bool
		want       error
	}{
		{name: "denied", id: 42, space: 7, want: denied},
		{name: "nil_element", nilElement: true, want: entity.ErrHookConfigStorage},
		{name: "invalid_id", space: 7, want: entity.ErrHookConfigStorage},
		{name: "wrong_space", id: 42, space: 8, want: entity.ErrHookConfigStorage},
	} {
		for _, template := range []bool{false, true} {
			t.Run(fmt.Sprintf("template=%t/%s", template, tc.name), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				if tc.want == denied {
					entityType := rpc.AuthEntityType_EvaluationExperiment
					if template {
						entityType = rpc.AuthEntityType_EvaluationExptTemplate
					}
					auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), []*rpc.AuthorizationWithoutSPIParam{{
						ObjectID: "42", SpaceID: 7, ResourceSpaceID: 7, OwnerID: gptr.Of(""),
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(entityType)}},
					}}).Return(denied)
				}
				storage := &hookApplicationConfigStore{}
				summaries := &hookApplicationSummaryStore{}
				app := &experimentApplication{auth: auth, hooks: &ExperimentHookApplicationDependencies{Configs: storage, Summaries: summaries, ExecutionScope: "test-scope"}}
				x := &entity.Experiment{ID: tc.id, SpaceID: tc.space}
				tpl := &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: tc.id, WorkspaceID: tc.space}}
				if tc.nilElement {
					x, tpl = nil, nil
				}
				var err error
				if template {
					err = app.readTemplateHooks(context.Background(), []*entity.ExptTemplate{tpl}, []*domain.ExptTemplate{{}}, 7)
				} else {
					err = app.readExperimentHooks(context.Background(), []*entity.Experiment{x}, []*domain.Experiment{{}}, 7)
				}
				require.ErrorIs(t, err, tc.want)
				require.Zero(t, storage.batchCalls)
				require.Zero(t, storage.reads)
				require.Zero(t, summaries.calls)
			})
		}
	}
}

func TestLifecycleHookUpdateRejectsMissingStorage(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage := repomocks.NewMockIExperimentRepo(ctrl)
	tracker := lwtmocks.NewMockILatestWriteTracker(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	auditor := auditmocks.NewMockIAuditService(ctrl)
	tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(42)).Return(false).AnyTimes()
	storage.EXPECT().MGetByID(gomock.Any(), []int64{42}, int64(7)).Return([]*entity.Experiment{{ID: 42, SpaceID: 7, Name: "original"}}, nil)
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	auditor.EXPECT().Audit(gomock.Any(), gomock.Any()).Return(audit.AuditRecord{}, nil).AnyTimes()
	legacyWrite := errors.New("legacy write reached")
	storage.EXPECT().Update(gomock.Any(), gomock.Any()).Return(legacyWrite).AnyTimes()
	manager := service.NewExptManager(nil, storage, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, auditor, nil, nil, tracker, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app := &experimentApplication{manager: manager, auth: auth}
	_, err := app.UpdateExperiment(context.Background(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, Name: gptr.Of("original"), LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false)}}})
	require.ErrorIs(t, err, entity.ErrHookConfigStorage, "new config must not be silently ignored by a legacy handler")
}

type hookApplicationConfigStore struct {
	record        entity.HookConfigRecord
	reads, writes int
	owner         hookcomponent.ConfigOwner
	writeErr      error
	experiment    *entity.Experiment
	template      *entity.ExptTemplate
	batchCalls    int
	rejectAudit   bool
	nameTaken     bool
}

func (s *hookApplicationConfigStore) UpdateExperimentWithHookConfig(ctx context.Context, owner hookcomponent.ConfigOwner, x *entity.Experiment, in entity.HookConfigUpdateInput) error {
	_, err := s.UpdateConfig(ctx, owner, in)
	if err != nil {
		return err
	}
	if s.experiment != nil {
		s.experiment.Name = x.Name
		s.experiment.Description = x.Description
		if x.NotificationConf != nil {
			s.experiment.NotificationConf = x.NotificationConf
		}
	}
	return nil
}
func (s *hookApplicationConfigStore) UpdateTemplateWithHookConfig(ctx context.Context, owner hookcomponent.ConfigOwner, x *entity.ExptTemplate, _ []*entity.ExptTemplateEvaluatorRef, in entity.HookConfigUpdateInput) error {
	_, err := s.UpdateConfig(ctx, owner, in)
	if err == nil {
		s.template = x
	}
	return err
}
func (s *hookApplicationConfigStore) MGetConfigs(_ context.Context, owners []hookcomponent.ConfigOwner) ([]repo.HookConfigReadResult, error) {
	s.batchCalls++
	out := make([]repo.HookConfigReadResult, len(owners))
	for i, owner := range owners {
		s.owner = owner
		out[i] = repo.HookConfigReadResult{Owner: owner, Record: &s.record}
	}
	return out, nil
}

func (s *hookApplicationConfigStore) GetConfig(_ context.Context, owner hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	s.reads++
	s.owner = owner
	return &s.record, nil
}
func (s *hookApplicationConfigStore) UpdateConfig(_ context.Context, owner hookcomponent.ConfigOwner, in entity.HookConfigUpdateInput) (bool, error) {
	s.writes++
	s.owner = owner
	if s.writeErr != nil {
		return false, s.writeErr
	}
	if in.ExpectedRevision != s.record.Revision || in.KeyID != "operator-key" {
		return false, entity.ErrHookStoreConflict
	}
	resolved, err := entity.ResolveLifecycleHookConf(s.record.Config, in.Config)
	if err != nil {
		return false, err
	}
	s.record.Config = resolved
	s.record.Revision = "next"
	return true, nil
}

type hookApplicationRuntime struct{ enabled bool }

func (r hookApplicationRuntime) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	return entity.HookRuntimeConfig{AdmissionEnabled: r.enabled}, nil
}

type hookApplicationSummaries struct{ repo.IHookSummaryRepo }

func newHookUpdateApplication(t *testing.T, got *entity.Experiment, storage *hookApplicationConfigStore, admission bool, authErr error) *experimentApplication {
	t.Helper()
	ctrl := gomock.NewController(t)
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	stats := repomocks.NewMockIExptStatsRepo(ctrl)
	aggr := repomocks.NewMockIExptAggrResultRepo(ctrl)
	auditor := auditmocks.NewMockIAuditService(ctrl)
	tracker := lwtmocks.NewMockILatestWriteTracker(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(42)).Return(false).AnyTimes()
	experiments.EXPECT().MGetByID(gomock.Any(), []int64{42}, int64(7)).Return([]*entity.Experiment{got}, nil).AnyTimes()
	experiments.EXPECT().GetByName(gomock.Any(), gomock.Any(), int64(7)).DoAndReturn(func(context.Context, string, int64) (*entity.Experiment, bool, error) {
		return nil, storage.nameTaken, nil
	}).AnyTimes()
	experiments.EXPECT().MGetBasicByID(gomock.Any(), []int64{42}).Return([]*entity.Experiment{got}, nil).AnyTimes()
	experiments.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{42}, int64(7)).Return(nil, nil).AnyTimes()
	experiments.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, x *entity.Experiment) error {
		got.Name = x.Name
		got.Description = x.Description
		return nil
	}).AnyTimes()
	stats.EXPECT().MGet(gomock.Any(), []int64{42}, int64(7)).Return(nil, nil).AnyTimes()
	aggr.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), []int64{42}).Return(nil, errors.New("aggregate storage unavailable in fixture")).AnyTimes()
	auditor.EXPECT().Audit(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, audit.AuditParam) (audit.AuditRecord, error) {
		if storage.rejectAudit {
			return audit.AuditRecord{AuditStatus: audit.AuditStatus_Rejected}, nil
		}
		return audit.AuditRecord{}, nil
	}).AnyTimes()
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(authErr).AnyTimes()
	got.EvalSetID = 11
	got.EvalSetVersionID = 11
	got.ExptType = entity.ExptType_Online
	storage.experiment = got
	results := &service.ExptResultServiceImpl{ExptStatsRepo: stats}
	aggrSvc := service.NewExptAggrResultService(nil, aggr, experiments, nil, nil, nil, nil, nil, nil, nil, nil)
	manager := service.NewExptManager(results, experiments, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, auditor, nil, nil, tracker, nil, service.NewEvaluationSetServiceImpl(hookCreationDatasetRPC{}), nil, nil, nil, aggrSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app, err := NewExperimentApplicationWithHooks(&experimentApplication{manager: manager, auth: auth}, ExperimentHookApplicationDependencies{Configs: storage, Summaries: hookApplicationSummaries{}, Runtime: hookApplicationRuntime{admission}, ExecutionScope: "test-scope", ConfigKeyID: "operator-key"})
	require.NoError(t, err)
	return app.(*experimentApplication)
}

func TestLifecycleHookUpdateRetainsConfigWhenAdmissionDisabled(t *testing.T) {
	storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1", Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/after")}}}}}
	app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, Name: "original", Description: "keep"}, storage, false, nil)
	req := &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"secret":"retained"}`), Retry: &domain.HookRetryConf{Enabled: gptr.Of(false), MaxRetries: gptr.Of(int32(7))}}}}
	result, err := app.UpdateExperiment(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "original", result.Experiment.GetName())
	require.Equal(t, "keep", result.Experiment.GetDesc())
	require.Equal(t, `{"secret":"retained"}`, result.Experiment.LifecycleHookConf.Before.GetParametersJSON())
	require.Equal(t, int32(7), result.Experiment.LifecycleHookConf.Before.Retry.GetMaxRetries())
	require.True(t, result.Experiment.LifecycleHookConf.After.GetEnabled())
	require.Equal(t, hookcomponent.ConfigOwner{WorkspaceID: 7, ObjectID: 42, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "test-scope"}, storage.owner)
	require.Equal(t, 1, storage.writes)
}

func TestLifecycleHookEmptyUpdateAfterRunUsesLegacyMetadataPath(t *testing.T) {
	storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1", Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":1}`)}}}}
	app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, LatestRunID: 90, Name: "old"}, storage, false, nil)
	result, err := app.UpdateExperiment(context.Background(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, Name: gptr.Of("renamed"), LifecycleHookConf: &domain.LifecycleHookConf{}})
	require.NoError(t, err)
	require.Equal(t, "renamed", result.Experiment.GetName())
	require.Equal(t, `{"keep":1}`, *storage.record.Config.Before.ParametersJSON)
	require.Zero(t, storage.reads)
	require.Zero(t, storage.writes)
}

func TestLifecycleHookTemplateQueryReadsAuthorizedConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	templates := repomocks.NewMockIExptTemplateRepo(ctrl)
	tracker := lwtmocks.NewMockILatestWriteTracker(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	templates.EXPECT().MGetByID(gomock.Any(), []int64{42}, int64(7)).Return([]*entity.ExptTemplate{{Meta: &entity.ExptTemplateMeta{ID: 42, WorkspaceID: 7, Name: "template"}}}, nil)
	tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(42)).Return(false)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Any()).Return(nil).AnyTimes()
	storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":1}`)}}}}
	app := &experimentApplication{auth: auth, templateManager: service.NewExptTemplateManager(templates, nil, nil, nil, service.NewEvaluationSetServiceImpl(hookCreationDatasetRPC{}), nil, tracker, nil, nil, nil, nil, nil), hooks: &ExperimentHookApplicationDependencies{Configs: storage, ExecutionScope: "test-scope"}}
	out, err := app.BatchGetExperimentTemplate(context.Background(), &expt.BatchGetExperimentTemplateRequest{WorkspaceID: 7, TemplateIds: []int64{42}})
	require.NoError(t, err)
	require.Len(t, out.ExperimentTemplates, 1)
	require.NotNil(t, out.ExperimentTemplates[0].LifecycleHookConf)
	require.Equal(t, `{"keep":1}`, out.ExperimentTemplates[0].LifecycleHookConf.Before.GetParametersJSON())
	require.Equal(t, hookcomponent.ConfigOwner{WorkspaceID: 7, ObjectID: 42, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: "test-scope"}, storage.owner)
}

func TestLifecycleHookUpdateGuards(t *testing.T) {
	denied := errors.New("denied")
	for _, tc := range []struct {
		name                    string
		run                     int64
		conf                    *domain.LifecycleHookConf
		authErr, writeErr, want error
		reads, writes           int
	}{
		{name: "permission", conf: &domain.LifecycleHookConf{Before: &domain.HookConfig{}}, authErr: denied, want: denied},
		{name: "immutable", run: 90, conf: &domain.LifecycleHookConf{Before: &domain.HookConfig{}}, want: entity.ErrHookConfigImmutable},
		{name: "CAS", conf: &domain.LifecycleHookConf{Before: &domain.HookConfig{}}, writeErr: entity.ErrHookStoreConflict, want: entity.ErrHookStoreConflict, reads: 1, writes: 1},
		{name: "run_started_after_read", conf: &domain.LifecycleHookConf{Before: &domain.HookConfig{}}, writeErr: entity.ErrHookConfigImmutable, want: entity.ErrHookConfigImmutable, reads: 1, writes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1"}, writeErr: tc.writeErr}
			app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, LatestRunID: tc.run}, storage, false, tc.authErr)
			out, err := app.UpdateExperiment(context.Background(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, LifecycleHookConf: tc.conf})
			require.Nil(t, out)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, tc.reads, storage.reads)
			require.Equal(t, tc.writes, storage.writes)
		})
	}
	for _, raw := range []string{`[]`, `null`, `{"x":1,"x":2}`, `{"secret":"x"} trailing`} {
		t.Run(raw, func(t *testing.T) {
			storage := &hookApplicationConfigStore{}
			app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7}, storage, true, nil)
			out, err := app.UpdateExperiment(context.Background(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{ParametersJSON: gptr.Of(raw)}}})
			require.Nil(t, out)
			require.Error(t, err)
			require.Zero(t, storage.writes)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

type hookApplicationSummaryStore struct {
	calls   int
	keys    []entity.HookRunKey
	missing bool
	err     error
}

func (s *hookApplicationSummaryStore) MGetSummaries(_ context.Context, keys []entity.HookRunKey) (map[entity.HookRunKey]*entity.LifecycleHookRunSummary, error) {
	s.calls++
	s.keys = append(s.keys, keys...)
	if s.err != nil {
		return nil, s.err
	}
	result := map[entity.HookRunKey]*entity.LifecycleHookRunSummary{}
	if s.missing {
		return result, nil
	}
	for _, k := range keys {
		result[k] = &entity.LifecycleHookRunSummary{RunID: k.RunID, Before: entity.HookRunSummary{Status: entity.HookOperationRunning, Response: &spi.InvokeExperimentHookResponse{Result_: map[string]string{"old": "response"}}, Error: &spi.HookError{Code: gptr.Of("old")}}, After: entity.HookRunSummary{Status: entity.HookOperationFailed, Error: &spi.HookError{Code: gptr.Of("BUSINESS"), Retryable: gptr.Of(false)}}}
	}
	return result, nil
}

func TestLifecycleHookSummaryBatchProjection(t *testing.T) {
	for _, tc := range []struct {
		name            string
		denied, missing bool
		count           int
	}{{"current_run_and_terminal_after", false, false, 2}, {"chunked", false, false, 101}, {"denied", true, false, 2}, {"missing", false, true, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			denied := errors.New("denied")
			var authErr error
			if tc.denied {
				authErr = denied
			}
			auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Any()).Return(authErr)
			storage := &hookApplicationConfigStore{}
			summaries := &hookApplicationSummaryStore{missing: tc.missing}
			app := &experimentApplication{auth: auth, hooks: &ExperimentHookApplicationDependencies{Configs: storage, Summaries: summaries, ExecutionScope: "test-scope"}}
			dos := make([]*entity.Experiment, tc.count)
			for i := range dos {
				dos[i] = &entity.Experiment{ID: int64(42 + i), SpaceID: 7, LatestRunID: int64(900 + i), Status: entity.ExptStatus_Success}
			}
			dtos := convertor.ToExptDTOs(dos)
			err := app.readExperimentHooks(context.Background(), dos, dtos, 7)
			if tc.denied {
				require.ErrorIs(t, err, denied)
				require.Zero(t, storage.reads)
				require.Zero(t, summaries.calls)
				return
			}
			if tc.missing {
				require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
				return
			}
			require.NoError(t, err)
			require.Equal(t, (tc.count+99)/100, summaries.calls)
			require.Equal(t, (tc.count+99)/100, storage.batchCalls)
			require.Zero(t, storage.reads)
			require.Equal(t, entity.HookRunKey{WorkspaceID: 7, ExperimentID: 42, RunID: 900}, summaries.keys[0])
			require.Equal(t, "900", dtos[0].LifecycleHookSummary.GetRunID())
			require.Nil(t, dtos[0].LifecycleHookSummary.Before.Response)
			require.Nil(t, dtos[0].LifecycleHookSummary.Before.Error)
			require.Equal(t, "BUSINESS", dtos[0].LifecycleHookSummary.After.Error.GetCode())
			require.NotNil(t, dtos[0].LifecycleHookSummary.After.Error.Retryable)
		})
	}
}

func TestLifecycleHookCreateHandlerRejectsBeforeLegacyWork(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	app := &experimentApplication{auth: auth}
	out, err := app.CreateExperiment(context.Background(), &expt.CreateExperimentRequest{WorkspaceID: 7, LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(false)}}})
	require.Nil(t, out)
	require.ErrorIs(t, err, entity.ErrHookConfigStorage)
}

func TestLifecycleHookCloneChecksConfigWithoutRunSummary(t *testing.T) {
	storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}}}
	app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, LatestRunID: 90}, storage, false, nil)
	auth := app.auth.(*rpcmocks.MockIAuthProvider)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	auth.EXPECT().MAuthorizeWithoutSPI(gomock.Any(), int64(7), gomock.Any()).Return(nil).AnyTimes()
	summaries := &hookApplicationSummaryStore{err: entity.ErrHookSummaryUnavailable}
	app.hooks.Summaries = summaries
	out, err := app.CloneExperiment(context.Background(), &expt.CloneExperimentRequest{WorkspaceID: gptr.Of(int64(7)), ExptID: gptr.Of(int64(42))})
	require.Nil(t, out)
	require.ErrorIs(t, err, entity.ErrHookConfigStorage)
	require.Zero(t, summaries.calls)
}

func TestLifecycleHookEnabledUpdateAdmissionAndDefaults(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			storage := &hookApplicationConfigStore{record: entity.HookConfigRecord{Revision: "v1"}}
			app := newHookUpdateApplication(t, &entity.Experiment{ID: 42, SpaceID: 7, Name: "original"}, storage, enabled, nil)
			out, err := app.UpdateExperiment(context.Background(), &expt.UpdateExperimentRequest{WorkspaceID: 7, ExptID: 42, LifecycleHookConf: &domain.LifecycleHookConf{Before: &domain.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &domain.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}})
			if !enabled {
				require.ErrorContains(t, err, "HOOK_FEATURE_DISABLED")
				require.Zero(t, storage.writes)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "{}", out.Experiment.LifecycleHookConf.Before.GetParametersJSON())
			require.Equal(t, int32(180), out.Experiment.LifecycleHookConf.Before.GetTimeoutSeconds())
			require.Equal(t, 1, storage.writes)
		})
	}
}
