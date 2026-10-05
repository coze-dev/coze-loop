// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/stretchr/testify/require"
)

type scheduleCallbackFunc func(context.Context) (*rpc.VerifiedScheduledCallback, error)

func (f scheduleCallbackFunc) Verify(ctx context.Context) (*rpc.VerifiedScheduledCallback, error) {
	return f(ctx)
}

type scheduleInstanceFunc func(context.Context, *rpc.VerifiedScheduledCallback, *entity.ExptTemplateScheduleBinding) (*rpc.ScheduledInstanceOwnership, error)

func (f scheduleInstanceFunc) Verify(ctx context.Context, m *rpc.VerifiedScheduledCallback, b *entity.ExptTemplateScheduleBinding) (*rpc.ScheduledInstanceOwnership, error) {
	return f(ctx, m, b)
}

type scheduleAuthFunc func(context.Context, *rpc.VerifiedScheduledRunBinding) error

func (f scheduleAuthFunc) AuthorizeScheduledRun(ctx context.Context, b *rpc.VerifiedScheduledRunBinding) error {
	return f(ctx, b)
}

type scheduleTemplateRead struct {
	repo.IExptTemplateScheduleStore
	read func() (*repo.ExptTemplateScheduleState, error)
}

func (r scheduleTemplateRead) Read(context.Context, entity.ExptTemplateScheduleBindingKey) (*repo.ExptTemplateScheduleState, error) {
	return r.read()
}

type scheduleIDs struct {
	idgen.IIDGenerator
	calls *[]string
}

func (g scheduleIDs) GenMultiIDs(context.Context, int) ([]int64, error) {
	*g.calls = append(*g.calls, "ids")
	return []int64{30, 40, 50}, nil
}

type schedulePreparation struct {
	calls   *[]string
	failure error
}

func (p schedulePreparation) Resolve(context.Context, *repo.ExptTemplateScheduleState) (*service.ScheduledTemplateResources, error) {
	*p.calls = append(*p.calls, "resources")
	return &service.ScheduledTemplateResources{Permissions: []rpc.ScheduledRunAuthorizationResource{{SpaceID: 99, ObjectID: "71", EntityType: rpc.AuthEntityType_EvaluationSet, Action: "read"}}}, nil
}
func (p schedulePreparation) Prepare(_ context.Context, tr entity.ScheduledRunTrigger, state *repo.ExptTemplateScheduleState, _ *service.ScheduledTemplateResources) (*entity.PreparedScheduledExpt, error) {
	*p.calls = append(*p.calls, "prepare")
	return &entity.PreparedScheduledExpt{Binding: *state.Binding, TemplateRevision: state.Revision}, nil
}
func (p schedulePreparation) Publish(context.Context, entity.ScheduledRunTrigger) error {
	*p.calls = append(*p.calls, "publish")
	return p.failure
}
func (p schedulePreparation) Abort(context.Context, entity.ScheduledRunTrigger, *service.ScheduledTemplateResources) {
}

type scheduleWriter struct{ calls *[]string }

func (w scheduleWriter) Write(context.Context, db.Provider, entity.ScheduledRunTrigger, string, *entity.PreparedScheduledExpt) error {
	*w.calls = append(*w.calls, "sql")
	return nil
}

type scheduleTriggers struct {
	calls   *[]string
	trigger entity.ScheduledRunTrigger
	lost    error
}

func (r *scheduleTriggers) Reserve(context.Context, *entity.ExptTemplateScheduleBinding, string, entity.ScheduledRunTriggerIDs) (*entity.ScheduledRunTrigger, error) {
	*r.calls = append(*r.calls, "reserve")
	cp := r.trigger
	return &cp, nil
}
func (r *scheduleTriggers) Commit(ctx context.Context, b *entity.ExptTemplateScheduleBinding, _ string, fn repo.ScheduledRunSQLSubmit) (*entity.ScheduledRunTrigger, error) {
	*r.calls = append(*r.calls, "commit")
	if r.trigger.Status != "submitted" {
		if err := fn(ctx, r.trigger, b.UserID, nil); err != nil {
			return nil, err
		}
		r.trigger.Status = "submitted"
	}
	if r.lost != nil {
		return nil, r.lost
	}
	cp := r.trigger
	return &cp, nil
}

