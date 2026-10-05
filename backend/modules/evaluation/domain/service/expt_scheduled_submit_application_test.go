// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type scheduledCallback struct{}

func (scheduledCallback) Verify(context.Context) (*rpc.VerifiedScheduledCallback, error) {
	return &rpc.VerifiedScheduledCallback{CallerPSM: "scheduler.worker", SignerPSM: "scheduler.worker", Method: "SubmitScheduledExptFromTemplate", Namespace: "ns", Region: "region", ExecutionScope: "local", InstanceID: "100"}, nil
}

type scheduledInstance struct{}

func (scheduledInstance) Verify(_ context.Context, m *rpc.VerifiedScheduledCallback, b *entity.ExptTemplateScheduleBinding) (*rpc.ScheduledInstanceOwnership, error) {
	return &rpc.ScheduledInstanceOwnership{InstanceID: m.InstanceID, JobID: b.JobID, JobHistoryID: "history", BindingID: b.BindingID, BindingVersion: b.Version, ExecutionScope: b.ExecutionScope}, nil
}

type scheduledAuthorization struct{ t *testing.T }

func (a scheduledAuthorization) AuthorizeScheduledRun(_ context.Context, in *rpc.VerifiedScheduledRunBinding) error {
	require.Equal(a.t, "scheduled-bound-user", in.Binding.UserID)
	require.Len(a.t, in.Resources, 1)
	require.Equal(a.t, "71", in.Resources[0].ObjectID)
	return nil
}

type lostScheduledCommit struct {
	repo.IScheduledRunTriggerRepo
	lost bool
}

func (r *lostScheduledCommit) Commit(ctx context.Context, b *entity.ExptTemplateScheduleBinding, i string, f repo.ScheduledRunSQLSubmit) (*entity.ScheduledRunTrigger, error) {
	tr, err := r.IScheduledRunTriggerRepo.Commit(ctx, b, i, f)
	if err == nil && !r.lost {
		r.lost = true
		return nil, errors.New("private committed acknowledgment lost")
	}
	return tr, err
}

func TestScheduledApplicationRealPrepareCommitAndLostReceiptMySQL(t *testing.T) {
	service.ScheduledSubmissionChainForTest(t, func(env service.ScheduledSubmissionTestEnvironment) {
		app := application.NewExperimentApplication(nil, nil, env.Manager, nil, nil, env.IDs, nil, nil, nil, nil, nil, nil, nil, nil, nil, env.Evaluators, nil, nil, nil, nil, nil)
		lost := &lostScheduledCommit{IScheduledRunTriggerRepo: env.Triggers}
		handler, err := application.NewExperimentScheduleService(app, application.ExperimentScheduleDependencies{Callback: scheduledCallback{}, Instance: scheduledInstance{}, Authorizer: scheduledAuthorization{t}, Templates: env.Templates, Triggers: lost, Writer: exptinfra.NewScheduledExptSubmissionWriter()})
		require.NoError(t, err)
		req := &expt.SubmitScheduledExptFromTemplateRequest{WorkspaceID: gptr.Of(env.Binding.SpaceID), TemplateID: gptr.Of(env.Binding.TemplateID), BindingID: gptr.Of(env.Binding.BindingID), BindingVersion: gptr.Of(env.Binding.Version)}
		_, err = handler.SubmitScheduledExptFromTemplate(context.Background(), req)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "private")
		require.Zero(t, *env.PublishCount)
		var original model.ExptTemplateTrigger
		s := env.DB.NewSession(context.Background())
		require.NoError(t, s.Where("binding_id=? AND space_id=?", env.Binding.BindingID, env.Binding.SpaceID).Take(&original).Error)
		require.Equal(t, "submitted", original.Status)
		resp, err := handler.SubmitScheduledExptFromTemplate(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, original.ExptID, resp.GetExperimentID())
		require.Equal(t, original.ExptRunID, resp.GetRunID())
		require.Equal(t, 1, *env.PublishCount)
		_, err = handler.SubmitScheduledExptFromTemplate(context.Background(), req)
		require.NoError(t, err)
		var n int64
		require.NoError(t, s.Model(&model.Experiment{}).Where("space_id=? AND expt_template_id=?", env.Binding.SpaceID, env.Binding.TemplateID).Count(&n).Error)
		require.EqualValues(t, 1, n)
		require.NoError(t, s.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=?", env.Binding.SpaceID, original.ExptID).Count(&n).Error)
		require.EqualValues(t, 1, n)
		key := entity.ExptTemplateScheduleBindingKey{SpaceID: env.Binding.SpaceID, TemplateID: env.Binding.TemplateID, ExecutionScope: "local"}
		state, err := env.Templates.Read(context.Background(), key)
		require.NoError(t, err)
		require.EqualValues(t, 1, state.Template.ExptInfo.CreatedExptCount, "replays cannot count the same accepted submission twice")
		require.Equal(t, original.ExptID, state.Template.ExptInfo.LatestExptID)
		_, err = exptinfra.NewExptTemplateScheduleBindingRepo(env.DB).DisableCAS(context.Background(), key, env.Binding.BindingID, env.Binding.Version)
		require.NoError(t, err)
		count := *env.PublishCount
		_, err = handler.SubmitScheduledExptFromTemplate(context.Background(), req)
		require.Error(t, err)
		require.Equal(t, count, *env.PublishCount)
	})
}

