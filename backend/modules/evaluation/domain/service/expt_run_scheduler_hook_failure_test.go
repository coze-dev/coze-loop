// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	evalmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	targetmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	targetconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/convertor"
	targetmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

type failureTickContextKey struct{}

type failureTargetRepo struct {
	repo.IEvalTargetRepo
	dao       targetmysql.EvalTargetRecordDAO
	version   *entity.EvalTarget
	onVersion func(context.Context) error
}

func (r failureTargetRepo) GetEvalTargetVersion(ctx context.Context, space, id int64) (*entity.EvalTarget, error) {
	if space != r.version.SpaceID || id != r.version.EvalTargetVersion.ID {
		return nil, fmt.Errorf("wrong target identity")
	}
	if r.onVersion != nil {
		if err := r.onVersion(ctx); err != nil {
			return nil, err
		}
	}
	return r.version, nil
}
func (r failureTargetRepo) BatchGetEvalTargetVersion(ctx context.Context, space int64, ids []int64) ([]*entity.EvalTarget, error) {
	var out []*entity.EvalTarget
	for _, id := range ids {
		v, err := r.GetEvalTargetVersion(ctx, space, id)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (r failureTargetRepo) GetEvalTargetRecordByIDAndSpaceID(ctx context.Context, space, id int64) (*entity.EvalTargetRecord, error) {
	row, err := r.dao.GetByIDAndSpaceID(ctx, id, space)
	if err != nil {
		return nil, err
	}
	return targetconvert.EvalTargetRecordPO2DO(row)
}
func (r failureTargetRepo) ListEvalTargetRecordByIDsAndSpaceID(ctx context.Context, space int64, ids []int64) ([]*entity.EvalTargetRecord, error) {
	rows, err := r.dao.ListByIDsAndSpaceID(ctx, ids, space)
	if err != nil {
		return nil, err
	}
	var out []*entity.EvalTargetRecord
	for _, row := range rows {
		v, err := targetconvert.EvalTargetRecordPO2DO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (r failureTargetRepo) SaveEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord, _ *bool) error {
	row, err := targetconvert.EvalTargetRecordDO2PO(record)
	if err != nil {
		return err
	}
	return r.dao.Save(ctx, row)
}

type failureSandbox struct {
	rpc.ISandboxSchedulerAdapter
	onGet     func(context.Context) error
	onDestroy func(context.Context, *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error)
	destroys  atomic.Int64
}

func (s *failureSandbox) Get(ctx context.Context, in *rpc.SandboxGetRequest) (*rpc.SandboxGetResponse, error) {
	if s.onGet != nil {
		if err := s.onGet(ctx); err != nil {
			return nil, err
		}
	}
	return &rpc.SandboxGetResponse{ExecuteInfo: &rpc.SandboxExecuteInfo{ExecuteID: in.ExecuteID, Status: rpc.SandboxExecuteStatusFailed}}, nil
}
func (s *failureSandbox) Destroy(ctx context.Context, in *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
	if in.DestroyType != rpc.SandboxDestroyTypeExecute || len(in.ExecuteIDs) != 1 {
		return nil, fmt.Errorf("non-exact destroy")
	}
	s.destroys.Add(1)
	if s.onDestroy != nil {
		return s.onDestroy(ctx, in)
	}
	return &rpc.SandboxDestroyResponse{AffectedCount: 1}, nil
}

type failureConfig struct {
	schedulerHookConfig
	pause func(context.Context) error
}

func (c failureConfig) GetConsumerConf(ctx context.Context) *entity.ExptConsumerConf {
	if c.pause != nil {
		_ = c.pause(ctx)
	}
	return &entity.ExptConsumerConf{ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1, AsyncZombieSecond: 1}}}
}

func schedulerFailureFixture(t *testing.T) (*finalizationManagerFixture, entity.HookExecutionManifest, *ExptSchedulerImpl, *entity.Experiment, *failureTargetRepo, *failureSandbox, int64) {
	t.Helper()
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Processing)
	m := ms[0]
	old := time.Now().UTC().Add(-4 * time.Hour).Truncate(time.Millisecond)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("updated_at", old).Error)
	targetID := finalizationTestIDs.Add(1)
	record := &entity.EvalTargetRecord{ID: targetID, SpaceID: f.space + 500, TargetID: 91, TargetVersionID: 92, ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), Ext: map[string]string{"sandbox_execute_id": "original-execute"},
		EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{"answer": {Text: gptr.Of("keep answer")}}, Ext: map[string]string{"artifact": "keep-artifact"}, EvalTargetSteps: []*entity.EvalTargetStep{{StepName: "keep-step"}}}}
	persistActiveTerminationTarget(t, f, record)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"target_id": 91, "target_version_id": 92, "target_space_id": record.SpaceID}).Error)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("target_result_id", targetID).Error)
	version := &entity.EvalTarget{ID: 91, SpaceID: record.SpaceID, EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 92, TargetID: 91, SpaceID: record.SpaceID, EvalTargetType: entity.EvalTargetTypeSandboxAgent}}
	r := &failureTargetRepo{dao: targetmysql.NewEvalTargetRecordDAO(f.p), version: version}
	sandbox := &failureSandbox{}
	target := &EvalTargetServiceImpl{evalTargetRepo: r, sandboxSchedulerAdapter: sandbox}
	f.base.(*ExptMangerImpl).evalTargetService = target
	finalizationRecreate(t, f)
	scheduler := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
	scheduler.ExptTurnResultRepo = exptinfra.NewExptTurnResultRepo(activeReferenceIDs{}, exptmysql.NewExptTurnResultDAO(f.p), nil)
	scheduler.Configer = &failureConfig{}
	scheduler.evalTargetService = target
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, TargetID: 91, TargetVersionID: 92, TargetSpaceID: record.SpaceID, Target: version}
	return f, m, scheduler, expt, r, sandbox, targetID
}

