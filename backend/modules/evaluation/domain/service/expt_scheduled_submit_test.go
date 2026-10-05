// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	am "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	bm "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	im "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lm "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	rm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestScheduledCreationPreparesWithoutSQLOrNewExperimentIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	m.audit.(*am.MockIAuditService).EXPECT().Audit(gomock.Any(), gomock.Any()).Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil)
	m.benefitService.(*bm.MockIBenefitService).EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, in *benefit.CheckAndDeductEvalBenefitParams) (*benefit.CheckAndDeductEvalBenefitResult, error) {
		require.Equal(t, "bound-user", in.ConnectorUID)
		require.EqualValues(t, 40, in.ExperimentID)
		return &benefit.CheckAndDeductEvalBenefitResult{IsFreeEvaluate: gptr.Of(true)}, nil
	})
	res := &exptCreationResources{tuple: &entity.ExptTuple{EvalSet: &entity.EvaluationSet{ID: 71, SpaceID: 10, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 72, EvaluationSetID: 71, SpaceID: 10, ItemCount: 1, EvaluationSetSchema: &entity.EvaluationSetSchema{}}}}, evalSetSpaceID: 10, targetSpaceID: 10}
	req := &entity.CreateExptParam{WorkspaceID: 10, Name: "scheduled", EvalSetID: 71, EvalSetVersionID: 72, ExptTemplateID: 20, ExptType: entity.ExptType_Offline, TriggerType: "schedule", ExptConf: &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(1)}}
	prepared, err := m.prepareScheduledCreation(context.Background(), req, &entity.Session{UserID: "bound-user"}, res, []int64{40, 60})
	require.NoError(t, err)
	require.EqualValues(t, 40, prepared.experiment.ID)
	require.EqualValues(t, 60, prepared.stats.ID)
	require.Equal(t, "bound-user", prepared.experiment.CreatedBy)
	require.Equal(t, "schedule", prepared.experiment.TriggerType)
	require.Equal(t, entity.CreditCostFree, prepared.experiment.CreditCost)
	require.EqualValues(t, 0, prepared.experiment.LatestRunID)
	require.EqualValues(t, 20, prepared.experiment.ExptTemplateMeta.ID)
	// No repository, stats writer, ID generator or write tracker expectations: preparation must not call them.
}

func TestScheduledPreparationUsesBoundIdentityAndRealSnapshot(t *testing.T) {
	conf := &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}
	f := newHookManagerFixture(t, conf)
	m := f.manager.(*ExptMangerImpl)
	m.configer = scheduledChainConfig{}
	version := &entity.EvaluationSetVersion{ID: 72, SpaceID: 10, EvaluationSetID: 71, ItemCount: 1, Version: "v1", EvaluationSetSchema: &entity.EvaluationSetSchema{}}
	set := &entity.EvaluationSet{ID: 71, SpaceID: 10, EvaluationSetVersion: version}
	m.evaluationSetService.(*sm.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(71), gomock.Any(), gomock.Any()).Return(set, nil).AnyTimes()
	m.evaluationSetVersionService.(*sm.MockEvaluationSetVersionService).EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(10), int64(72), gomock.Any(), gomock.Any()).Return(version, set, nil).AnyTimes()
	m.idgenerator.(*im.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{60}, nil)
	m.idgenerator.(*im.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(61), nil)
	m.exptRepo.(*rm.MockIExperimentRepo).EXPECT().GetByName(gomock.Any(), gomock.Any(), int64(10)).Return(nil, false, nil)
	m.audit.(*am.MockIAuditService).EXPECT().Audit(gomock.Any(), gomock.Any()).Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil)
	m.benefitService.(*bm.MockIBenefitService).EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	m.mutex.(*lm.MockILocker).EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:40", gomock.Any(), time.Hour, time.Second).Return(true, "", nil)
	state := &repo.ExptTemplateScheduleState{Revision: "template-revision", Template: &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "template", ExptType: entity.ExptType_Offline}}, Config: &entity.HookConfigRecord{Config: conf}, Binding: &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: "binding", Version: 1, UserID: "bound-user", IdentityType: "fornax_user", SpaceID: 10, TemplateID: 20, ExecutionScope: "local", Namespace: "ns", Group: "group", BizKey: "key", JobID: "job", Enabled: true, BoundAt: time.Unix(100, 0), Callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}}
	p, err := NewScheduledTemplatePreparation(m, func(context.Context, *entity.ExptTemplate) (*entity.CreateExptParam, error) {
		return &entity.CreateExptParam{WorkspaceID: 10, EvalSetID: 71, EvalSetVersionID: 72, ExptTemplateID: 20, ExptType: entity.ExptType_Offline, ExptConf: &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(1)}}, nil
	})
	require.NoError(t, err)
	resources, err := p.Resolve(context.Background(), state)
	require.NoError(t, err)
	require.Len(t, resources.Permissions, 1)
	require.EqualValues(t, 10, resources.Permissions[0].SpaceID)
	tr := entity.ScheduledRunTrigger{ID: 30, SpaceID: 10, TemplateID: 20, ExperimentID: 40, RunID: 50, BindingID: "binding", BindingVersion: 1, InstanceID: "instance", Status: "pending", CreatedAt: time.Unix(101, 0)}
	in, err := p.Prepare(context.Background(), tr, state, resources)
	require.NoError(t, err)
	require.EqualValues(t, 40, in.Experiment.ID)
	require.EqualValues(t, 50, in.Run.RunLog.ID)
	require.Equal(t, "bound-user", in.Run.RunLog.CreatedBy)
	require.Equal(t, "bound-user", in.Experiment.CreatedBy)
	require.NotEmpty(t, in.ConfigCipher)
	snapshot, err := m.hooks.Codec.DecodeSnapshot(context.Background(), in.Run.Key, "local", in.Run.Snapshot)
	require.NoError(t, err)
	require.Equal(t, "bound-user", snapshot.Input().Context.GetInitiator().GetUserID())
	require.EqualValues(t, 50, snapshot.Input().Schedule.Key.RunID)
	require.Empty(t, f.wake.events, "prepare cannot publish before the SQL commit")
}