type scheduledAuthWithEdit struct {
	scheduledAuthorization
	edit func()
}

func (a scheduledAuthWithEdit) AuthorizeScheduledRun(ctx context.Context, in *rpc.VerifiedScheduledRunBinding) error {
	if err := a.scheduledAuthorization.AuthorizeScheduledRun(ctx, in); err != nil {
		return err
	}
	a.edit()
	return nil
}

func scheduledRealHandler(t *testing.T, env service.ScheduledSubmissionTestEnvironment, auth rpc.IScheduledRunAuthorizer) (expt.ExperimentScheduleService, *expt.SubmitScheduledExptFromTemplateRequest) {
	app := application.NewExperimentApplication(nil, nil, env.Manager, nil, nil, env.IDs, nil, nil, nil, nil, nil, nil, nil, nil, nil, env.Evaluators, nil, nil, nil, nil, nil)
	h, err := application.NewExperimentScheduleService(app, application.ExperimentScheduleDependencies{Callback: scheduledCallback{}, Instance: scheduledInstance{}, Authorizer: auth, Templates: env.Templates, Triggers: env.Triggers, Writer: exptinfra.NewScheduledExptSubmissionWriter()})
	require.NoError(t, err)
	return h, &expt.SubmitScheduledExptFromTemplateRequest{WorkspaceID: gptr.Of(env.Binding.SpaceID), TemplateID: gptr.Of(env.Binding.TemplateID), BindingID: gptr.Of(env.Binding.BindingID), BindingVersion: gptr.Of(env.Binding.Version)}
}