func callSchedulerFailure(ctx context.Context, s *ExptSchedulerImpl, f *finalizationManagerFixture, m entity.HookExecutionManifest, expt *entity.Experiment, kind string) error {
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit}
	items := []*entity.ExptEvalItem{{ItemID: m.Frozen.ItemID, State: entity.ItemRunState_Processing, UpdatedAt: gptr.Of(time.Now().Add(-4 * time.Hour))}}
	if kind == "zombie" {
		_, _, err := s.handleZombies(ctx, event, items, expt)
		return err
	}
	_, _, err := s.sweepTerminatedSandboxItems(ctx, event, items, expt)
	return err
}

func runSchedulerFailureBarrier(t *testing.T, kind, phase string) {
	t.Helper()
	f, m, s, expt, r, sandbox, targetID := schedulerFailureFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	entered, resume := make(chan struct{}), make(chan struct{})
	var once atomic.Bool
	pause := func(ctx context.Context) error {
		if ctx.Value(failureTickContextKey{}) == nil || !once.CompareAndSwap(false, true) {
			return nil
		}
		close(entered)
		select {
		case <-resume:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if phase == "output" {
		var versions atomic.Int64
		r.onVersion = func(ctx context.Context) error {
			if ctx.Value(failureTickContextKey{}) == nil {
				return nil
			}
			n := versions.Add(1)
			if kind == "zombie" || n == 2 {
				return pause(ctx)
			}
			return nil
		}
	} else if kind == "zombie" {
		s.Configer = &failureConfig{pause: pause}
	} else {
		sandbox.onGet = pause
	}
	done := make(chan error, 1)
	go func() {
		done <- callSchedulerFailure(context.WithValue(ctx, failureTickContextKey{}, true), s, f, m, expt, kind)
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("scheduler barrier not reached")
	}
	require.NoError(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel paused failure tick", nil))
	require.True(t, finalizationRead(t, f).State.After.Activated)
	before := hookPersistenceRead(t, f, m)
	var targetBefore targetmodel.TargetRecord
	require.NoError(t, f.sql.First(&targetBefore, targetID).Error)
	close(resume)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("failure tick did not resume")
	}
	if phase == "output" {
		var after targetmodel.TargetRecord
		require.NoError(t, f.sql.First(&after, targetID).Error)
		require.JSONEq(t, string(gptr.Indirect(targetBefore.OutputData)), string(gptr.Indirect(after.OutputData)), "stale target Save destroyed preserved output")
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(gptr.Indirect(after.OutputData), &fields))
	}
	require.Equal(t, before, hookPersistenceRead(t, f, m), "stale failure tick regressed Terminal/Resulted or error output")
}

func TestHookSchedulerFailureStaleRows(t *testing.T) {
	for _, kind := range []string{"zombie", "sweep"} {
		t.Run(kind, func(t *testing.T) { runSchedulerFailureBarrier(t, kind, "rows") })
	}
}
func TestHookSchedulerFailureStaleTargetOutput(t *testing.T) {
	for _, kind := range []string{"zombie", "sweep"} {
		t.Run(kind, func(t *testing.T) { runSchedulerFailureBarrier(t, kind, "output") })
	}
}

func TestHookSchedulerFailureConstructorRejectsUnmanagedManager(t *testing.T) {
	f, _ := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	gate := exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil })
	decision, err := gate.CanDispatch(context.Background(), f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateReady, decision.Gate, "real persisted managed Run must be Ready")
	base := &ExptSchedulerImpl{Manager: f.base, ResultSvc: &ExptResultServiceImpl{}, Publisher: hookPersistencePublisher{publish: func(context.Context) error { return nil }}}
	_, err = NewHookAwareExptSchedulerSvc(base, gate)
	require.Error(t, err, "Hook-aware construction must reject missing finalization even when the real managed gate is Ready")
}

