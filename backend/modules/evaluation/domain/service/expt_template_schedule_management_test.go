// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	sessions "github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type templateScheduleTestStore struct {
	repo.IExptTemplateRepo
	repo.IExptTemplateScheduleBindingRepo
	state                        repo.ExptTemplateScheduleState
	writes, legacy, receiptCalls int
	readCalls, failReadAt        int
	readErr                      error
	fail, disableDuringReceipt   bool
	replyMode                    string
	closeFailureKey              string
	closed                       []string
	callback                     entity.ExptTemplateScheduleCallback
	payload                      map[string]any
}

func (s *templateScheduleTestStore) Read(context.Context, entity.ExptTemplateScheduleBindingKey) (*repo.ExptTemplateScheduleState, error) {
	s.readCalls++
	if s.readCalls == s.failReadAt {
		return nil, s.readErr
	}
	out := s.state
	raw, _ := json.Marshal(s.state.Template)
	out.Template = &entity.ExptTemplate{}
	_ = json.Unmarshal(raw, out.Template)
	conf, err := entity.ResolveLifecycleHookConf(nil, s.state.Config.Config)
	if err != nil {
		return nil, err
	}
	out.Config = &entity.HookConfigRecord{Config: conf, Revision: s.state.Config.Revision}
	if s.state.Binding != nil {
		copy := *s.state.Binding
		out.Binding = &copy
	}
	return &out, nil
}
func (s *templateScheduleTestStore) GetByID(context.Context, int64, *int64) (*entity.ExptTemplate, error) {
	return s.state.Template, nil
}
func (s *templateScheduleTestStore) GetByName(context.Context, string, int64, entity.ExptType) (*entity.ExptTemplate, bool, error) {
	return nil, false, nil
}
func (s *templateScheduleTestStore) UpdateFields(context.Context, int64, map[string]any) error {
	s.legacy++
	return nil
}
func (s *templateScheduleTestStore) Write(_ context.Context, in repo.ExptTemplateScheduleWrite) error {
	if !in.Create && in.ExpectedRevision != s.state.Revision {
		return entity.ErrHookStoreConflict
	}
	s.writes++
	s.state.Revision = fmt.Sprint(s.writes)
	if in.Create {
		s.state.Binding = nil
		s.state.Config = &entity.HookConfigRecord{}
	}
	if in.Template != nil {
		s.state.Template = in.Template
	}
	if c, ok := in.Fields["cron_activate"].(bool); ok {
		s.state.Template.ExptInfo = &entity.ExptInfo{CronActivate: c}
	}
	if n, ok := in.Fields["name"].(string); ok {
		s.state.Template.Meta.Name = n
	}
	if in.Binding != nil {
		b := *in.Binding
		s.state.Binding = &b
	}
	if in.Disable && s.state.Binding != nil {
		s.state.Binding.Enabled = false
		s.state.Binding.Version++
	}
	if in.Hook.Config != nil {
		s.state.Config.Config, _ = entity.ResolveLifecycleHookConf(s.state.Config.Config, in.Hook.Config)
	}
	return nil
}

type templateScheduleSets struct{ IEvaluationSetService }

func (templateScheduleSets) BatchGetEvaluationSets(context.Context, *int64, []int64, *bool, *entity.SharedResourceOption) ([]*entity.EvaluationSet, error) {
	return nil, nil
}

type templateScheduleLWT struct {
	platestwrite.ILatestWriteTracker
}

func (templateScheduleLWT) SetWriteFlag(context.Context, platestwrite.ResourceType, int64, ...platestwrite.SetWriteFlagOpt) {
}

type templateScheduleIDs struct{ n int64 }

func (g *templateScheduleIDs) GenID(context.Context) (int64, error) { g.n++; return g.n, nil }
func (g *templateScheduleIDs) GenMultiIDs(ctx context.Context, n int) ([]int64, error) {
	out := make([]int64, n)
	for i := range out {
		out[i], _ = g.GenID(ctx)
	}
	return out, nil
}

