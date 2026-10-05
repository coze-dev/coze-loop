// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

func lazyTurnRepos(f *finalizationManagerFixture) (repo.IExptItemResultRepo, repo.IExptTurnResultRepo) {
	return exptinfra.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(f.p)),
		exptinfra.NewExptTurnResultRepo(activeReferenceIDs{}, exptmysql.NewExptTurnResultDAO(f.p), exptmysql.NewExptTurnEvaluatorResultRefDAO(f.p))
}

// The real dispatch writes Processing before the consumer can create any turnlog.
func TestHookLazyTurnsDispatchedUnadmittedCancellationMySQL(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := manifests[0]
	items, turns := lazyTurnRepos(f)
	ctrl := gomock.NewController(t)
	config := cm.NewMockIConfiger(ctrl)
	pub := em.NewMockExptEventPublisher(ctrl)
	metrics := mm.NewMockExptMetric(ctrl)
	result := sm.NewMockExptResultService(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{IntervalSecond: 1}})
	pub.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	metrics.EXPECT().EmitItemExecEval(f.space, int64(entity.EvaluationModeSubmit), 1)
	result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), f.space, f.expt, []int64{m.Frozen.ItemID}).Return(nil)
	scheduler := &ExptSchedulerImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns, ExptStatsRepo: exptinfra.NewExptStatsRepo(exptmysql.NewExptStatsDAO(f.p)), Configer: config, Publisher: pub, Metric: metrics, ResultSvc: result}
	ctx := context.Background()
	require.NoError(t, scheduler.handleToSubmits(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit}, []*entity.ExptEvalItem{{ItemID: m.Frozen.ItemID, State: entity.ItemRunState_Queueing}}))
	var row model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&row, m.ItemRunLogID).Error)
	require.Equal(t, int32(entity.ItemRunState_Processing), row.Status)
	var ledger model.ExptLifecycleRunItem
	require.NoError(t, f.sql.First(&ledger, m.Frozen.ID).Error)
	require.Nil(t, ledger.AdmittedAt)
	assertLazyTurnCount(t, f, 0)
	require.NoError(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil))
	assertLazyTurnFinalized(t, f, m)
}

func admitLazyTurn(t *testing.T, f *finalizationManagerFixture, m entity.HookExecutionManifest) (context.Context, *entity.ExptItemEvalCtx, *ExptRecordEvalModeSubmit) {
	t.Helper()
	ctx := context.Background()
	before, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	admitted, err := f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, ItemID: m.Frozen.ItemID})
	require.NoError(t, err)
	require.True(t, admitted.NewlyAdmitted)
	var ledger model.ExptLifecycleRunItem
	require.NoError(t, f.sql.First(&ledger, m.Frozen.ID).Error)
	require.NotNil(t, ledger.AdmittedAt)
	items, turns := lazyTurnRepos(f)
	item, err := items.GetItemRunLog(ctx, f.expt, f.key.RunID, m.Frozen.ItemID, f.space)
	require.NoError(t, err)
	progress := exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil })
	ctx = context.WithValue(ctx, itemHookProgressContextKey{}, itemHookProgressBinding{key: f.key, itemID: m.Frozen.ItemID, itemVersion: m.Frozen.ItemVersionID, repo: progress, targets: new(sync.Map)})
	eiec := &entity.ExptItemEvalCtx{
		Event:               &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, EvalSetItemID: m.Frozen.ItemID, ExptRunMode: entity.EvaluationModeSubmit},
		Expt:                &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: f.key.RunID, ExptType: entity.ExptType_Offline},
		EvalSetItem:         &entity.EvaluationSetItem{ItemID: m.Frozen.ItemID, ItemVersionID: gptr.Of(m.Frozen.ItemVersionID)},
		ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: item, TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}},
	}
	for _, turn := range m.Turns {
		eiec.EvalSetItem.Turns = append(eiec.EvalSetItem.Turns, &entity.Turn{ID: turn.TurnID})
	}
	return ctx, eiec, &ExptRecordEvalModeSubmit{exptItemResultRepo: items, exptTurnResultRepo: turns, idgen: activeReferenceIDs{}}
}

