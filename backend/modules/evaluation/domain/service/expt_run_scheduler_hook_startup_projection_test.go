// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	rm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

// Only the external index sink is replaced; SQL selection and payload construction are real.
type startupProjectionIndex struct {
	repo.IExptTurnResultFilterRepo
	rows       map[int64]*entity.ExptTurnResultFilterEntity
	batches    []int
	beforeSave func() error
}

func (s *startupProjectionIndex) GetExptTurnResultFilterKeyMappings(context.Context, int64, int64) ([]*entity.ExptTurnResultFilterKeyMapping, error) {
	return nil, nil
}
func (s *startupProjectionIndex) Save(_ context.Context, rows []*entity.ExptTurnResultFilterEntity) error {
	if s.beforeSave != nil {
		if err := s.beforeSave(); err != nil {
			return err
		}
	}
	s.batches = append(s.batches, len(rows))
	for _, row := range rows {
		s.rows[row.ItemID] = row
	}
	return nil
}

func (s *startupProjectionIndex) QueryItemIDStates(_ context.Context, filter *entity.ExptTurnResultFilterAccelerator) (map[int64]entity.ItemRunState, int64, error) {
	if len(filter.ItemRunStatus) != 1 || filter.ItemRunStatus[0].Op != "=" || len(filter.ItemRunStatus[0].Values) != 1 {
		return nil, 0, errors.New("fixture requires one exact status filter")
	}
	wanted := filter.ItemRunStatus[0].Values[0].(entity.ItemRunState)
	states := make(map[int64]entity.ItemRunState)
	for id, row := range s.rows {
		if row.ExptID == filter.ExptID && row.SpaceID == filter.SpaceID && row.Status == wanted {
			states[id] = row.Status
		}
	}
	return states, int64(len(states)), nil
}

type startupProjectionTemplateRepo struct{ repo.IExptTemplateRepo }

func (r startupProjectionTemplateRepo) GetByID(ctx context.Context, id int64, space *int64) (*entity.ExptTemplate, error) {
	return r.GetBasicByID(ctx, id, space)
}
func startupProjectionTemplateManager(f *startupProjectionFixture) *ExptTemplateManagerImpl {
	return &ExptTemplateManagerImpl{templateRepo: startupProjectionTemplateRepo{exptinfra.NewExptTemplateRepo(exptmysql.NewExptTemplateDAO(f.f.p), nil, activeReferenceIDs{})}}
}

type startupProjectionPublisher struct {
	events.ExptEventPublisher
	items []int64
	ticks int
}

// This fixture has no configured evaluators; the local DB has no evaluator-configuration ref table.
type startupProjectionEvaluatorRefs struct{ exptmysql.IExptEvaluatorRefDAO }

func (startupProjectionEvaluatorRefs) MGetByExptID(context.Context, []int64, int64) ([]*model.ExptEvaluatorRef, error) {
	return nil, nil
}
func (p *startupProjectionPublisher) BatchPublishExptRecordEvalEvent(_ context.Context, items []*entity.ExptItemEvalEvent, _ *time.Duration) error {
	for _, item := range items {
		p.items = append(p.items, item.EvalSetItemID)
	}
	return nil
}
func (p *startupProjectionPublisher) PublishExptScheduleEvent(context.Context, *entity.ExptScheduleEvent, *time.Duration) error {
	p.ticks++
	return nil
}

type startupProjectionFixture struct {
	f        *finalizationManagerFixture
	ms       []entity.HookExecutionManifest
	s        *ExptSchedulerImpl
	index    *startupProjectionIndex
	pub      *startupProjectionPublisher
	event    *entity.ExptScheduleEvent
	template int64
}