func TestTemplateScheduleHookWrappersAndClone(t *testing.T) {
	m, s, ctx := templateScheduleTestManager(t)
	base := m.(*ExptTemplateManagerImpl)
	base.idgen = &templateScheduleIDs{n: 100}
	base.evaluationSetService = templateScheduleSets{}
	base.lwt = templateScheduleLWT{}
	conf := s.state.Config.Config
	source := s.state.Template.ExptSource
	s.state.Binding = &entity.ExptTemplateScheduleBinding{BindingID: "source-authority", UserID: "source-owner", JobID: "source-job"}
	create, err := WithExptTemplateHookConfigCreate(base, &hookCreateCapture{}, repo.HookConfigCreateInput{WorkspaceID: 10, ExecutionScope: "local", KeyID: "key", Config: conf})
	require.NoError(t, err)
	created, err := create.Create(ctx, &entity.CreateExptTemplateParam{SpaceID: 10, Name: "new-authorized-clone", CronActivate: true, ExptSource: source}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	require.Equal(t, int64(101), created.GetID())
	require.NotNil(t, s.state.Binding)
	require.Equal(t, "trusted-operator", s.state.Binding.UserID)
	require.NotEqual(t, "source-authority", s.state.Binding.BindingID)
	require.Zero(t, s.legacy)
	before := *s.state.Binding
	_, err = base.Update(ctx, &entity.UpdateExptTemplateParam{TemplateID: 101, SpaceID: 10, Description: "omitted hook and scheduler"}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	require.Equal(t, before, *s.state.Binding)
	owner := hook.ConfigOwner{WorkspaceID: 10, ObjectID: 101, ExecutionScope: "local", Kind: hook.ConfigOwnerTemplate}
	update, err := WithExptTemplateHookConfigUpdate(base, &hookManagementCapture{}, owner, entity.HookConfigUpdateInput{KeyID: "key", Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}})
	require.NoError(t, err)
	_, err = update.Update(ctx, &entity.UpdateExptTemplateParam{TemplateID: 101, SpaceID: 10}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	require.False(t, *s.state.Config.Config.Before.Enabled)
	require.False(t, s.state.Binding.Enabled)
	require.Equal(t, 1, s.legacy, "explicitly disabling hooks restores the unchanged no-Hook scheduler path")
}
func TestTemplateSchedulePendingReadbackReplay(t *testing.T) {
	m, s, ctx := templateScheduleTestManager(t)
	s.fail = true
	_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{UserID: "body"})
	require.ErrorIs(t, err, ErrExptTemplateSchedulePending)
	before := *s.state.Binding
	s.fail = false
	require.NoError(t, ReconcileExptTemplateSchedule(context.Background(), m, 10, 20))
	require.True(t, s.state.Binding.Active())
	require.Equal(t, before.Version, s.state.Binding.Version)
	require.Equal(t, before.UserID, s.state.Binding.UserID)
	require.Equal(t, before.BindingID, s.state.Binding.BindingID)
	require.Equal(t, 2, s.receiptCalls)
	require.Zero(t, s.legacy)
}
func (s *templateScheduleTestStore) ActivateCAS(_ context.Context, _ entity.ExptTemplateScheduleBindingKey, id string, v int64, r entity.ExptTemplateScheduleReceipt, _ ...db.Option) (bool, error) {
	b := s.state.Binding
	if b == nil || !b.Enabled || b.Version != v || b.BindingID != id {
		return false, entity.ErrExptTemplateScheduleBindingConflict
	}
	if r.Callback != b.Callback || r.BizKey != b.BizKey {
		return false, entity.ErrExptTemplateScheduleBindingConflict
	}
	b.JobID = r.JobID
	return true, nil
}
func (s *templateScheduleTestStore) CreatePeriodicJob(context.Context, *rpc.CreatePeriodicJobParam) error {
	s.legacy++
	return nil
}
func (s *templateScheduleTestStore) CloseJob(_ context.Context, key string) error {
	s.closed = append(s.closed, key)
	if key == s.closeFailureKey {
		return errors.New("temporary close failure")
	}
	return nil
}

func TestTemplateScheduleDisableCloseReplay(t *testing.T) {
	m, s, ctx := templateScheduleTestManager(t)
	_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{})
	require.NoError(t, err)
	key := s.state.Binding.BizKey
	s.closeFailureKey = key
	_, err = m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(false)}, &entity.Session{})
	require.ErrorIs(t, err, ErrExptTemplateSchedulePending)
	require.False(t, s.state.Binding.Enabled)
	s.closeFailureKey = ""
	s.closed = nil
	require.NoError(t, ReconcileExptTemplateSchedule(ctx, m, 10, 20))
	require.Contains(t, s.closed, key, "reconciliation must retry closing the disabled bound job")
	require.False(t, s.state.Binding.Active())
	require.Zero(t, s.legacy)
}
func (s *templateScheduleTestStore) GetJob(context.Context, string) (*rpc.ScheduleJobDetail, error) {
	return nil, nil
}
func (s *templateScheduleTestStore) CreatePeriodicJobWithReceipt(_ context.Context, p *rpc.CreatePeriodicJobParam, ns, group string) (*entity.ExptTemplateScheduleReceipt, error) {
	s.receiptCalls++
	if s.writes == 0 || s.state.Binding == nil || s.state.Binding.JobID != "" {
		return nil, errors.New("registration before pending commit")
	}
	_ = json.Unmarshal([]byte(p.CallbackPayload), &s.payload)
	if s.fail {
		return nil, errors.New("readback temporarily unavailable")
	}
	if s.disableDuringReceipt {
		s.state.Binding.Enabled = false
		s.state.Binding.Version++
	}
	if s.replyMode == "nil" {
		return nil, nil
	}
	r := &entity.ExptTemplateScheduleReceipt{JobID: "actual-job", Namespace: ns, Group: group, BizKey: p.BizKey, Callback: s.callback}
	if s.replyMode == "wrong_target" {
		r.Callback.Method = "SubmitExptFromTemplate"
	}
	if s.replyMode == "wrong_namespace" {
		r.Namespace = "other"
	}
	return r, nil
}

