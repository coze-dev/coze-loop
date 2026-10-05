// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	sessions "github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	mysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

type templateChainIDs struct{ next int64 }

const bindingMySQLSpace int64 = 7378265404009000

func (g *templateChainIDs) GenID(context.Context) (int64, error) { g.next++; return g.next, nil }
func (g *templateChainIDs) GenMultiIDs(ctx context.Context, n int) ([]int64, error) {
	out := make([]int64, n)
	for i := range out {
		out[i], _ = g.GenID(ctx)
	}
	return out, nil
}

type templateChainSets struct{ service.IEvaluationSetService }

func (templateChainSets) BatchGetEvaluationSets(context.Context, *int64, []int64, *bool, *entity.SharedResourceOption) ([]*entity.EvaluationSet, error) {
	return nil, nil
}

type templateChainLWT struct {
	platestwrite.ILatestWriteTracker
}

func (templateChainLWT) SetWriteFlag(context.Context, platestwrite.ResourceType, int64, ...platestwrite.SetWriteFlagOpt) {
}

type templateChainRegistrar struct {
	store         repo.IExptTemplateScheduleStore
	fail          bool
	calls, legacy int
	callback      entity.ExptTemplateScheduleCallback
}

func (r *templateChainRegistrar) CreatePeriodicJob(context.Context, *rpc.CreatePeriodicJobParam) error {
	r.legacy++
	return nil
}
func (r *templateChainRegistrar) CloseJob(context.Context, string) error { return nil }
func (r *templateChainRegistrar) GetJob(context.Context, string) (*rpc.ScheduleJobDetail, error) {
	return nil, nil
}
func (r *templateChainRegistrar) CreatePeriodicJobWithReceipt(ctx context.Context, in *rpc.CreatePeriodicJobParam, ns, group string) (*entity.ExptTemplateScheduleReceipt, error) {
	r.calls++
	var payload struct {
		SpaceID    int64  `json:"workspace_id"`
		TemplateID int64  `json:"template_id"`
		BindingID  string `json:"binding_id"`
		Version    int64  `json:"binding_version"`
	}
	if err := json.Unmarshal([]byte(in.CallbackPayload), &payload); err != nil {
		return nil, err
	}
	readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	state, err := r.store.Read(readCtx, entity.ExptTemplateScheduleBindingKey{SpaceID: payload.SpaceID, TemplateID: payload.TemplateID, ExecutionScope: "local"})
	if err != nil {
		return nil, err
	}
	if state.Binding == nil || state.Binding.JobID != "" || state.Binding.BindingID != payload.BindingID || state.Binding.Version != payload.Version || state.Binding.BizKey != in.BizKey {
		return nil, errors.New("registration must observe committed pending binding")
	}
	if r.fail {
		return nil, errors.New("scheduler readback unavailable")
	}
	return &entity.ExptTemplateScheduleReceipt{JobID: "db-chain-job", Namespace: ns, Group: group, BizKey: in.BizKey, Callback: r.callback}, nil
}