func newStartupProjectionFixture(t *testing.T, mode entity.ExptRunMode) *startupProjectionFixture {
	t.Helper()
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing, entity.ItemRunState_Queueing)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("mode", int32(mode)).Error)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumn("pending_cnt", 2).Error)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("result_state", 0).Error)
	template := finalizationTestIDs.Add(1)
	info, err := json.Marshal(entity.ExptInfo{CreatedExptCount: 4, LatestExptID: f.expt, LatestExptStatus: entity.ExptStatus_Pending, LatestExptStartTime: 1234567})
	require.NoError(t, err)
	require.NoError(t, f.sql.Create(&model.ExptTemplate{ID: template, SpaceID: f.space, Name: fmt.Sprint(template), ExptInfo: &info, CronActivate: true}).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", template).Delete(&model.ExptTemplate{}).Error)
	})
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("expt_template_id", template).Error)
	s := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
	s.ExptItemResultRepo, s.ExptTurnResultRepo = lazyTurnRepos(f)
	expts := exptinfra.NewExptRepo(exptmysql.NewExptDAO(f.p), startupProjectionEvaluatorRefs{}, activeReferenceIDs{})
	index := &startupProjectionIndex{rows: make(map[int64]*entity.ExptTurnResultFilterEntity)}
	ctrl := gomock.NewController(t)
	targets := sm.NewMockIEvalTargetService(ctrl)
	targets.EXPECT().BatchGetRecordByIDs(gomock.Any(), f.space, gomock.Any()).Return([]*entity.EvalTargetRecord{}, nil).AnyTimes()
	annotations := rm.NewMockIExptAnnotateRepo(ctrl)
	annotations.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), f.space, gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil).AnyTimes()
	annotations.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), f.space, gomock.Any()).Return([]*entity.AnnotateRecord{}, nil).AnyTimes()
	result := &ExptResultServiceImpl{ExperimentRepo: expts, ExptItemResultRepo: s.ExptItemResultRepo, ExptTurnResultRepo: s.ExptTurnResultRepo, exptTurnResultFilterRepo: index, ExptAnnotateRepo: annotations, evalTargetService: targets, evaluatorRecordService: activeStoredEvaluatorReader{sql: f.sql}}
	metric := mm.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptTurnResultFilterQueryLatency(f.space, gomock.Any(), false).AnyTimes()
	result.Metric = metric
	s.ResultSvc, err = result.WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	s.Idem = idemrepo.NewIdempotentService(idemredis.NewIdemDAO(f.redis))
	pub := &startupProjectionPublisher{}
	s.Publisher = pub
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: f.key.RunID, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, EvalConf: &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(1)}, TrialRunItemCount: 2, EvalSet: &entity.EvaluationSet{ID: 71, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 71, EvaluationSetID: 71}}}
	s.Manager = startupDetailManager{IExptManager: f.manager, expt: expt}
	s.schedulerModeFactory = NewSchedulerModeFactory(f.manager, s.ExptItemResultRepo, s.ExptStatsRepo, s.ExptTurnResultRepo, activeReferenceIDs{}, nil, expts, nil, s.Idem, s.Configer, pub, nil, s.ResultSvc, nil, nil, nil)
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: mode}
	exists, err := s.Idem.Exist(context.Background(), makeStartIdemKey(event))
	require.NoError(t, err)
	require.False(t, exists)
	return &startupProjectionFixture{f: f, ms: ms, s: s, index: index, pub: pub, event: event, template: template}
}

func startupProjectionInfo(t *testing.T, f *startupProjectionFixture) entity.ExptInfo {
	t.Helper()
	var row model.ExptTemplate
	require.NoError(t, f.f.sql.First(&row, f.template).Error)
	var info entity.ExptInfo
	require.NoError(t, json.Unmarshal(gptr.Indirect(row.ExptInfo), &info))
	return info
}

// Omitting the full startup upsert loses the undispatched item; omitting the guarded merge leaves Pending.
func TestHookStartupProjectionDefaultModes(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newStartupProjectionFixture(t, mode)
			m, err := f.s.schedulerModeFactory.NewSchedulerMode(mode)
			require.NoError(t, err)
			if mode == entity.EvaluationModeSubmit {
				require.IsType(t, &ExptSubmitExec{}, m)
			} else {
				require.IsType(t, &ExptTrialRunExec{}, m)
			}
			require.NoError(t, f.s.schedule(context.Background(), f.event))
			require.Len(t, f.pub.items, 1)
			require.Equal(t, 1, f.pub.ticks)
			require.NotNil(t, f.index.rows[f.pub.items[0]], "dispatched item must reach the real index builder")
			info := startupProjectionInfo(t, f)
			if info.LatestExptStatus != entity.ExptStatus_Processing {
				t.Errorf("template startup status: got %v, want Processing", info.LatestExptStatus)
			}
			require.Equal(t, int64(4), info.CreatedExptCount)
			require.Equal(t, int64(1234567), info.LatestExptStartTime)
			require.Equal(t, f.f.expt, info.LatestExptID)
			for _, item := range f.ms {
				if item.Frozen.ItemID == f.pub.items[0] {
					continue
				}
				row := f.index.rows[item.Frozen.ItemID]
				require.NotNil(t, row, "undispatched Queueing item missing from accelerator input")
				require.Equal(t, entity.ItemRunState_Queueing, row.Status)
				require.Equal(t, item.Frozen.ItemVersionID, row.ItemVersionID)
			}
			require.Equal(t, []int{2, 1}, f.index.batches)
			require.True(t, info.CronActivate)
			results, states, total, err := f.s.ResultSvc.(*ExptResultServiceImpl).ListTurnResult(context.Background(), &entity.MGetExperimentResultParam{SpaceID: f.f.space, BaseExptID: &f.f.expt, UseAccelerator: true, Page: entity.NewPage(1, 10), FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{f.f.expt: {ItemRunStatus: []*entity.FieldFilter{{Op: "=", Values: []any{entity.ItemRunState_Queueing}}}}}}, &entity.Experiment{ID: f.f.expt, SpaceID: f.f.space, ExptType: entity.ExptType_Offline})
			require.NoError(t, err)
			require.Equal(t, int64(1), total)
			require.Len(t, results, 1)
			require.Equal(t, entity.ItemRunState_Queueing, states[results[0].ItemID])
			require.NotEqual(t, f.pub.items[0], results[0].ItemID)
		})
	}
}