func TestScheduledResourceResolutionRequiresSourceGrant(t *testing.T) {
	for _, mode := range []string{"allowed", "revoked", "wrong_version", "wrong_consumer"} {
		t.Run(mode, func(t *testing.T) {
			conf := &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}
			f := newHookManagerFixture(t, conf)
			m := f.manager.(*ExptMangerImpl)
			ctrl := gomock.NewController(t)
			provider := cm.NewMockSharedResourceConfigProvider(ctrl)
			auth := rpcm.NewMockIAuthProvider(ctrl)
			consumer, version := int64(10), int64(72)
			if mode == "wrong_consumer" {
				consumer = 11
			}
			if mode == "wrong_version" {
				version = 73
			}
			cfg := sharedCfg(99, 71, consumer, entity.SharedAccessLevelExecute, entity.SharedVersionPolicySpecified, []int64{version})
			if mode == "revoked" {
				cfg = &entity.SharedResourceConfig{}
			}
			provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(cfg, nil)
			m.resourceAccessAuthorizer = NewResourceAccessAuthorizer(auth, provider)
			set := &entity.EvaluationSet{ID: 71, SpaceID: 99}
			m.evaluationSetService.(*sm.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(71), gomock.Any(), gomock.Any()).Return(set, nil)
			if mode == "allowed" {
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
					require.EqualValues(t, 10, p.SpaceID)
					require.EqualValues(t, 99, p.ResourceSpaceID)
					return nil
				})
				v := &entity.EvaluationSetVersion{ID: 72, SpaceID: 99, EvaluationSetID: 71, Version: "v1"}
				m.evaluationSetVersionService.(*sm.MockEvaluationSetVersionService).EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(99), int64(72), gomock.Any(), gomock.Any()).Return(v, set, nil)
			}
			b := &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: "b", Version: 1, UserID: "u", IdentityType: "fornax_user", SpaceID: 10, TemplateID: 20, ExecutionScope: "local", Namespace: "ns", Group: "g", BizKey: "key", JobID: "job", Enabled: true, BoundAt: time.Unix(100, 0), Callback: entity.ExptTemplateScheduleCallback{PSM: "p", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
			p, err := NewScheduledTemplatePreparation(m, func(context.Context, *entity.ExptTemplate) (*entity.CreateExptParam, error) {
				return &entity.CreateExptParam{WorkspaceID: 10, ExptTemplateID: 20, EvalSetID: 71, EvalSetVersionID: 72}, nil
			})
			require.NoError(t, err)
			got, err := p.Resolve(context.Background(), &repo.ExptTemplateScheduleState{Binding: b, Template: &entity.ExptTemplate{}})
			if mode != "allowed" {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Len(t, got.Permissions, 1)
			require.EqualValues(t, 99, got.Permissions[0].SpaceID)
			require.Equal(t, "71", got.Permissions[0].ObjectID)
			require.Equal(t, entity.SharedAccessLevelExecute, got.resolved.evalSetAccessLevel)
		})
	}
}

func TestScheduledTargetRejectsWrongReturnedVersion(t *testing.T) {
	f := newHookManagerFixture(t, &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}})
	m := f.manager.(*ExptMangerImpl)
	set := &entity.EvaluationSet{ID: 71, SpaceID: 10}
	version := &entity.EvaluationSetVersion{ID: 72, EvaluationSetID: 71, SpaceID: 10}
	m.evaluationSetService.(*sm.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(71), gomock.Any(), gomock.Any()).Return(set, nil)
	m.evaluationSetVersionService.(*sm.MockEvaluationSetVersionService).EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(10), int64(72), gomock.Any(), gomock.Any()).Return(version, set, nil)
	target := &entity.EvalTarget{ID: 81, SpaceID: 10, EvalTargetType: entity.EvalTargetTypeLoopPrompt, EvalTargetVersion: &entity.EvalTargetVersion{ID: 83, TargetID: 81, SpaceID: 10, EvalTargetType: entity.EvalTargetTypeLoopPrompt}}
	m.evalTargetService.(*sm.MockIEvalTargetService).EXPECT().GetEvalTarget(gomock.Any(), int64(81)).Return(target, nil).AnyTimes()
	m.evalTargetService.(*sm.MockIEvalTargetService).EXPECT().GetEvalTargetVersion(gomock.Any(), int64(10), int64(82), true).Return(target, nil)
	b := &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: "b", Version: 1, UserID: "u", IdentityType: "fornax_user", SpaceID: 10, TemplateID: 20, ExecutionScope: "local", Namespace: "ns", Group: "g", BizKey: "key", JobID: "job", Enabled: true, BoundAt: time.Unix(100, 0), Callback: entity.ExptTemplateScheduleCallback{PSM: "p", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
	p, err := NewScheduledTemplatePreparation(m, func(context.Context, *entity.ExptTemplate) (*entity.CreateExptParam, error) {
		return &entity.CreateExptParam{WorkspaceID: 10, ExptTemplateID: 20, EvalSetID: 71, EvalSetVersionID: 72, TargetID: gptr.Of(int64(81)), TargetVersionID: 82}, nil
	})
	require.NoError(t, err)
	got, err := p.Resolve(context.Background(), &repo.ExptTemplateScheduleState{Binding: b, Template: &entity.ExptTemplate{}})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Nil(t, got)
}