func TestHookLazyTurnsAdmittedFailureCancellationMySQL(t *testing.T) {
	for _, stage := range []string{"reservation", "preeval"} {
		t.Run(stage, func(t *testing.T) {
			f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
			m := manifests[0]
			ctx, eiec, pre := admitLazyTurn(t, f, m)
			injected := errors.New("lazy turn injected failure")
			if stage == "reservation" {
				ctx = ctxcache.Init(ctx)
				eiec.Event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: "local"})
				consumer := &ExptItemEventEvalServiceImpl{centralGuard: &itemHookGuard{ports: &itemHookPorts{}, err: injected}}
				require.ErrorIs(t, consumer.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
					t.Fatal("PreEval must not run on reservation error")
					return nil
				})(ctx, eiec.Event), injected)
			} else {
				require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register("lazy-turn-create-failure", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_turn_result_run_log" {
						tx.AddError(injected)
					}
				}))
				err := pre.PreEval(ctx, eiec)
				require.NoError(t, f.sql.Callback().Create().Remove("lazy-turn-create-failure"))
				require.Error(t, err)
			}
			assertLazyTurnCount(t, f, 0)
			require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
			assertLazyTurnFinalized(t, f, m)
		})
	}
}

type lazyTurnBarrierIDs struct {
	idgen.IIDGenerator
	entered chan struct{}
	release chan struct{}
}

func (g lazyTurnBarrierIDs) GenMultiIDs(ctx context.Context, n int) ([]int64, error) {
	close(g.entered)
	select {
	case <-g.release:
		return (activeReferenceIDs{}).GenMultiIDs(ctx, n)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestHookLazyTurnsDelayedPreEvalAfterCancellationMySQL(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := manifests[0]
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	entered, release := make(chan struct{}), make(chan struct{})
	pre.idgen = lazyTurnBarrierIDs{entered: entered, release: release}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- pre.PreEval(ctx, eiec) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("PreEval did not reach the pre-create barrier")
	}
	err := f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil)
	close(release)
	preErr := <-done
	assert.NoError(t, err)
	assert.Error(t, preErr, "late PreEval must not create execution logs after cancellation")
	assertLazyTurnCount(t, f, 0)
	if err == nil {
		assertLazyTurnFinalized(t, f, m)
	}
}

func assertLazyTurnCount(t *testing.T, f *finalizationManagerFixture, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, f.sql.Unscoped().Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.key.RunID).Count(&count).Error)
	assert.Equal(t, want, count, "lazy absence must not fabricate execution logs")
}

func assertLazyTurnFinalized(t *testing.T, f *finalizationManagerFixture, m entity.HookExecutionManifest) {
	t.Helper()
	state := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, state.State.Finalize)
	require.True(t, state.State.After.Activated)
	assertLazyTurnCount(t, f, 0)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
	require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
	for _, frozen := range m.Turns {
		var turn model.ExptTurnResult
		require.NoError(t, f.sql.First(&turn, frozen.ResultID).Error)
		require.Equal(t, int32(entity.TurnRunState_Terminal), turn.Status)
		require.Equal(t, frozen.TurnID, turn.TurnID)
		require.Zero(t, turn.TargetResultID)
	}
	var run model.ExptRunLog
	require.NoError(t, f.sql.First(&run, f.key.RunID).Error)
	require.Equal(t, int32(len(m.Turns)), run.TerminatedCnt)
	require.Zero(t, run.SuccessCnt+run.FailCnt+run.PendingCnt+run.ProcessingCnt)
	require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
	require.Equal(t, state, finalizationRead(t, f))
}

func lazyTurnManifest(t *testing.T, f *finalizationManagerFixture, m entity.HookExecutionManifest, n int) entity.HookExecutionManifest {
	t.Helper()
	for i := 1; i < n; i++ {
		turn := entity.HookExecutionTurnManifest{TurnID: int64(i + 10), TurnIdx: int32(i), ResultID: finalizationTestIDs.Add(1)}
		m.Turns = append(m.Turns, turn)
		require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: turn.ResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: turn.TurnID, TurnIdx: gptr.Of(turn.TurnIdx), Status: int32(entity.TurnRunState_Queueing)}).Error)
	}
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", raw).Error)
	return m
}