func TestHookStartupProjectionReplayAndFailureRetry(t *testing.T) {
	for _, cache := range []bool{true, false} {
		t.Run(fmt.Sprint(cache), func(t *testing.T) {
			f := newStartupProjectionFixture(t, entity.EvaluationModeSubmit)
			if !cache {
				f.s.Idem = nil
			}
			injected := errors.New("external index unavailable")
			f.index.beforeSave = func() error { return injected }
			_, stop, err := f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
			require.NoError(t, err)
			require.False(t, stop)
			require.Equal(t, entity.ExptStatus_Processing, startupProjectionInfo(t, f).LatestExptStatus)
			f.index.beforeSave = nil
			for i := 0; i < 2; i++ {
				initialized, stop, err := f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
				require.NoError(t, err)
				require.True(t, initialized)
				require.False(t, stop)
			}
			require.Equal(t, entity.ExptStatus_Processing, startupProjectionInfo(t, f).LatestExptStatus)
			if cache {
				require.Equal(t, []int{2}, f.index.batches)
			} else {
				require.Equal(t, []int{2, 2}, f.index.batches)
			}
			for _, m := range f.ms {
				snap := hookPersistenceRead(t, f.f, m)
				require.Equal(t, int32(entity.ItemRunState_Queueing), snap.Log.Status)
				require.Equal(t, int32(2), snap.Stats.PendingCnt)
			}
		})
	}
}