func TestHookSchedulerFailureActivePreservesRecordsAndArchives(t *testing.T) {
	for _, kind := range []string{"zombie", "sweep"} {
		t.Run(kind, func(t *testing.T) {
			f, m, s, expt, _, sandbox, targetID := schedulerFailureFixture(t)
			require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"pending_cnt": 0, "processing_cnt": 1}).Error)
			ids := []int64{finalizationTestIDs.Add(1), finalizationTestIDs.Add(1)}
			for i, id := range ids {
				status := int32(entity.EvaluatorRunStatusSuccess)
				if i == 1 {
					status = int32(entity.EvaluatorRunStatusAsyncInvoking)
				}
				require.NoError(t, f.sql.Create(&evalmodel.EvaluatorRecord{ID: id, SpaceID: f.space, ExperimentID: gptr.Of(f.expt), ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, EvaluatorVersionID: int64(93 + i), Status: status, SourceType: 1, OutputData: gptr.Of([]byte(`{"custom":{"keep":true},"evaluator_result":{"score":0.75}}`))}).Error)
			}
			t.Cleanup(func() {
				require.NoError(t, f.sql.Unscoped().Where("id IN ?", ids).Delete(&evalmodel.EvaluatorRecord{}).Error)
				require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
			})
			raw, err := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 93, RecordID: ids[0]}, {VersionID: 94, RecordID: ids[1]}}})
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("evaluator_result_ids", raw).Error)
			result := &ExptResultServiceImpl{evaluatorRecordService: activeStoredEvaluatorReader{sql: f.sql}, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil), idgen: activeReferenceIDs{}}
			s.ResultSvc = result
			var targetBefore targetmodel.TargetRecord
			require.NoError(t, f.sql.First(&targetBefore, targetID).Error)
			var successBefore evalmodel.EvaluatorRecord
			require.NoError(t, f.sql.First(&successBefore, ids[0]).Error)
			sandbox.onDestroy = func(_ context.Context, in *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
				require.Equal(t, fmt.Sprint(f.expt), in.TaskID)
				require.Equal(t, kind == "zombie", in.ZombieTimeout)
				return &rpc.SandboxDestroyResponse{AffectedCount: 1}, nil
			}
			require.NoError(t, callSchedulerFailure(context.Background(), s, f, m, expt, kind))
			state := hookPersistenceRead(t, f, m)
			require.Equal(t, int32(entity.ItemRunState_Fail), state.Log.Status)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(state.Log.ResultState))
			require.Equal(t, int32(entity.ItemRunState_Processing), state.Item.Status, "archive owns status/counter projection")
			require.Equal(t, int32(1), state.Stats.ProcessingCnt)
			var targetAfter targetmodel.TargetRecord
			require.NoError(t, f.sql.First(&targetAfter, targetID).Error)
			var before, after map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(gptr.Indirect(targetBefore.OutputData), &before))
			require.NoError(t, json.Unmarshal(gptr.Indirect(targetAfter.OutputData), &after))
			for _, k := range []string{"OutputFields", "EvalTargetSteps", "Ext"} {
				require.JSONEq(t, string(before[k]), string(after[k]))
			}
			require.Equal(t, targetBefore.Ext, targetAfter.Ext)
			require.Equal(t, int32(entity.EvalTargetRunStatusFail), targetAfter.Status)
			var completed, failed evalmodel.EvaluatorRecord
			require.NoError(t, f.sql.First(&completed, ids[0]).Error)
			require.NoError(t, f.sql.First(&failed, ids[1]).Error)
			require.Equal(t, successBefore, completed)
			require.Equal(t, int32(entity.EvaluatorRunStatusFail), failed.Status)
			require.Contains(t, string(gptr.Indirect(failed.OutputData)), `"keep":true`)
			archive, err := result.WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
			require.NoError(t, err)
			refs, err := archive.RecordItemRunLogs(context.Background(), f.expt, f.key.RunID, m.Frozen.ItemID, f.space, expt)
			require.NoError(t, err)
			require.Len(t, refs, 2)
			archived := hookPersistenceRead(t, f, m)
			require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(archived.Log.ResultState))
			require.Equal(t, int32(1), archived.Stats.FailCnt)
			require.Zero(t, archived.Stats.ProcessingCnt)
			require.Equal(t, targetID, archived.Turn.TargetResultID)
		})
	}
}