func scheduleHandlerFixture(t *testing.T) (ExperimentScheduleDependencies, *expt.SubmitScheduledExptFromTemplateRequest, *[]string, *scheduleTriggers) {
	calls := new([]string)
	b := &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: "binding", Version: 2, UserID: "bound-user", IdentityType: "fornax_user", SpaceID: 10, TemplateID: 20, ExecutionScope: "local", Namespace: "ns", Group: "group", BizKey: "safe-key", JobID: "job", Enabled: true, BoundAt: time.Unix(100, 0), Callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
	tr := &scheduleTriggers{calls: calls, trigger: entity.ScheduledRunTrigger{ID: 30, BindingID: b.BindingID, BindingVersion: b.Version, InstanceID: "100", SpaceID: 10, TemplateID: 20, ExperimentID: 40, RunID: 50, Status: "pending", CreatedAt: time.Unix(101, 0)}}
	d := ExperimentScheduleDependencies{Triggers: tr, Writer: scheduleWriter{calls}, Prepare: schedulePreparation{calls: calls}, IDs: scheduleIDs{calls: calls}}
	d.Callback = scheduleCallbackFunc(func(context.Context) (*rpc.VerifiedScheduledCallback, error) {
		*calls = append(*calls, "authenticate")
		return &rpc.VerifiedScheduledCallback{CallerPSM: "scheduler.worker", SignerPSM: "scheduler.worker", Method: b.Callback.Method, Namespace: "ns", ExecutionScope: "local", Region: "region", InstanceID: "100"}, nil
	})
	d.Templates = scheduleTemplateRead{read: func() (*repo.ExptTemplateScheduleState, error) {
		*calls = append(*calls, "template")
		return &repo.ExptTemplateScheduleState{Template: &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10}, ExptInfo: &entity.ExptInfo{CronActivate: true}}, Binding: b, Revision: "revision", Config: &entity.HookConfigRecord{Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true)}}}}, nil
	}}
	d.Instance = scheduleInstanceFunc(func(_ context.Context, m *rpc.VerifiedScheduledCallback, got *entity.ExptTemplateScheduleBinding) (*rpc.ScheduledInstanceOwnership, error) {
		*calls = append(*calls, "instance")
		require.Equal(t, "bound-user", got.UserID)
		return &rpc.ScheduledInstanceOwnership{InstanceID: m.InstanceID, JobID: b.JobID, JobHistoryID: "history", BindingID: b.BindingID, BindingVersion: b.Version, ExecutionScope: b.ExecutionScope}, nil
	})
	d.Authorizer = scheduleAuthFunc(func(_ context.Context, in *rpc.VerifiedScheduledRunBinding) error {
		*calls = append(*calls, "authorize")
		require.Equal(t, "bound-user", in.Binding.UserID)
		require.Equal(t, int64(99), in.Resources[0].SpaceID)
		return nil
	})
	return d, &expt.SubmitScheduledExptFromTemplateRequest{WorkspaceID: gptr.Of(int64(10)), TemplateID: gptr.Of(int64(20)), BindingID: gptr.Of("binding"), BindingVersion: gptr.Of(int64(2))}, calls, tr
}

func TestScheduledHandlerOrderedCommitAndReplay(t *testing.T) {
	d, req, calls, tr := scheduleHandlerFixture(t)
	a, err := NewExperimentScheduleApplication(d)
	require.NoError(t, err)
	resp, err := a.SubmitScheduledExptFromTemplate(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int64(40), resp.GetExperimentID())
	require.Equal(t, int64(50), resp.GetRunID())
	require.Equal(t, []string{"authenticate", "template", "instance", "resources", "authorize", "ids", "reserve", "prepare", "commit", "sql", "publish"}, *calls)
	*calls = nil
	_, err = a.SubmitScheduledExptFromTemplate(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "submitted", tr.trigger.Status)
	require.NotContains(t, *calls, "prepare")
	require.NotContains(t, *calls, "sql")
}