func startupProjectionNewLatest(t *testing.T, f *startupProjectionFixture) int64 {
	t.Helper()
	next := finalizationTestIDs.Add(1)
	_, err := f.f.repo.CreateRunWithHooks(context.Background(), entity.HookCreateRunInput{Key: entity.HookRunKey{WorkspaceID: f.f.space, ExperimentID: f.f.expt, RunID: next}, ExpectedLatestRunID: f.f.key.RunID, RunLog: &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.f.space, ExptID: f.f.expt, Mode: 1, Status: 3, CreatedBy: "user"}, Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"}, After: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
	require.NoError(t, err)
	return next
}

func TestHookStartupProjectionPausedIndexCancellationAndNewLatest(t *testing.T) {
	for _, action := range []string{"cancel", "new-latest", "new-template-experiment"} {
		t.Run(action, func(t *testing.T) {
			f := newStartupProjectionFixture(t, entity.EvaluationModeSubmit)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			entered, resume := make(chan struct{}), make(chan struct{})
			first := true
			f.index.beforeSave = func() error {
				if !first {
					return nil
				}
				first = false
				close(entered)
				select {
				case <-resume:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			done := make(chan error, 1)
			go func() { done <- f.s.schedule(ctx, f.event) }()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("startup never reached external index")
			}
			wantedExpt := f.f.expt
			switch action {
			case "cancel":
				require.NoError(t, f.f.manager.Kill(ctx, f.f.expt, &f.f.key.RunID, f.f.space, "during initial indexing", nil))
			case "new-latest":
				next := startupProjectionNewLatest(t, f)
				for _, table := range []any{&model.ExptItemResult{}, &model.ExptTurnResult{}} {
					require.NoError(t, f.f.sql.Model(table).Where("space_id=? AND expt_id=?", f.f.space, f.f.expt).UpdateColumns(map[string]any{"expt_run_id": next, "status": int32(entity.ItemRunState_Fail)}).Error)
				}
			case "new-template-experiment":
				wantedExpt = finalizationTestIDs.Add(1)
			}
			require.NoError(t, startupProjectionTemplateManager(f).UpdateExptInfo(ctx, f.template, f.f.space, wantedExpt, entity.ExptStatus_Terminated, 0, nil))
			infoBefore := startupProjectionInfo(t, f)
			var exptBefore model.Experiment
			require.NoError(t, f.f.sql.First(&exptBefore, f.f.expt).Error)
			before := []hookPersistenceSnapshot{hookPersistenceRead(t, f.f, f.ms[0]), hookPersistenceRead(t, f.f, f.ms[1])}
			close(resume)
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("startup failed to resume")
			}
			require.Equal(t, infoBefore, startupProjectionInfo(t, f), "late startup overwrote current template projection")
			if action != "new-template-experiment" {
				require.Empty(t, f.pub.items)
				for i, m := range f.ms {
					require.Equal(t, before[i], hookPersistenceRead(t, f.f, m))
					require.Equal(t, entity.ItemRunState(before[i].Item.Status), f.index.rows[m.Frozen.ItemID].Status, "post-race index must read current projection")
				}
				var exptAfter model.Experiment
				require.NoError(t, f.f.sql.First(&exptAfter, f.f.expt).Error)
				require.Equal(t, exptBefore, exptAfter)
			} else {
				require.Len(t, f.pub.items, 1)
			}
		})
	}
}

type startupProjectionMissingCapability struct{ repo.IHookSchedulerRepo }

func TestHookStartupProjectionMissingCapabilityFailsClosed(t *testing.T) {
	f := newStartupProjectionFixture(t, entity.EvaluationModeSubmit)
	f.s.hookScheduler = startupProjectionMissingCapability{f.s.hookScheduler}
	require.Error(t, f.s.schedule(context.Background(), f.event))
	require.Empty(t, f.pub.items)
	require.Empty(t, f.index.batches)
}

func TestHookStartupProjectionTemplateFailureRetry(t *testing.T) {
	f := newStartupProjectionFixture(t, entity.EvaluationModeSubmit)
	injected := errors.New("template write unavailable")
	require.NoError(t, f.f.sql.Callback().Update().Before("gorm:update").Register("startup-template-fail", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_template" {
			tx.AddError(injected)
		}
	}))
	t.Cleanup(func() { _ = f.f.sql.Callback().Update().Remove("startup-template-fail") })
	_, stop, err := f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
	require.NoError(t, err)
	require.False(t, stop)
	require.Equal(t, entity.ExptStatus_Pending, startupProjectionInfo(t, f).LatestExptStatus)
	require.NoError(t, f.f.sql.Callback().Update().Remove("startup-template-fail"))
	_, stop, err = f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
	require.NoError(t, err)
	require.False(t, stop)
	require.Equal(t, entity.ExptStatus_Processing, startupProjectionInfo(t, f).LatestExptStatus)
	require.Equal(t, []int{2, 2}, f.index.batches, "failed effects must not publish the completion key")
}

func TestHookStartupProjectionStorageRejectsInvalidCurrentState(t *testing.T) {
	for _, kind := range []string{"scope", "receipt", "status", "foreign-template", "corrupt-template"} {
		t.Run(kind, func(t *testing.T) {
			f := newStartupProjectionFixture(t, entity.EvaluationModeSubmit)
			scope := "local"
			switch kind {
			case "scope":
				scope = "other"
			case "receipt":
				require.NoError(t, f.f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.f.space, f.f.key.RunID).UpdateColumn("execution_initialized", false).Error)
			case "status":
				require.NoError(t, f.f.sql.Model(&model.Experiment{}).Where("id=?", f.f.expt).UpdateColumn("status", int32(entity.ExptStatus_Pending)).Error)
			case "foreign-template":
				require.NoError(t, f.f.sql.Model(&model.ExptTemplate{}).Where("id=?", f.template).UpdateColumn("space_id", f.f.space+1).Error)
			case "corrupt-template":
				require.NoError(t, f.f.sql.Model(&model.ExptTemplate{}).Where("id=?", f.template).UpdateColumn("expt_info", []byte("broken")).Error)
			}
			var before model.ExptTemplate
			require.NoError(t, f.f.sql.First(&before, f.template).Error)
			active, err := f.f.deps.Repository.(repo.IHookSchedulerStartupProjectionRepo).ProjectHookSchedulerStartup(context.Background(), f.f.key, scope)
			require.Error(t, err)
			require.Equal(t, kind == "foreign-template" || kind == "corrupt-template", active, "only optional template errors retain proven eligibility")
			var after model.ExptTemplate
			require.NoError(t, f.f.sql.First(&after, f.template).Error)
			require.Equal(t, before, after)
		})
	}
}