func TestScheduledApplicationRevisionRollbackThenSameIDsMySQL(t *testing.T) {
	service.ScheduledSubmissionChainForTest(t, func(env service.ScheduledSubmissionTestEnvironment) {
		s := env.DB.NewSession(context.Background())
		edited := false
		h, req := scheduledRealHandler(t, env, scheduledAuthWithEdit{scheduledAuthorization{t}, func() {
			if !edited {
				edited = true
				require.NoError(t, s.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", env.Binding.TemplateID, env.Binding.SpaceID).UpdateColumn("name", "edited-during-auth").Error)
			}
		}})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := h.SubmitScheduledExptFromTemplate(ctx, req)
		require.ErrorIs(t, err, entity.ErrHookStoreConflict)
		require.Zero(t, *env.PublishCount)
		var pending model.ExptTemplateTrigger
		require.NoError(t, s.Where("binding_id=? AND space_id=?", env.Binding.BindingID, env.Binding.SpaceID).Take(&pending).Error)
		require.Equal(t, "pending", pending.Status)
		for _, table := range []string{"experiment", "expt_run_log", "expt_stats", "expt_lifecycle_run"} {
			var n int64
			q := s.Table(table).Where("space_id=?", env.Binding.SpaceID)
			if table == "experiment" {
				q = q.Where("id=?", pending.ExptID)
			} else {
				q = q.Where("expt_id=?", pending.ExptID)
			}
			require.NoError(t, q.Count(&n).Error)
			require.Zero(t, n, table)
		}
		resp, err := h.SubmitScheduledExptFromTemplate(ctx, req)
		require.NoError(t, err, "confirmed rollback must release only its owner lease")
		require.Equal(t, pending.ExptID, resp.GetExperimentID())
		require.Equal(t, pending.ExptRunID, resp.GetRunID())
		require.Equal(t, 1, *env.PublishCount)
	})
}

func TestScheduledApplicationConcurrentSameInstanceMySQL(t *testing.T) {
	service.ScheduledSubmissionChainForTest(t, func(env service.ScheduledSubmissionTestEnvironment) {
		h, req := scheduledRealHandler(t, env, scheduledAuthorization{t})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		start := make(chan struct{})
		done := make(chan error, 4)
		for i := 0; i < 4; i++ {
			go func() { <-start; _, err := h.SubmitScheduledExptFromTemplate(ctx, req); done <- err }()
		}
		close(start)
		success := 0
		for i := 0; i < 4; i++ {
			if <-done == nil {
				success++
			}
		}
		require.Positive(t, success)
		resp, err := h.SubmitScheduledExptFromTemplate(context.Background(), req)
		require.NoError(t, err)
		s := env.DB.NewSession(context.Background())
		var n int64
		require.NoError(t, s.Model(&model.ExptTemplateTrigger{}).Where("binding_id=? AND space_id=?", env.Binding.BindingID, env.Binding.SpaceID).Count(&n).Error)
		require.EqualValues(t, 1, n)
		require.NoError(t, s.Model(&model.Experiment{}).Where("space_id=? AND expt_template_id=?", env.Binding.SpaceID, env.Binding.TemplateID).Count(&n).Error)
		require.EqualValues(t, 1, n)
		require.NoError(t, s.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=?", env.Binding.SpaceID, resp.GetExperimentID()).Count(&n).Error)
		require.EqualValues(t, 1, n)
		var tr model.ExptTemplateTrigger
		require.NoError(t, s.Where("binding_id=? AND space_id=?", env.Binding.BindingID, env.Binding.SpaceID).Take(&tr).Error)
		require.Equal(t, resp.GetRunID(), tr.ExptRunID)
	})
}

type scheduledRegistrationProbe struct {
	server.Server
	calls   int
	name    string
	handler expt.ExperimentScheduleService
}

func (p *scheduledRegistrationProbe) RegisterService(info *serviceinfo.ServiceInfo, h interface{}, _ ...server.RegisterOption) error {
	p.calls++
	p.name = info.ServiceName
	p.handler = h.(expt.ExperimentScheduleService)
	return nil
}

func TestScheduledApplicationRegistrationRequiresAuthenticationMySQL(t *testing.T) {
	service.ScheduledSubmissionChainForTest(t, func(env service.ScheduledSubmissionTestEnvironment) {
		app := application.NewExperimentApplication(nil, nil, env.Manager, nil, nil, env.IDs, nil, nil, nil, nil, nil, nil, nil, nil, nil, env.Evaluators, nil, nil, nil, nil, nil)
		deps := application.ExperimentScheduleDependencies{Instance: scheduledInstance{}, Authorizer: scheduledAuthorization{t}, Templates: env.Templates, Triggers: env.Triggers, Writer: exptinfra.NewScheduledExptSubmissionWriter()}
		probe := new(scheduledRegistrationProbe)
		require.Error(t, application.RegisterExperimentScheduleService(probe, app, deps))
		require.Zero(t, probe.calls)
		deps.Callback = scheduledCallback{}
		require.NoError(t, application.RegisterExperimentScheduleService(probe, app, deps))
		require.Equal(t, 1, probe.calls)
		require.Equal(t, "ExperimentScheduleService", probe.name)
		resp, err := probe.handler.SubmitScheduledExptFromTemplate(context.Background(), &expt.SubmitScheduledExptFromTemplateRequest{WorkspaceID: gptr.Of(env.Binding.SpaceID), TemplateID: gptr.Of(env.Binding.TemplateID), BindingID: gptr.Of(env.Binding.BindingID), BindingVersion: gptr.Of(env.Binding.Version)})
		require.NoError(t, err)
		require.Positive(t, resp.GetExperimentID())
		require.Equal(t, 1, *env.PublishCount)
	})
}