func TestHookSchedulerFailureCleanupFailureRetriesWithoutWrites(t *testing.T) {
	f, m, s, expt, _, sandbox, targetID := schedulerFailureFixture(t)
	before := hookPersistenceRead(t, f, m)
	var targetBefore targetmodel.TargetRecord
	require.NoError(t, f.sql.First(&targetBefore, targetID).Error)
	sandbox.onDestroy = func(context.Context, *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
		return nil, errors.New("destroy unavailable")
	}
	sandbox.onGet = func(context.Context) error { return errors.New("readback unavailable") }
	require.Error(t, callSchedulerFailure(context.Background(), s, f, m, expt, "zombie"))
	require.Equal(t, before, hookPersistenceRead(t, f, m))
	var after targetmodel.TargetRecord
	require.NoError(t, f.sql.First(&after, targetID).Error)
	require.Equal(t, targetBefore, after)
	sandbox.onDestroy, sandbox.onGet = nil, nil
	require.NoError(t, callSchedulerFailure(context.Background(), s, f, m, expt, "zombie"))
	require.Equal(t, int32(entity.ItemRunState_Fail), hookPersistenceRead(t, f, m).Log.Status)
}

func TestHookSchedulerFailureTransactionRollback(t *testing.T) {
	f, m, s, expt, _, _, targetID := schedulerFailureFixture(t)
	before := hookPersistenceRead(t, f, m)
	var targetBefore targetmodel.TargetRecord
	require.NoError(t, f.sql.First(&targetBefore, targetID).Error)
	injected := errors.New("failure item update unavailable")
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("hook-failure-rollback", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result_run_log" {
			tx.AddError(injected)
		}
	}))
	t.Cleanup(func() { _ = f.sql.Callback().Update().Remove("hook-failure-rollback") })
	require.ErrorIs(t, callSchedulerFailure(context.Background(), s, f, m, expt, "zombie"), injected)
	require.Equal(t, before, hookPersistenceRead(t, f, m))
	var after targetmodel.TargetRecord
	require.NoError(t, f.sql.First(&after, targetID).Error)
	require.Equal(t, targetBefore, after, "record closure rolls back with item failure")
}