func TestHookLazyTurnsRejectCorruptStoredRowsMySQL(t *testing.T) {
	for _, kind := range []string{"success_missing", "fail_missing", "deleted", "foreign_space", "foreign_experiment", "foreign_version", "extra_turn", "partial"} {
		t.Run(kind, func(t *testing.T) {
			f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
			m := manifests[0]
			if kind == "partial" {
				m = lazyTurnManifest(t, f, m, 2)
			}
			ctx, eiec, pre := admitLazyTurn(t, f, m)
			if kind == "success_missing" || kind == "fail_missing" {
				status := entity.ItemRunState_Success
				if kind == "fail_missing" {
					status = entity.ItemRunState_Fail
				}
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("status", int32(status)).Error)
			} else {
				require.NoError(t, pre.PreEval(ctx, eiec))
				var row model.ExptTurnResultRunLog
				require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.key.RunID, m.Frozen.ItemID).Order("turn_id").First(&row).Error)
				switch kind {
				case "deleted":
					row.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
				case "foreign_space":
					row.SpaceID++
				case "foreign_experiment":
					row.ExptID++
				case "foreign_version":
					row.ItemVersionID++
				case "extra_turn":
					row.ID = finalizationTestIDs.Add(1)
					row.TurnID = 9999
				}
				if kind == "extra_turn" {
					require.NoError(t, f.sql.Create(&row).Error)
				} else if kind == "partial" {
					require.NoError(t, f.sql.Unscoped().Delete(&row).Error)
				} else {
					require.NoError(t, f.sql.Unscoped().Save(&row).Error)
				}
				t.Cleanup(func() {
					require.NoError(t, f.sql.Unscoped().Delete(&model.ExptTurnResultRunLog{}, "id=?", row.ID).Error)
				})
			}
			require.Error(t, pre.PreEval(ctx, eiec), "corrupt rows cannot be repaired by lazy creation")
			require.ErrorIs(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil), entity.ErrHookStoreCorrupt)
			require.False(t, finalizationRead(t, f).State.After.Activated)
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
			require.NotEqual(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
		})
	}
}

type lazyTurnMissingInitializer struct{ repo.IHookTurnProgressRepo }

func TestHookLazyTurnsMissingCapabilityFailsClosedMySQL(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	ctx, eiec, pre := admitLazyTurn(t, f, manifests[0])
	binding := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	binding.repo = lazyTurnMissingInitializer{binding.repo}
	ctx = context.WithValue(ctx, itemHookProgressContextKey{}, binding)
	require.True(t, itemHookControlOnly(pre.PreEval(ctx, eiec)))
	assertLazyTurnCount(t, f, 0)
}