func TestTemplateScheduleManagerChainMySQL(t *testing.T) {
	require.NotEmpty(t, os.Getenv("HOOK_MYSQL_TEST_DSN"))
	p, configs, store, bindings := exptinfra.NewTemplateScheduleChainTestDependencies(t)
	ctx := sessions.WithCtxUser(context.Background(), &sessions.User{ID: "manager-db-actor"})
	s := p.NewSession(ctx, db.WithMaster())
	const templateID int64 = 7378265404009331
	var existing int64
	require.NoError(t, s.Unscoped().Model(&model.ExptTemplate{}).Where("id=?", templateID).Count(&existing).Error)
	require.Zero(t, existing)
	t.Cleanup(func() {
		require.NoError(t, s.Unscoped().Where("expt_template_id=? AND space_id=?", templateID, bindingMySQLSpace).Delete(&model.ExptTemplateEvaluatorRef{}).Error)
		require.NoError(t, s.Unscoped().Where("id=? AND space_id=? AND created_by=?", templateID, bindingMySQLSpace, "manager-db-actor").Delete(&model.ExptTemplate{}).Error)
	})
	ids := &templateChainIDs{next: templateID - 1}
	registrar := &templateChainRegistrar{store: store, fail: true, callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Cluster: "default", Method: "SubmitScheduledExptFromTemplate"}}
	legacy := exptinfra.NewExptTemplateRepo(mysql.NewExptTemplateDAO(p), mysql.NewExptTemplateEvaluatorRefDAO(p), ids)
	base := service.NewExptTemplateManager(legacy, ids, nil, nil, templateChainSets{}, nil, templateChainLWT{}, nil, nil, nil, registrar, nil)
	manager, err := service.WithExptTemplateScheduleManagement(base, service.ExptTemplateScheduleDependencies{Store: store, Bindings: bindings, Receipts: registrar, Scope: "local", Namespace: "ns", Group: "group", Callback: registrar.callback})
	require.NoError(t, err)
	creator, err := service.WithExptTemplateHookConfigCreate(manager, configs, repo.HookConfigCreateInput{WorkspaceID: bindingMySQLSpace, ExecutionScope: "local", KeyID: "key", Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}})
	require.NoError(t, err)
	_, err = creator.Create(ctx, &entity.CreateExptTemplateParam{SpaceID: bindingMySQLSpace, Name: "managed-pending", CronActivate: true, ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Evaluation, Scheduler: &entity.ExptSchedulerDO{Enabled: gptr.Of(true), Frequency: gptr.Of(entity.FrequencyEveryDay), TriggerAt: gptr.Of(time.Now().Unix())}}}, &entity.Session{UserID: "body-forgery"})
	require.ErrorIs(t, err, service.ErrExptTemplateSchedulePending)
	key := entity.ExptTemplateScheduleBindingKey{SpaceID: bindingMySQLSpace, TemplateID: templateID, ExecutionScope: "local"}
	pending, err := store.Read(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "managed-pending", pending.Template.GetName())
	require.Equal(t, "manager-db-actor", pending.Binding.UserID)
	require.True(t, *pending.Config.Config.Before.Enabled)
	require.False(t, pending.Binding.Active())
	require.Zero(t, registrar.legacy)
	registrar.fail = false
	require.NoError(t, service.ReconcileExptTemplateSchedule(context.Background(), manager, bindingMySQLSpace, templateID))
	active, err := store.Read(ctx, key)
	require.NoError(t, err)
	require.True(t, active.Binding.Active())
	require.Equal(t, pending.Binding.Version, active.Binding.Version)
	_, err = manager.Update(ctx, &entity.UpdateExptTemplateParam{TemplateID: templateID, SpaceID: bindingMySQLSpace, Description: "only-description"}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	unchanged, err := store.Read(ctx, key)
	require.NoError(t, err)
	require.Equal(t, active.Binding, unchanged.Binding)
	require.Equal(t, active.Config, unchanged.Config)
	_, err = manager.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: templateID, SpaceID: bindingMySQLSpace, CronActivate: gptr.Of(false)}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	disabled, err := store.Read(ctx, key)
	require.NoError(t, err)
	require.False(t, disabled.Binding.Enabled)
	require.False(t, disabled.Template.ExptInfo.CronActivate)
	_, err = bindings.ActivateCAS(ctx, key, active.Binding.BindingID, active.Binding.Version, entity.ExptTemplateScheduleReceipt{JobID: "db-chain-job", Namespace: "ns", Group: "group", BizKey: active.Binding.BizKey, Callback: registrar.callback})
	require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingConflict)
	ctx = sessions.WithCtxUser(ctx, &sessions.User{ID: "second-operator"})
	_, err = manager.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: templateID, SpaceID: bindingMySQLSpace, CronActivate: gptr.Of(true)}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	restarted, err := store.Read(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "second-operator", restarted.Binding.UserID)
	require.Equal(t, int64(3), restarted.Binding.Version)
	require.True(t, restarted.Binding.Active())
	require.Equal(t, 3, registrar.calls)
	require.Zero(t, registrar.legacy)
}

type templatePostcommitReadFailure struct {
	repo.IExptTemplateScheduleStore
	reads, writes, failReadAt int
	fault                     error
}

func (s *templatePostcommitReadFailure) Write(ctx context.Context, in repo.ExptTemplateScheduleWrite) error {
	s.writes++
	return s.IExptTemplateScheduleStore.Write(ctx, in)
}
func (s *templatePostcommitReadFailure) Read(ctx context.Context, key entity.ExptTemplateScheduleBindingKey) (*repo.ExptTemplateScheduleState, error) {
	s.reads++
	if s.reads == s.failReadAt {
		return nil, s.fault
	}
	return s.IExptTemplateScheduleStore.Read(ctx, key)
}