func TestHookStartupProjectionIndexedTemplateReads(t *testing.T) {
	f := newStartupProjectionFixture(t, entity.EvaluationModeSubmit)
	for _, query := range []struct {
		sql  string
		args []any
	}{
		{"SELECT expt_template_id FROM experiment WHERE id=? AND deleted_at IS NULL LIMIT 1 FOR UPDATE", []any{f.f.expt}},
		{"SELECT * FROM expt_template WHERE id=? AND space_id=? AND deleted_at IS NULL LIMIT 1 FOR UPDATE", []any{f.template, f.f.space}},
	} {
		var plan []struct {
			Type string `gorm:"column:type"`
			Key  string `gorm:"column:key"`
			Rows int64  `gorm:"column:rows"`
		}
		require.NoError(t, f.f.sql.Raw("EXPLAIN "+query.sql, query.args...).Scan(&plan).Error)
		require.Len(t, plan, 1)
		require.Equal(t, "const", plan[0].Type)
		require.Equal(t, "PRIMARY", plan[0].Key)
		require.Equal(t, int64(1), plan[0].Rows)
		t.Logf("EXPLAIN %s => %+v", query.sql, plan)
	}
}

// Optional projection outages must not prevent eligible work; an incomplete receipt permits repair.
func TestHookStartupProjectionBestEffortDefaultModes(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, failure := range []string{"index", "template-read", "template-write", "template-missing", "template-deleted"} {
			t.Run(fmt.Sprintf("%d/%s", mode, failure), func(t *testing.T) {
				f := newStartupProjectionFixture(t, mode)
				var template model.ExptTemplate
				require.NoError(t, f.f.sql.First(&template, f.template).Error)
				var parent model.Experiment
				require.NoError(t, f.f.sql.First(&parent, f.f.expt).Error)
				injected := errors.New("optional startup projection unavailable")
				switch failure {
				case "index":
					f.index.beforeSave = func() error { return injected }
				case "template-read":
					require.NoError(t, f.f.sql.Callback().Query().Before("gorm:query").Register("startup-optional-read", func(tx *gorm.DB) {
						if tx.Statement.Table == "expt_template" {
							tx.AddError(injected)
						}
					}))
					t.Cleanup(func() { _ = f.f.sql.Callback().Query().Remove("startup-optional-read") })
				case "template-write":
					require.NoError(t, f.f.sql.Callback().Update().Before("gorm:update").Register("startup-optional-write", func(tx *gorm.DB) {
						if tx.Statement.Table == "expt_template" {
							tx.AddError(injected)
						}
					}))
					t.Cleanup(func() { _ = f.f.sql.Callback().Update().Remove("startup-optional-write") })
				case "template-missing":
					require.NoError(t, f.f.sql.Unscoped().Where("id=?", f.template).Delete(&model.ExptTemplate{}).Error)
				case "template-deleted":
					require.NoError(t, f.f.sql.Where("id=?", f.template).Delete(&model.ExptTemplate{}).Error)
				}
				require.NoError(t, f.s.schedule(context.Background(), f.event), "optional effect failure blocked business scheduling")
				require.Len(t, f.pub.items, 1)
				require.Equal(t, 1, f.pub.ticks)
				var after model.Experiment
				require.NoError(t, f.f.sql.First(&after, f.f.expt).Error)
				require.Equal(t, parent, after, "startup must not reset experiment status/counts")
				beforeRepair := []hookPersistenceSnapshot{hookPersistenceRead(t, f.f, f.ms[0]), hookPersistenceRead(t, f.f, f.ms[1])}
				require.Equal(t, int32(1), beforeRepair[0].Stats.PendingCnt)
				require.Equal(t, int32(1), beforeRepair[0].Stats.ProcessingCnt)
				f.index.beforeSave = nil
				if failure == "template-read" {
					require.NoError(t, f.f.sql.Callback().Query().Remove("startup-optional-read"))
				}
				if failure == "template-write" {
					require.NoError(t, f.f.sql.Callback().Update().Remove("startup-optional-write"))
				}
				if failure == "template-missing" {
					require.NoError(t, f.f.sql.Create(&template).Error)
				}
				if failure == "template-deleted" {
					require.NoError(t, f.f.sql.Unscoped().Model(&model.ExptTemplate{}).Where("id=?", f.template).UpdateColumn("deleted_at", nil).Error)
				}
				initialized, stop, err := f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
				require.NoError(t, err)
				require.True(t, initialized)
				require.False(t, stop)
				info := startupProjectionInfo(t, f)
				require.Equal(t, entity.ExptStatus_Processing, info.LatestExptStatus)
				require.Equal(t, int64(4), info.CreatedExptCount)
				require.Equal(t, int64(1234567), info.LatestExptStartTime)
				for i, m := range f.ms {
					require.Equal(t, beforeRepair[i], hookPersistenceRead(t, f.f, m), "projection repair rewrote execution state/counts")
					require.NotNil(t, f.index.rows[m.Frozen.ItemID])
					require.Equal(t, entity.ItemRunState(beforeRepair[i].Item.Status), f.index.rows[m.Frozen.ItemID].Status)
				}
				batches := len(f.index.batches)
				_, stop, err = f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
				require.NoError(t, err)
				require.False(t, stop)
				require.Len(t, f.index.batches, batches, "successful repair must finish the receipt")
			})
		}
	}
}

