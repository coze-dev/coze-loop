// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	am "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	bm "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	quota "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	red "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type scheduledChainIDs struct{ idgen.IIDGenerator }

func (scheduledChainIDs) GenID(context.Context) (int64, error) {
	return finalizationTestIDs.Add(1), nil
}
func (scheduledChainIDs) GenMultiIDs(_ context.Context, n int) ([]int64, error) {
	out := make([]int64, n)
	for i := range out {
		out[i] = finalizationTestIDs.Add(1)
	}
	return out, nil
}

type scheduledChainConfig struct{ component.IConfiger }

func (scheduledChainConfig) GetExptExecConf(context.Context, int64) *entity.ExptExecConf {
	return &entity.ExptExecConf{SpaceExptConcurLimit: 10, ZombieIntervalSecond: 3600, ExptItemEvalConf: &entity.ExptItemEvalConf{MaxItemConcurNum: 100}}
}
func (scheduledChainConfig) GetRetryYieldEnabled(context.Context, int64) bool { return false }

type scheduledChainRuntime struct{}

func (scheduledChainRuntime) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	return entity.HookRuntimeConfig{AdmissionEnabled: true}, nil
}

type scheduledChainWake struct{}

func (scheduledChainWake) PublishWake(context.Context, entity.HookWakeEvent) error { return nil }

type scheduledChainProtector struct{ cipher.AEAD }