func TestTemplateSchedulePostcommitReadbackMySQL(t *testing.T) {
	require.NotEmpty(t, os.Getenv("HOOK_MYSQL_TEST_DSN"))
	for index, phase := range []string{"after_write", "after_activate"} {
		t.Run(phase, func(t *testing.T) {
			p, configs, store, bindings := exptinfra.NewTemplateScheduleChainTestDependencies(t)
			ctx := sessions.WithCtxUser(context.Background(), &sessions.User{ID: "postcommit-operator"})
			s := p.NewSession(ctx, db.WithMaster())
			templateID := int64(7378265404009491) + int64(index)*10
			var existing int64
			require.NoError(t, s.Unscoped().Model(&model.ExptTemplate{}).Where("id=?", templateID).Count(&existing).Error)
			require.Zero(t, existing)
			t.Cleanup(func() {
				require.NoError(t, s.Unscoped().Where("expt_template_id=? AND space_id=?", templateID, bindingMySQLSpace).Delete(&model.ExptTemplateEvaluatorRef{}).Error)
				require.NoError(t, s.Unscoped().Where("id=? AND space_id=? AND created_by=?", templateID, bindingMySQLSpace, "postcommit-operator").Delete(&model.ExptTemplate{}).Error)
			})
			fault := errors.New("private SQL/SDK token readback failure")
			failing := &templatePostcommitReadFailure{IExptTemplateScheduleStore: store, failReadAt: index + 1, fault: fault}
			ids := &templateChainIDs{next: templateID - 1}
			registrar := &templateChainRegistrar{store: store, callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Cluster: "default", Method: "SubmitScheduledExptFromTemplate"}}
			legacy := exptinfra.NewExptTemplateRepo(mysql.NewExptTemplateDAO(p), mysql.NewExptTemplateEvaluatorRefDAO(p), ids)
			base := service.NewExptTemplateManager(legacy, ids, nil, nil, templateChainSets{}, nil, templateChainLWT{}, nil, nil, nil, registrar, nil)
			manager, err := service.WithExptTemplateScheduleManagement(base, service.ExptTemplateScheduleDependencies{Store: failing, Bindings: bindings, Receipts: registrar, Scope: "local", Namespace: "ns", Group: "group", Callback: registrar.callback})
			require.NoError(t, err)
			creator, err := service.WithExptTemplateHookConfigCreate(manager, configs, repo.HookConfigCreateInput{WorkspaceID: bindingMySQLSpace, ExecutionScope: "local", KeyID: "key", Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}})
			require.NoError(t, err)
			created, createErr := creator.Create(ctx, &entity.CreateExptTemplateParam{SpaceID: bindingMySQLSpace, Name: "postcommit-" + phase, CronActivate: true, ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Evaluation, Scheduler: &entity.ExptSchedulerDO{Enabled: gptr.Of(true), Frequency: gptr.Of(entity.FrequencyEveryDay), TriggerAt: gptr.Of(time.Now().Unix())}}}, &entity.Session{UserID: "body-forgery"})
			require.Nil(t, created)
			require.ErrorIs(t, createErr, fault)
			key := entity.ExptTemplateScheduleBindingKey{SpaceID: bindingMySQLSpace, TemplateID: templateID, ExecutionScope: "local"}
			persisted, err := store.Read(ctx, key)
			require.NoError(t, err)
			require.Equal(t, templateID, persisted.Template.GetID())
			require.Equal(t, "postcommit-operator", persisted.Binding.UserID)
			require.Equal(t, int64(1), persisted.Binding.Version)
			require.Equal(t, index == 1, persisted.Binding.Active())
			require.Equal(t, 1, failing.writes)
			var pending *service.ExptTemplateSchedulePendingError
			require.ErrorAs(t, createErr, &pending)
			require.ErrorIs(t, createErr, service.ErrExptTemplateSchedulePending)
			require.Equal(t, templateID, pending.TemplateID)
			require.Equal(t, int64(1), pending.BindingVersion)
			require.EqualError(t, createErr, service.ErrExptTemplateSchedulePending.Error())
			require.NotContains(t, createErr.Error(), "private")
			lastID := ids.next
			require.NoError(t, service.ReconcileExptTemplateSchedule(sessions.WithCtxUser(ctx, &sessions.User{ID: "different-retry-user"}), manager, bindingMySQLSpace, pending.TemplateID))
			recovered, err := store.Read(ctx, key)
			require.NoError(t, err)
			require.True(t, recovered.Binding.Active())
			require.Equal(t, persisted.Binding.BindingID, recovered.Binding.BindingID)
			require.Equal(t, pending.BindingVersion, recovered.Binding.Version)
			require.Equal(t, "postcommit-operator", recovered.Binding.UserID)
			require.Equal(t, persisted.Config, recovered.Config)
			require.Equal(t, 1, failing.writes)
			require.Equal(t, lastID, ids.next)
			require.Equal(t, 1, registrar.calls)
			require.Zero(t, registrar.legacy)
		})
	}
}