func TestHookStartupProjectionBestEffortStillStopsCancelledRun(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, action := range []string{"cancel", "new-latest"} {
			t.Run(fmt.Sprintf("%d/%s", mode, action), func(t *testing.T) {
				f := newStartupProjectionFixture(t, mode)
				var parent model.Experiment
				var before []hookPersistenceSnapshot
				first := true
				f.index.beforeSave = func() error {
					if first {
						first = false
						if action == "cancel" {
							require.NoError(t, f.f.manager.Kill(context.Background(), f.f.expt, &f.f.key.RunID, f.f.space, "index outage cancellation", nil))
						} else {
							startupProjectionNewLatest(t, f)
						}
						require.NoError(t, f.f.sql.First(&parent, f.f.expt).Error)
						for _, m := range f.ms {
							before = append(before, hookPersistenceRead(t, f.f, m))
						}
					}
					return errors.New("index still unavailable")
				}
				require.NoError(t, f.s.schedule(context.Background(), f.event))
				require.Empty(t, f.pub.items)
				require.Zero(t, f.pub.ticks)
				require.Equal(t, entity.ExptStatus_Pending, startupProjectionInfo(t, f).LatestExptStatus)
				var after model.Experiment
				require.NoError(t, f.f.sql.First(&after, f.f.expt).Error)
				require.Equal(t, parent, after)
				for i, m := range f.ms {
					require.Equal(t, before[i], hookPersistenceRead(t, f.f, m))
				}
			})
		}
	}
}

type startupProjectionReceiptFailure struct {
	idem.IdempotentService
	read bool
}

func (s startupProjectionReceiptFailure) Exist(ctx context.Context, key string) (bool, error) {
	if s.read {
		return false, errors.New("projection receipt read unavailable")
	}
	return s.IdempotentService.Exist(ctx, key)
}
func (s startupProjectionReceiptFailure) Set(context.Context, string, time.Duration) error {
	return errors.New("projection receipt write unavailable")
}

func TestHookStartupProjectionReceiptOutageIsBestEffort(t *testing.T) {
	for _, read := range []bool{false, true} {
		t.Run(fmt.Sprint(read), func(t *testing.T) {
			f := newStartupProjectionFixture(t, entity.EvaluationModeSubmit)
			original := f.s.Idem
			f.s.Idem = startupProjectionReceiptFailure{IdempotentService: original, read: read}
			_, stop, err := f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
			require.NoError(t, err)
			require.False(t, stop)
			require.Equal(t, entity.ExptStatus_Processing, startupProjectionInfo(t, f).LatestExptStatus)
			f.s.Idem = original
			_, stop, err = f.s.resumeHookSchedulerInitialization(context.Background(), f.event)
			require.NoError(t, err)
			require.False(t, stop)
			require.Equal(t, []int{2, 2}, f.index.batches)
		})
	}
}
