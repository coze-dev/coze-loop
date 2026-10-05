// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"github.com/cloudwego/kitex/server"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt/experimentscheduleservice"
	convert "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

var ErrScheduledSubmissionUnavailable = errors.New("scheduled submission unavailable")

type ExperimentScheduleDependencies struct {
	Callback   rpc.ScheduledCallbackVerifier
	Instance   rpc.ScheduledInstanceVerifier
	Authorizer rpc.IScheduledRunAuthorizer
	Templates  repo.IExptTemplateScheduleStore
	Triggers   repo.IScheduledRunTriggerRepo
	Writer     repo.IScheduledExptSubmissionWriter
	Prepare    service.IScheduledTemplatePreparation
	IDs        idgen.IIDGenerator
}

type experimentScheduleApplication struct {
	deps ExperimentScheduleDependencies
}

func NewExperimentScheduleApplication(deps ExperimentScheduleDependencies) (expt.ExperimentScheduleService, error) {
	for _, dep := range []any{deps.Callback, deps.Instance, deps.Authorizer, deps.Templates, deps.Triggers, deps.Writer, deps.Prepare, deps.IDs} {
		if dep == nil {
			return nil, ErrScheduledSubmissionUnavailable
		}
		v := reflect.ValueOf(dep)
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
			if v.IsNil() {
				return nil, ErrScheduledSubmissionUnavailable
			}
		}
	}
	return &experimentScheduleApplication{deps: deps}, nil
}

type scheduledSubmissionFailure struct{ cause error }

func (e scheduledSubmissionFailure) Error() string { return ErrScheduledSubmissionUnavailable.Error() }
func (e scheduledSubmissionFailure) Unwrap() error { return e.cause }