func TestScheduledHandlerLostCommitReceipt(t *testing.T) {
	d, req, calls, tr := scheduleHandlerFixture(t)
	tr.lost = errors.New("private SQL diagnostic")
	a, err := NewExperimentScheduleApplication(d)
	require.NoError(t, err)
	_, err = a.SubmitScheduledExptFromTemplate(context.Background(), req)
	require.ErrorIs(t, err, tr.lost)
	require.NotContains(t, err.Error(), "private")
	require.NotContains(t, *calls, "publish")
	tr.lost = nil
	*calls = nil
	resp, err := a.SubmitScheduledExptFromTemplate(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int64(50), resp.GetRunID())
	require.NotContains(t, *calls, "sql")
	require.Contains(t, *calls, "publish")
}

func TestScheduledHandlerAuthorizationFailsBeforeEffects(t *testing.T) {
	d, req, calls, _ := scheduleHandlerFixture(t)
	d.Authorizer = scheduleAuthFunc(func(context.Context, *rpc.VerifiedScheduledRunBinding) error {
		return rpc.ErrScheduledRunAuthorizationDenied
	})
	a, err := NewExperimentScheduleApplication(d)
	require.NoError(t, err)
	_, err = a.SubmitScheduledExptFromTemplate(context.Background(), req)
	require.ErrorIs(t, err, rpc.ErrScheduledRunAuthorizationDenied)
	for _, effect := range []string{"ids", "reserve", "prepare", "sql", "publish"} {
		require.NotContains(t, *calls, effect)
	}
}

func TestScheduledHandlerMissingAuthenticationProvider(t *testing.T) {
	d, _, _, _ := scheduleHandlerFixture(t)
	d.Callback = nil
	a, err := NewExperimentScheduleApplication(d)
	require.Error(t, err)
	require.Nil(t, a)
}

func TestScheduledHandlerRejectsStaleOrDisabledMetadata(t *testing.T) {
	for _, mode := range []string{"old_request_version", "wrong_namespace", "old_instance", "missing_history", "disabled", "cron_off"} {
		t.Run(mode, func(t *testing.T) {
			d, req, calls, _ := scheduleHandlerFixture(t)
			switch mode {
			case "old_request_version":
				req.BindingVersion = gptr.Of(int64(1))
			case "wrong_namespace":
				original := d.Callback
				d.Callback = scheduleCallbackFunc(func(ctx context.Context) (*rpc.VerifiedScheduledCallback, error) {
					m, err := original.Verify(ctx)
					m.Namespace = "other"
					return m, err
				})
			case "old_instance", "missing_history":
				original := d.Instance
				d.Instance = scheduleInstanceFunc(func(ctx context.Context, m *rpc.VerifiedScheduledCallback, b *entity.ExptTemplateScheduleBinding) (*rpc.ScheduledInstanceOwnership, error) {
					o, err := original.Verify(ctx, m, b)
					if mode == "old_instance" {
						o.BindingVersion = 1
					} else {
						o.JobHistoryID = ""
					}
					return o, err
				})
			case "disabled", "cron_off":
				original := d.Templates
				d.Templates = scheduleTemplateRead{read: func() (*repo.ExptTemplateScheduleState, error) {
					s, err := original.Read(context.Background(), entity.ExptTemplateScheduleBindingKey{})
					if mode == "disabled" {
						s.Binding.Enabled = false
					} else {
						s.Template.ExptInfo.CronActivate = false
					}
					return s, err
				}}
			}
			a, err := NewExperimentScheduleApplication(d)
			require.NoError(t, err)
			_, err = a.SubmitScheduledExptFromTemplate(context.Background(), req)
			require.Error(t, err)
			for _, effect := range []string{"resources", "authorize", "ids", "reserve", "prepare", "sql", "publish"} {
				require.NotContains(t, *calls, effect)
			}
		})
	}
}