func TestTemplateSchedulePartialSchedulerUpdate(t *testing.T) {
	m, s, ctx := templateScheduleTestManager(t)
	m.(*ExptTemplateManagerImpl).evaluationSetService = templateScheduleSets{}
	_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{})
	require.NoError(t, err)
	trigger := *s.state.Template.ExptSource.Scheduler.TriggerAt
	patch := &entity.ExptSource{SourceType: entity.SourceType_Evaluation, Scheduler: &entity.ExptSchedulerDO{Frequency: gptr.Of(entity.FrequencyMonday)}}
	_, err = m.Update(sessions.WithCtxUser(ctx, &sessions.User{ID: "scheduler-editor"}), &entity.UpdateExptTemplateParam{TemplateID: 20, SpaceID: 10, ExptSource: patch}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	require.True(t, s.state.Binding.Active())
	require.Equal(t, int64(2), s.state.Binding.Version)
	require.Equal(t, "scheduler-editor", s.state.Binding.UserID)
	require.Equal(t, trigger, *s.state.Template.ExptSource.Scheduler.TriggerAt)
	require.Nil(t, patch.Scheduler.Enabled)
}
func TestTemplateScheduleReceiptAndLegacyBoundaries(t *testing.T) {
	for _, mode := range []string{"nil", "wrong_target", "wrong_namespace"} {
		t.Run(mode, func(t *testing.T) {
			m, s, ctx := templateScheduleTestManager(t)
			s.replyMode = mode
			_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{})
			require.ErrorIs(t, err, ErrExptTemplateSchedulePending)
			require.False(t, s.state.Binding.Active())
			require.Zero(t, s.legacy)
		})
	}
	t.Run("historical_unbound_unrelated_edit", func(t *testing.T) {
		m, s, ctx := templateScheduleTestManager(t)
		s.state.Template.ExptInfo.CronActivate = true
		_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, Description: "metadata only"}, &entity.Session{})
		require.ErrorIs(t, err, repo.ErrHookConfigScheduleBindingRequired)
		require.Nil(t, s.state.Binding)
		require.Zero(t, s.receiptCalls)
		require.Zero(t, s.legacy)
	})
	t.Run("no_hook_legacy", func(t *testing.T) {
		m, s, _ := templateScheduleTestManager(t)
		s.state.Config.Config = nil
		_, err := m.UpdateMeta(context.Background(), &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{UserID: "legacy-user"})
		require.NoError(t, err)
		require.Equal(t, 1, s.legacy)
		require.Nil(t, s.state.Binding)
		require.Zero(t, s.receiptCalls)
	})
}
func templateScheduleTestManager(t *testing.T) (IExptTemplateManager, *templateScheduleTestStore, context.Context) {
	t.Helper()
	s := &templateScheduleTestStore{callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Cluster: "default", Method: "SubmitScheduledExptFromTemplate"}}
	s.state = repo.ExptTemplateScheduleState{Revision: "old", Config: &entity.HookConfigRecord{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}}, Template: &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "existing"}, ExptInfo: &entity.ExptInfo{}, ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Evaluation, Scheduler: &entity.ExptSchedulerDO{Enabled: gptr.Of(true), Frequency: gptr.Of(entity.FrequencyEveryDay), TriggerAt: gptr.Of(time.Now().Unix())}}}}
	m, err := WithExptTemplateScheduleManagement(&ExptTemplateManagerImpl{templateRepo: s, scheduleAdapter: s, idgen: hookCreateIDs{}}, ExptTemplateScheduleDependencies{Store: s, Bindings: s, Receipts: s, Scope: "local", Namespace: "ns", Group: "group", Callback: s.callback})
	require.NoError(t, err)
	return m, s, sessions.WithCtxUser(context.Background(), &sessions.User{ID: "trusted-operator"})
}
func TestTemplateScheduleManagementEnableAndInvalidate(t *testing.T) {
	m, s, ctx := templateScheduleTestManager(t)
	_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{UserID: "body-forgery"})
	require.NoError(t, err)
	require.Equal(t, 1, s.writes)
	require.Zero(t, s.legacy)
	require.Equal(t, 1, s.receiptCalls)
	require.True(t, s.state.Binding.Active())
	require.Equal(t, "trusted-operator", s.state.Binding.UserID)
	require.Len(t, s.payload, 4)
	require.Equal(t, float64(10), s.payload["workspace_id"])
	require.Equal(t, float64(20), s.payload["template_id"])
	require.Equal(t, float64(1), s.payload["binding_version"])
	original := *s.state.Binding
	_, err = m.UpdateMeta(sessions.WithCtxUser(ctx, &sessions.User{ID: "unrelated-editor"}), &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, Description: "unrelated"}, &entity.Session{UserID: "other"})
	require.NoError(t, err)
	require.Equal(t, original, *s.state.Binding)
	_, err = m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(false)}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	require.False(t, s.state.Binding.Enabled)
	require.Equal(t, int64(2), s.state.Binding.Version)
	_, err = m.UpdateMeta(sessions.WithCtxUser(ctx, &sessions.User{ID: "reenabler"}), &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{UserID: "body"})
	require.NoError(t, err)
	require.Equal(t, "reenabler", s.state.Binding.UserID)
	require.Equal(t, int64(3), s.state.Binding.Version)
}
func TestTemplateScheduleManagementFailureAndIdentity(t *testing.T) {
	for _, kind := range []string{"no_ctx", "readback_failed", "disable_during_register"} {
		t.Run(kind, func(t *testing.T) {
			m, s, ctx := templateScheduleTestManager(t)
			s.fail = kind == "readback_failed"
			s.disableDuringReceipt = kind == "disable_during_register"
			if kind == "no_ctx" {
				ctx = context.Background()
			}
			_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{UserID: "forged"})
			require.Error(t, err)
			if kind == "disable_during_register" {
				require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingConflict)
				require.NotErrorIs(t, err, ErrExptTemplateSchedulePending)
			}
			require.Zero(t, s.legacy)
			if kind == "no_ctx" {
				require.Zero(t, s.writes)
			} else {
				require.False(t, s.state.Binding.Active())
				require.True(t, s.state.Template.ExptInfo.CronActivate)
			}
		})
	}
}