func (a *experimentScheduleApplication) SubmitScheduledExptFromTemplate(ctx context.Context, req *expt.SubmitScheduledExptFromTemplateRequest) (*expt.SubmitScheduledExptFromTemplateResponse, error) {
	fail := func(err error) (*expt.SubmitScheduledExptFromTemplateResponse, error) {
		return nil, scheduledSubmissionFailure{cause: err}
	}
	if a == nil || ctx == nil || req == nil || req.GetWorkspaceID() <= 0 || req.GetTemplateID() <= 0 || req.GetBindingID() == "" || req.GetBindingVersion() <= 0 {
		return fail(ErrScheduledSubmissionUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	d := a.deps
	m, err := d.Callback.Verify(ctx)
	if err != nil {
		return fail(err)
	}
	if m == nil || m.CallerPSM == "" || m.SignerPSM == "" || m.Method != "SubmitScheduledExptFromTemplate" || m.Namespace == "" || m.ExecutionScope == "" || m.Region == "" || m.InstanceID == "" {
		return fail(rpc.ErrScheduledCallbackUntrusted)
	}
	meta := *m
	key := entity.ExptTemplateScheduleBindingKey{SpaceID: req.GetWorkspaceID(), TemplateID: req.GetTemplateID(), ExecutionScope: meta.ExecutionScope}
	state, err := d.Templates.Read(ctx, key)
	if err != nil {
		return fail(err)
	}
	if state == nil || state.Template == nil || state.Template.GetID() != key.TemplateID || state.Template.GetSpaceID() != key.SpaceID || state.Template.ExptInfo == nil || !state.Template.ExptInfo.CronActivate || state.Revision == "" || !state.Binding.Active() || state.Config == nil || state.Config.Config == nil {
		return fail(entity.ErrHookStoreConflict)
	}
	b := *state.Binding
	if b.BindingID != req.GetBindingID() || b.Version != req.GetBindingVersion() || b.SpaceID != key.SpaceID || b.TemplateID != key.TemplateID || b.ExecutionScope != meta.ExecutionScope || b.Namespace != meta.Namespace || b.Callback.Method != meta.Method {
		return fail(entity.ErrHookStoreConflict)
	}
	conf := state.Config.Config
	if !(conf.Before != nil && gptr.Indirect(conf.Before.Enabled)) && !(conf.After != nil && gptr.Indirect(conf.After.Enabled)) {
		return fail(entity.ErrHookStoreConflict)
	}
	owned, err := d.Instance.Verify(ctx, &meta, &b)
	if err != nil {
		return fail(err)
	}
	if owned == nil || owned.InstanceID != meta.InstanceID || owned.JobID != b.JobID || owned.JobHistoryID == "" || owned.BindingID != b.BindingID || owned.BindingVersion != b.Version || owned.ExecutionScope != b.ExecutionScope {
		return fail(rpc.ErrScheduledInstanceMismatch)
	}
	resources, err := d.Prepare.Resolve(ctx, state)
	if err != nil {
		return fail(err)
	}
	if resources == nil || resources.Permissions == nil {
		return fail(rpc.ErrScheduledRunAuthorizationInvalid)
	}
	if err := d.Authorizer.AuthorizeScheduledRun(ctx, &rpc.VerifiedScheduledRunBinding{Binding: &b, Resources: resources.Permissions}); err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	ids, err := d.IDs.GenMultiIDs(ctx, 3)
	if err != nil {
		return fail(err)
	}
	if len(ids) != 3 {
		return fail(ErrScheduledSubmissionUnavailable)
	}
	tr, err := d.Triggers.Reserve(ctx, &b, owned.InstanceID, entity.ScheduledRunTriggerIDs{TriggerID: ids[0], ExperimentID: ids[1], RunID: ids[2]})
	if err != nil {
		return fail(err)
	}
	if !scheduledTriggerMatches(tr, &b, owned.InstanceID) {
		return fail(entity.ErrHookStoreConflict)
	}
	var prepared *entity.PreparedScheduledExpt
	if tr.Status == entity.ScheduledRunTriggerPending {
		prepared, err = d.Prepare.Prepare(ctx, *tr, state, resources)
		if err != nil {
			return fail(err)
		}
		if prepared == nil {
			return fail(ErrScheduledSubmissionUnavailable)
		}
	}
	committed, err := d.Triggers.Commit(ctx, &b, owned.InstanceID, func(ctx context.Context, actual entity.ScheduledRunTrigger, user string, tx db.Provider) error {
		if prepared == nil || actual.ID != tr.ID || actual.ExperimentID != tr.ExperimentID || actual.RunID != tr.RunID || user != b.UserID {
			return entity.ErrHookStoreConflict
		}
		return d.Writer.Write(ctx, tx, actual, user, prepared)
	})
	if err != nil {
		d.Prepare.Abort(ctx, *tr, resources)
		return fail(err)
	}
	if !scheduledTriggerMatches(committed, &b, owned.InstanceID) || committed.Status != entity.ScheduledRunTriggerSubmitted || committed.ID != tr.ID || committed.ExperimentID != tr.ExperimentID || committed.RunID != tr.RunID {
		return fail(entity.ErrHookStoreConflict)
	}
	if err := d.Prepare.Publish(ctx, *committed); err != nil {
		return fail(err)
	}
	return &expt.SubmitScheduledExptFromTemplateResponse{ExperimentID: gptr.Of(committed.ExperimentID), RunID: gptr.Of(committed.RunID), BaseResp: base.NewBaseResp()}, nil
}

func scheduledTriggerMatches(tr *entity.ScheduledRunTrigger, b *entity.ExptTemplateScheduleBinding, instance string) bool {
	return tr != nil && tr.Validate() == nil && tr.BindingID == b.BindingID && tr.BindingVersion == b.Version && tr.SpaceID == b.SpaceID && tr.TemplateID == b.TemplateID && tr.InstanceID == instance
}

// NewExperimentScheduleService binds the real Manager preparation path; authentication remains mandatory.
func NewExperimentScheduleService(app IExperimentApplication, deps ExperimentScheduleDependencies) (expt.ExperimentScheduleService, error) {
	a, ok := app.(*experimentApplication)
	if !ok || a == nil {
		return nil, ErrScheduledSubmissionUnavailable
	}
	prepared, err := service.NewScheduledTemplatePreparation(a.manager, a.scheduledTemplateCreateParam, a.prepareScheduledTarget)
	if err != nil {
		return nil, scheduledSubmissionFailure{cause: err}
	}
	deps.Prepare = prepared
	if deps.IDs == nil {
		deps.IDs = a.idgen
	}
	return NewExperimentScheduleApplication(deps)
}

// RegisterExperimentScheduleService is opt-in; no service is registered if a verifier/provider is absent.
func RegisterExperimentScheduleService(s server.Server, app IExperimentApplication, deps ExperimentScheduleDependencies) error {
	handler, err := NewExperimentScheduleService(app, deps)
	if err != nil {
		return err
	}
	if s == nil {
		return ErrScheduledSubmissionUnavailable
	}
	return experimentscheduleservice.RegisterService(s, handler)
}

func (a *experimentApplication) scheduledTemplateCreateParam(ctx context.Context, t *entity.ExptTemplate) (*entity.CreateExptParam, error) {
	r := convert.OpenAPITemplateToSubmitExperimentRequest(t, t.GetName(), t.GetSpaceID())
	if r == nil {
		return nil, entity.ErrHookStoreConflict
	}
	create := &expt.CreateExperimentRequest{WorkspaceID: r.WorkspaceID, Name: r.Name, Desc: r.Desc, ExptTemplateID: r.ExptTemplateID, EvalSetID: r.EvalSetID, EvalSetVersionID: r.EvalSetVersionID, TargetID: r.TargetID, TargetVersionID: r.TargetVersionID, EvaluatorVersionIds: r.EvaluatorVersionIds, EvaluatorIDVersionList: r.EvaluatorIDVersionList, TargetFieldMapping: r.TargetFieldMapping, EvaluatorFieldMapping: r.EvaluatorFieldMapping, TargetRuntimeParam: r.TargetRuntimeParam, ItemConcurNum: r.ItemConcurNum, ItemRetryNum: r.ItemRetryNum, EvaluatorsConcurNum: r.EvaluatorsConcurNum, ExptType: r.ExptType, EnableWeightedScore: r.EnableWeightedScore, EnableExtractTrajectory: r.EnableExtractTrajectory, NotificationConf: r.NotificationConf, VerificationConfig: r.VerificationConfig, TriggerType: gptr.Of("schedule")}
	ids, configs, weights, err := a.resolveEvaluatorVersionIDsFromCreateReq(ctx, create)
	if err != nil {
		return nil, err
	}
	create.EvaluatorVersionIds = ids
	create.EvaluatorScoreWeights = weights
	return convert.ConvertCreateReq(create, configs)
}

func (a *experimentApplication) prepareScheduledTarget(ctx context.Context, e *entity.Experiment) error {
	if !e.IsSandboxAgentTarget() {
		return nil
	}
	if a.sandboxSchedulerAdapter == nil {
		return ErrScheduledSubmissionUnavailable
	}
	dto := convert.ToExptDTO(e)
	concurrency := sandboxTaskConcurrencyForMode(e.EvalConf.ItemConcurNum, sandboxCountModeForExperimentDTO(dto))
	return a.initSandboxTask(ctx, "scheduled submit", e.ID, concurrency, e.SpaceID, sandboxTenantForExperimentDTO(dto))
}