func (p scheduledChainProtector) Protect(_ context.Context, _ string, plain []byte) ([]byte, error) {
	nonce := make([]byte, p.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return p.Seal(nonce, nonce, plain, nil), nil
}
func (p scheduledChainProtector) Unprotect(_ context.Context, _ string, data []byte) ([]byte, error) {
	if len(data) < p.NonceSize() {
		return nil, fmt.Errorf("invalid cipher")
	}
	return p.Open(nil, data[:p.NonceSize()], data[p.NonceSize():], nil)
}

type scheduledChainPublisher struct {
	events.ExptEventPublisher
	publish func(context.Context, *entity.ExptScheduleEvent) error
}

func (p scheduledChainPublisher) PublishExptScheduleEvent(ctx context.Context, e *entity.ExptScheduleEvent, _ *time.Duration) error {
	return p.publish(ctx, e)
}

type ScheduledSubmissionTestEnvironment struct {
	DB           db.Provider
	Manager      IExptManager
	IDs          idgen.IIDGenerator
	Evaluators   EvaluatorService
	Templates    repo.IExptTemplateScheduleStore
	Triggers     repo.IScheduledRunTriggerRepo
	Binding      *entity.ExptTemplateScheduleBinding
	PublishCount *int
}

// The external-package test supplies the real application; only resource RPCs and MQ are replaced.
func ScheduledSubmissionChainForTest(t *testing.T, run func(ScheduledSubmissionTestEnvironment)) {
	f := newManagerFinalizationData(t, "tx")
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	ids := scheduledChainIDs{}
	c, err := redis.NewClient(&red.Options{Addr: miniredis.RunT(t).Addr()})
	require.NoError(t, err)
	if raw, ok := redis.Unwrap(c); ok {
		t.Cleanup(func() { require.NoError(t, raw.Close()) })
	}
	m.configer = scheduledChainConfig{}
	m.idgenerator = ids
	m.mutex = lock.NewRedisLocker(c)
	m.quotaRepo = exptinfra.NewQuotaService(quota.NewQuotaDAO(c), m.mutex)
	m.exptRepo = exptinfra.NewExptRepo(exptmysql.NewExptDAO(f.p), exptmysql.NewExptEvaluatorRefDAO(f.p), ids)
	block, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	codec := hookinfra.NewStorageCodec(scheduledChainProtector{gcm})
	m.hooks = &ExptManagerHookDependencies{Initialization: exptinfra.NewHookRunInitializationRepo(f.p), Runs: exptinfra.NewHookRunRepo(f.p), Configs: exptinfra.NewHookConfigRepo(f.p, codec), Codec: codec, Runtime: scheduledChainRuntime{}, Wake: scheduledChainWake{}, ExecutionScope: "local", SnapshotKeyID: "key"}
	version := &entity.EvaluationSetVersion{ID: 72, SpaceID: f.space, EvaluationSetID: 71, ItemCount: 1, Version: "v1", EvaluationSetSchema: &entity.EvaluationSetSchema{}}
	set := &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: version}
	m.evaluationSetService.(*sm.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(71), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *int64, int64, *bool, *entity.SharedResourceOption) (*entity.EvaluationSet, error) {
		cp := *set
		return &cp, nil
	}).AnyTimes()
	m.evaluationSetVersionService.(*sm.MockEvaluationSetVersionService).EXPECT().GetEvaluationSetVersion(gomock.Any(), f.space, int64(72), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *bool, *entity.SharedResourceOption) (*entity.EvaluationSetVersion, *entity.EvaluationSet, error) {
		cp, v := *set, *version
		return &v, &cp, nil
	}).AnyTimes()
	m.audit.(*am.MockIAuditService).EXPECT().Audit(gomock.Any(), gomock.Any()).Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).AnyTimes()
	m.benefitService.(*bm.MockIBenefitService).EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).AnyTimes()
	count := 0
	var publishMu sync.Mutex
	m.publisher = scheduledChainPublisher{publish: func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		publishMu.Lock()
		defer publishMu.Unlock()
		var intent model.ExptTemplateTrigger
		require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, event.ExptID, event.ExptRunID).Take(&intent).Error)
		require.Equal(t, "submitted", intent.Status, "MQ may only observe a committed trigger")
		var row model.ExptRunLog
		require.NoError(t, f.sql.Where("id=? AND space_id=?", event.ExptRunID, f.space).Take(&row).Error)
		require.Equal(t, "scheduled-bound-user", row.CreatedBy)
		require.Equal(t, "scheduled-bound-user", event.Session.UserID)
		require.EqualValues(t, 1, gptr.Indirect(row.LifecycleHookVersion))
		count++
		return nil
	}}
	templateID := finalizationTestIDs.Add(1)
	b := &entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: fmt.Sprintf("scheduled-chain-%d", templateID), Version: 1, UserID: "scheduled-bound-user", IdentityType: "fornax_user", SpaceID: f.space, TemplateID: templateID, ExecutionScope: "local", Namespace: "ns", Group: "group", BizKey: fmt.Sprint(templateID), Enabled: true, BoundAt: time.Unix(100, 0), Callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
	key := entity.ExptTemplateScheduleBindingKey{SpaceID: f.space, TemplateID: templateID, ExecutionScope: "local"}
	store := exptinfra.NewExptTemplateScheduleStore(f.p, codec)
	t.Cleanup(func() {
		var triggers []model.ExptTemplateTrigger
		require.NoError(t, f.sql.Where("binding_id=? AND space_id=?", b.BindingID, f.space).Find(&triggers).Error)
		for _, tr := range triggers {
			for _, table := range []string{"expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_run_log", "expt_stats", "expt_evaluator_ref", "expt_turn_result_filter_key_mapping"} {
				require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, tr.ExptID).Delete(map[string]any{}).Error)
			}
			require.NoError(t, f.sql.Unscoped().Where("id=? AND space_id=? AND expt_template_id=?", tr.ExptID, f.space, templateID).Delete(&model.Experiment{}).Error)
		}
		require.NoError(t, f.sql.Where("binding_id=? AND space_id=?", b.BindingID, f.space).Delete(&model.ExptTemplateTrigger{}).Error)
		require.NoError(t, f.sql.Where("expt_template_id=? AND space_id=?", templateID, f.space).Delete(&model.ExptTemplateEvaluatorRef{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("id=? AND space_id=?", templateID, f.space).Delete(&model.ExptTemplate{}).Error)
	})
	template := &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: templateID, WorkspaceID: f.space, Name: "scheduled-template", ExptType: entity.ExptType_Offline}, TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 71, EvalSetVersionID: 72}, ExptInfo: &entity.ExptInfo{CronActivate: true}, TemplateConf: &entity.ExptTemplateConfiguration{ItemConcurNum: gptr.Of(1)}}
	conf := &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}
	require.NoError(t, store.Write(ctx, repo.ExptTemplateScheduleWrite{Key: key, Create: true, Template: template, Binding: b, Hook: entity.HookConfigUpdateInput{Config: conf, KeyID: "key"}}))
	_, err = exptinfra.NewExptTemplateScheduleBindingRepo(f.p).ActivateCAS(ctx, key, b.BindingID, 1, entity.ExptTemplateScheduleReceipt{JobID: "job", Namespace: "ns", Group: "group", BizKey: b.BizKey, Callback: b.Callback})
	require.NoError(t, err)
	b.JobID = "job"
	run(ScheduledSubmissionTestEnvironment{DB: f.p, Manager: m, IDs: ids, Evaluators: m.evaluatorService, Templates: store, Triggers: exptinfra.NewScheduledRunTriggerRepo(f.p), Binding: b, PublishCount: &count})
}

var _ hook.Protector = scheduledChainProtector{}