func TestHookLazyTurnsGenuinePreEvalAndReplayMySQL(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := lazyTurnManifest(t, f, manifests[0], 2)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	require.NoError(t, pre.PreEval(ctx, eiec))
	assertLazyTurnCount(t, f, 2)
	var before []model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Order("turn_id").Find(&before).Error)
	require.Len(t, before, 2)
	for i, row := range before {
		require.Equal(t, m.Turns[i].TurnID, row.TurnID)
		require.Equal(t, int32(entity.TurnRunState_Processing), row.Status)
		require.Equal(t, row.ID, eiec.ExistItemEvalResult.TurnResultRunLogs[row.TurnID].ID)
	}
	require.NoError(t, pre.PreEval(ctx, eiec))
	var after []model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Order("turn_id").Find(&after).Error)
	require.Equal(t, before, after)
	require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
	assertLazyTurnCount(t, f, 2)
	require.True(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookLazyTurnsCreateTransactionPrecedesCancelMySQL(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := lazyTurnManifest(t, f, manifests[0], 2)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register("lazy-turn-create-barrier", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_turn_result_run_log" {
			close(entered)
			select {
			case <-release:
			case <-tx.Statement.Context.Done():
				tx.AddError(tx.Statement.Context.Err())
			}
		}
	}))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	created, terminated := make(chan error, 1), make(chan error, 1)
	go func() { created <- pre.PreEval(ctx, eiec) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("initializer did not reach insert")
	}
	go func() { terminated <- f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil) }()
	// Observe the actual MySQL lock wait, not a timing assumption about goroutine scheduling.
	blocked := false
	require.Eventually(t, func() bool {
		var n int64
		err := f.sql.Raw("SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON w.BLOCKING_ENGINE_LOCK_ID=l.ENGINE_LOCK_ID WHERE l.OBJECT_SCHEMA=DATABASE() AND l.OBJECT_NAME='experiment' AND l.LOCK_DATA=?", fmt.Sprint(f.expt)).Scan(&n).Error
		blocked = err == nil && n > 0
		return blocked
	}, 3*time.Second, 10*time.Millisecond)
	unblock()
	require.NoError(t, <-created)
	require.NoError(t, <-terminated)
	require.NoError(t, f.sql.Callback().Create().Remove("lazy-turn-create-barrier"))
	assertLazyTurnCount(t, f, 2)
	var rows []model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Find(&rows).Error)
	for _, row := range rows {
		require.Equal(t, int32(entity.TurnRunState_Terminal), row.Status)
	}
	require.True(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookLazyTurnsBatchFailureRollsBackBeforeCancelMySQL(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := lazyTurnManifest(t, f, manifests[0], 51)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	batches := 0
	require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register("lazy-turn-second-batch-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_turn_result_run_log" {
			batches++
			if batches == 2 {
				tx.AddError(errors.New("second batch failed"))
			}
		}
	}))
	err := pre.PreEval(ctx, eiec)
	require.NoError(t, f.sql.Callback().Create().Remove("lazy-turn-second-batch-failure"))
	require.Error(t, err)
	require.Equal(t, 2, batches, "the first batch must actually have reached the database")
	assertLazyTurnCount(t, f, 0)
	require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
	assertLazyTurnFinalized(t, f, m)
}

func TestHookLazyTurnsRejectUnfrozenCandidatesMySQL(t *testing.T) {
	for _, kind := range []string{"missing", "extra", "duplicate", "foreign"} {
		t.Run(kind, func(t *testing.T) {
			f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
			m := lazyTurnManifest(t, f, manifests[0], 2)
			ctx, eiec, pre := admitLazyTurn(t, f, m)
			switch kind {
			case "missing":
				eiec.EvalSetItem.Turns = eiec.EvalSetItem.Turns[:1]
			case "extra":
				eiec.EvalSetItem.Turns = append(eiec.EvalSetItem.Turns, &entity.Turn{ID: 999})
			case "duplicate":
				eiec.EvalSetItem.Turns[1] = &entity.Turn{ID: 0}
			case "foreign":
				eiec.EvalSetItem.Turns[1] = &entity.Turn{ID: 999}
			}
			require.Error(t, pre.PreEval(ctx, eiec))
			assertLazyTurnCount(t, f, 0)
		})
	}
}

func TestHookLazyTurnsNewLatestRejectsOldPreEvalMySQL(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	ctx, eiec, pre := admitLazyTurn(t, f, manifests[0])
	newRun := finalizationTestIDs.Add(1)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("latest_run_id", newRun).Error)
	require.Error(t, pre.PreEval(ctx, eiec))
	assertLazyTurnCount(t, f, 0)
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	require.Equal(t, newRun, expt.LatestRunID)
}

func TestHookLazyTurnsUnsupportedAfterOnlyAppendUsesLegacyPathMySQL(t *testing.T) {
	f := newFinalizationManagerFixture(t, "tx")
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("mode", int32(entity.EvaluationModeAppend)).Error)
	var life model.ExptLifecycleRun
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
	require.False(t, life.BeforeEnabled)
	require.True(t, life.AfterEnabled)
	initializer := exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }).(repo.IHookTurnLogInitializer)
	handled, rows, err := initializer.InitializeHookTurnRunLogs(context.Background(), f.key, 1, 0, nil)
	require.NoError(t, err)
	require.False(t, handled)
	require.Empty(t, rows)
	var after model.ExptLifecycleRun
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&after).Error)
	require.Equal(t, life, after)
}