func TestTemplateScheduleReadbackCommitBoundary(t *testing.T) {
	t.Run("before_write_is_not_saved", func(t *testing.T) {
		m, s, ctx := templateScheduleTestManager(t)
		fault := errors.New("private prewrite read failure")
		s.failReadAt = 1
		s.readErr = fault
		_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{})
		require.ErrorIs(t, err, fault)
		require.NotErrorIs(t, err, ErrExptTemplateSchedulePending)
		require.Zero(t, s.writes)
	})
	t.Run("disabled_version_survives_failed_read", func(t *testing.T) {
		m, s, ctx := templateScheduleTestManager(t)
		_, err := m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(true)}, &entity.Session{})
		require.NoError(t, err)
		fault := errors.New("private SQL readback diagnostic")
		s.failReadAt = s.readCalls + 2
		s.readErr = fault
		_, err = m.UpdateMeta(ctx, &entity.UpdateExptTemplateMetaParam{TemplateID: 20, SpaceID: 10, CronActivate: gptr.Of(false)}, &entity.Session{})
		require.ErrorIs(t, err, fault)
		require.False(t, s.state.Binding.Enabled)
		require.Equal(t, int64(2), s.state.Binding.Version)
		var pending *ExptTemplateSchedulePendingError
		require.ErrorAs(t, err, &pending)
		require.ErrorIs(t, err, ErrExptTemplateSchedulePending)
		require.Equal(t, int64(20), pending.TemplateID)
		require.Equal(t, int64(2), pending.BindingVersion)
		require.EqualError(t, err, ErrExptTemplateSchedulePending.Error())
		require.NoError(t, ReconcileExptTemplateSchedule(sessions.WithCtxUser(ctx, &sessions.User{ID: "different-retry-user"}), m, 10, pending.TemplateID))
		require.Equal(t, 2, s.writes)
		require.Equal(t, "trusted-operator", s.state.Binding.UserID)
		require.Equal(t, pending.BindingVersion, s.state.Binding.Version)
		require.False(t, s.state.Binding.Active())
	})
}