type failureArchiveOnly struct{ repo.IHookItemArchiveRepo }
type failureDispatchOnly struct {
	repo.IHookItemArchiveRepo
	dispatch repo.IHookSchedulerRepo
}

func (r failureDispatchOnly) PersistHookDispatch(ctx context.Context, in entity.HookSchedulerDispatchInput) ([]int64, error) {
	return r.dispatch.PersistHookDispatch(ctx, in)
}

func TestHookSchedulerFailureConstructorCapabilities(t *testing.T) {
	f, _ := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	archive := f.deps.Repository.(repo.IHookItemArchiveRepo)
	for _, storage := range []repo.IHookFinalizationRepo{failureArchiveOnly{archive}, failureDispatchOnly{archive, f.deps.Repository.(repo.IHookSchedulerRepo)}} {
		manager := *f.manager
		deps := *manager.finalization
		deps.Repository = storage
		manager.finalization = &deps
		base := &ExptSchedulerImpl{Manager: &manager, ResultSvc: &ExptResultServiceImpl{}, Publisher: hookPersistencePublisher{publish: func(context.Context) error { return nil }}}
		_, err := NewHookAwareExptSchedulerSvc(base, exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }))
		require.Error(t, err, "missing dispatch or failure capability must fail construction")
	}
}

func TestHookSchedulerFailureNilCapabilityNeverFallsBack(t *testing.T) {
	f, m, s, expt, _, _, _ := schedulerFailureFixture(t)
	s.hookScheduler = nil
	before := hookPersistenceRead(t, f, m)
	for _, kind := range []string{"zombie", "sweep"} {
		require.Error(t, callSchedulerFailure(context.Background(), s, f, m, expt, kind))
	}
	require.Error(t, hookPersistenceDispatch(context.Background(), s, f, []entity.HookExecutionManifest{m}))
	require.Equal(t, before, hookPersistenceRead(t, f, m))
}

func TestHookSchedulerFailureNewLatestDuringCleanup(t *testing.T) {
	for _, kind := range []string{"zombie", "sweep"} {
		t.Run(kind, func(t *testing.T) {
			f, m, s, expt, r, _, targetID := schedulerFailureFixture(t)
			before := hookPersistenceRead(t, f, m)
			var targetBefore targetmodel.TargetRecord
			require.NoError(t, f.sql.First(&targetBefore, targetID).Error)
			var once atomic.Bool
			r.onVersion = func(ctx context.Context) error {
				if !once.CompareAndSwap(false, true) {
					return nil
				}
				next := finalizationTestIDs.Add(1)
				_, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}, ExpectedLatestRunID: f.key.RunID,
					RunLog:   &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"},
					Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"},
					After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
				return err
			}
			require.NoError(t, callSchedulerFailure(context.Background(), s, f, m, expt, kind))
			require.Equal(t, before, hookPersistenceRead(t, f, m))
			var after targetmodel.TargetRecord
			require.NoError(t, f.sql.First(&after, targetID).Error)
			require.Equal(t, targetBefore, after)
		})
	}
}

func TestHookSchedulerFailureZombieCleanupExactExtraMapping(t *testing.T) {
	svc, r, s, record := hookCleanupFixture(t)
	record.EvalTargetOutputData.Ext = map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: `["51-orch","51-macvm"]`, entity.SandboxAgentExtKeyExtraExecuteID: "extra"}
	r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil)
	for _, want := range []rpc.SandboxDestroyRequest{
		{TaskID: "2", WorkspaceID: 9, DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-orch"}, ZombieTimeout: true},
		{TaskID: "2-macvm", WorkspaceID: 9, DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-macvm"}, ZombieTimeout: true},
		{TaskID: "2", WorkspaceID: 9, DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51"}, ZombieTimeout: true},
		{TaskID: "2", WorkspaceID: 9, DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"extra"}},
	} {
		s.EXPECT().Destroy(gomock.Any(), &want).Return(&rpc.SandboxDestroyResponse{AffectedCount: 1}, nil)
	}
	require.NoError(t, svc.CleanupHookSchedulerTargets(context.Background(), entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, []*entity.EvalTargetRecord{record}, true))
}
