// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/redis/go-redis/v9"
)

func manualRetryScaleItems(t *testing.T) int {
	t.Helper()
	n := 900
	if raw := os.Getenv("EXPT_RETRY_SCALE_ITEMS"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		require.NoError(t, err)
		n = parsed
	}
	require.Positive(t, n)
	return n
}

// prepareManualRetryScaleFixture rewrites the standard real-MySQL fixture into
// the frozen snapshot shape FailRetry validates: n failed items, two turns per
// item, one target record and one evaluator record/ref per turn.
func prepareManualRetryScaleFixture(t *testing.T, n int) *manualRetryFixture {
	t.Helper()
	f := newManualRetryFixture(t)
	conn := f.conn
	for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_stats"} {
		require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id = ? AND expt_id = ?", f.space, f.expt).Error)
	}
	require.NoError(t, conn.Exec("DELETE FROM eval_target_record WHERE space_id = ?", f.space).Error)
	require.NoError(t, conn.Exec("DELETE FROM evaluator_record WHERE space_id = ?", f.space).Error)
	t.Cleanup(func() {
		require.NoError(t, conn.Exec("DELETE FROM eval_target_record WHERE space_id = ?", f.space).Error)
		require.NoError(t, conn.Exec("DELETE FROM evaluator_record WHERE space_id = ?", f.space).Error)
	})

	const targetVersion = int64(42)
	require.NoError(t, conn.Model(&model.Experiment{}).Where("id = ?", f.expt).Updates(map[string]any{
		"latest_run_id":        f.run,
		"status":               int32(entity.ExptStatus_Processing),
		"expt_type":            int32(entity.ExptType_Offline),
		"eval_set_source_type": int32(entity.ExptEvalSetSourceType_SingleSet),
		"eval_set_id":          f.base + 31,
		"eval_set_version_id":  f.base + 32,
		"target_id":            f.base + 41,
		"target_version_id":    targetVersion,
		"target_type":          1,
		"scheduler_mode":       entity.ExptDispatchModeLegacy,
	}).Error)

	raw, err := entity.EncodeExptRunLogItems([]entity.ExptRunLogItems{{ItemIDs: []int64{1}, CreateAt: gptr.Of(time.Now().Unix())}})
	require.NoError(t, err)
	require.NoError(t, conn.Model(&model.ExptRunLog{}).Where("id = ?", f.run).Updates(map[string]any{
		"status":         int64(entity.ExptStatus_Processing),
		"item_ids":       raw,
		"pending_cnt":    int32(0),
		"processing_cnt": int32(0),
		"success_cnt":    int32(0),
		"fail_cnt":       int32(n),
		"terminated_cnt": int32(0),
	}).Error)
	require.NoError(t, conn.Create(&model.ExptStats{ID: f.base + 4, SpaceID: f.space, ExptID: f.expt, FailCnt: int32(n)}).Error)

	itemRows := make([]*model.ExptItemResult, 0, n)
	itemLogs := make([]*model.ExptItemResultRunLog, 0, n)
	turnRows := make([]*model.ExptTurnResult, 0, n*2)
	turnLogs := make([]*model.ExptTurnResultRunLog, 0, n*2)
	refRows := make([]*model.ExptTurnEvaluatorResultRef, 0, n*2)
	targetRows := make([]map[string]any, 0, n*2)
	evalRows := make([]map[string]any, 0, n*2)
	for i := 1; i <= n; i++ {
		item := int64(i)
		version := f.base + 1000 + item
		itemRows = append(itemRows, &model.ExptItemResult{ID: f.base + 100000 + item, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: version, Status: int32(entity.ItemRunState_Fail), Ext: gptr.Of([]byte(`{}`))})
		itemLogs = append(itemLogs, &model.ExptItemResultRunLog{ID: f.base + 200000 + item, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: version, Status: int32(entity.ItemRunState_Fail), ResultState: gptr.Of(int32(entity.ExptItemResultStateResulted))})
		for turn := 0; turn < 2; turn++ {
			turnID := int64(100000 + i*2 + turn)
			status := int32(entity.TurnRunState_Fail)
			if turn == 1 {
				status = int32(entity.TurnRunState_Success)
			}
			targetID := f.base + 600000 + int64(i*2+turn)
			evalID := f.base + 700000 + int64(i*2+turn)
			turnResultID := f.base + 300000 + int64(i*2+turn)
			turnRows = append(turnRows, &model.ExptTurnResult{ID: turnResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: version, TurnID: turnID, Status: status, TargetResultID: targetID})
			evalResults, err := json.Marshal(&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: evalID}})
			require.NoError(t, err)
			turnLogs = append(turnLogs, &model.ExptTurnResultRunLog{ID: f.base + 400000 + int64(i*2+turn), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: version, TurnID: turnID, Status: status, TargetResultID: targetID, EvaluatorResultIds: &evalResults})
			refRows = append(refRows, &model.ExptTurnEvaluatorResultRef{ID: f.base + 500000 + int64(i*2+turn), SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: turnResultID, EvaluatorVersionID: 51, EvaluatorResultID: evalID})
			targetRows = append(targetRows, map[string]any{"id": targetID, "space_id": f.space, "target_id": f.base + 41, "target_version_id": targetVersion, "experiment_run_id": f.run, "item_id": item, "item_version_id": version, "turn_id": turnID, "log_id": "", "trace_id": "", "status": 1})
			evalRows = append(evalRows, map[string]any{"id": evalID, "space_id": f.space, "evaluator_version_id": 51, "experiment_id": f.expt, "experiment_run_id": f.run, "item_id": item, "item_version_id": version, "turn_id": turnID, "trace_id": "", "status": 1, "source_type": 1, "inline_key": "", "alias": "", "target_record_id": 0, "created_by": "", "updated_by": ""})
		}
	}
	require.NoError(t, conn.CreateInBatches(itemRows, 500).Error)
	require.NoError(t, conn.CreateInBatches(itemLogs, 500).Error)
	require.NoError(t, conn.CreateInBatches(turnRows, 500).Error)
	require.NoError(t, conn.CreateInBatches(turnLogs, 500).Error)
	require.NoError(t, conn.CreateInBatches(refRows, 500).Error)
	require.NoError(t, conn.Table("eval_target_record").CreateInBatches(targetRows, 500).Error)
	require.NoError(t, conn.Table("evaluator_record").CreateInBatches(evalRows, 500).Error)
	return f
}

type scaleItemStorage struct {
	*singleSetItemStorage
}

func (s *scaleItemStorage) MGetItemRunLog(ctx context.Context, expt, run int64, ids []int64, space int64) ([]*entity.ExptItemResultRunLog, error) {
	rows, err := mysql.NewExptItemResultDAO(s.f.provider).MGetItemRunLog(ctx, expt, run, ids, space)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.ExptItemResultRunLog, 0, len(rows))
	for _, row := range rows {
		result = append(result, convert.NewExptItemResultRunLogConverter().PO2DO(row))
	}
	return result, nil
}

func (s *scaleItemStorage) MGetItemTurnResults(ctx context.Context, space, expt int64, ids []int64) ([]*entity.ExptTurnResult, error) {
	rows, err := mysql.NewExptItemResultDAO(s.f.provider).MGetItemTurnResults(ctx, space, expt, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.ExptTurnResult, 0, len(rows))
	for _, row := range rows {
		result = append(result, convert.NewExptTurnResultConvertor().PO2DO(row, nil))
	}
	return result, nil
}

func (s *scaleItemStorage) ScanItemResults(ctx context.Context, expt, cursor, limit int64, statuses []int32, space int64) ([]*entity.ExptItemResult, int64, error) {
	return exptrepo.NewExptItemResultRepo(mysql.NewExptItemResultDAO(s.f.provider)).ScanItemResults(ctx, expt, cursor, limit, statuses, space)
}

type scaleTurnStorage struct {
	*singleSetTurnStorage
}

func (s *scaleTurnStorage) MGetItemTurnRunLogs(ctx context.Context, expt, run int64, ids []int64, space int64) ([]*entity.ExptTurnResultRunLog, error) {
	rows, err := mysql.NewExptTurnResultDAO(s.f.provider).MGetItemTurnRunLogs(ctx, expt, run, ids, space)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.ExptTurnResultRunLog, 0, len(rows))
	for _, row := range rows {
		log, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(row)
		if err != nil {
			return nil, err
		}
		result = append(result, log)
	}
	return result, nil
}

type scaleRefStorage struct {
	*singleSetRefStorage
}

func (s *scaleRefStorage) MGetByExptIDAndItemIDs(ctx context.Context, space, expt int64, ids []int64) ([]*entity.ExptItemRef, error) {
	rows, err := mysql.NewExptItemRefDAO(s.f.provider).MGetByExptIDAndItemIDs(ctx, space, expt, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.ExptItemRef, 0, len(rows))
	for _, row := range rows {
		result = append(result, convert.NewExptItemRefConvertor().PO2DO(row))
	}
	return result, nil
}

type scaleTimedPort struct {
	repo.IExptManualRetryRepo
	mu           sync.Mutex
	preflight    time.Duration
	requeue      time.Duration
	preflightN   int
	requeueN     int
	preflightCnt int
	requeueCnt   int
}

func (p *scaleTimedPort) PreflightTerminalItems(ctx context.Context, req *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error) {
	start := time.Now()
	result, err := p.IExptManualRetryRepo.PreflightTerminalItems(ctx, req)
	p.mu.Lock()
	p.preflight += time.Since(start)
	p.preflightN += len(req.Items)
	p.preflightCnt++
	p.mu.Unlock()
	return result, err
}

func (p *scaleTimedPort) RequeueTerminalItems(ctx context.Context, req *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error) {
	start := time.Now()
	result, err := p.IExptManualRetryRepo.RequeueTerminalItems(ctx, req)
	p.mu.Lock()
	p.requeue += time.Since(start)
	p.requeueN += len(req.Items)
	p.requeueCnt++
	p.mu.Unlock()
	return result, err
}

type scaleLockSpan struct {
	key   string
	start time.Time
	end   time.Time
}

type scaleTimedLocker struct {
	lock.ILocker
	mu    sync.Mutex
	spans []*scaleLockSpan
}

func (l *scaleTimedLocker) LockWithRenew(ctx context.Context, key string, ttl, maxHold time.Duration) (bool, context.Context, func(), error) {
	start := time.Now()
	ok, child, cancel, err := l.ILocker.LockWithRenew(ctx, key, ttl, maxHold)
	if !ok || err != nil {
		return ok, child, cancel, err
	}
	span := &scaleLockSpan{key: key, start: start}
	l.mu.Lock()
	l.spans = append(l.spans, span)
	l.mu.Unlock()
	return ok, child, func() {
		l.mu.Lock()
		span.end = time.Now()
		l.mu.Unlock()
		cancel()
	}, nil
}

func (l *scaleTimedLocker) summary() (keys int, acquire, maxRunHold time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, span := range l.spans {
		end := span.end
		if end.IsZero() {
			end = now
		}
		keys++
		acquire += end.Sub(span.start)
		if len(span.key) >= len("expt_run_exec_lock:") && span.key[:len("expt_run_exec_lock:")] == "expt_run_exec_lock:" {
			if hold := end.Sub(span.start); hold > maxRunHold {
				maxRunHold = hold
			}
		}
	}
	return keys, acquire, maxRunHold
}

type scaleWaker struct{}

func (scaleWaker) WakeCurrentRun(context.Context, string) error { return nil }

func newScaleManager(t *testing.T, f *manualRetryFixture, port repo.IExptManualRetryRepo, locker lock.ILocker) service.IExptManager {
	t.Helper()
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	s := &singleSetRetryStorage{f: f}
	return service.NewExptManager(
		nil,
		&singleSetExperimentStorage{singleSetRetryStorage: s},
		&singleSetRunStorage{singleSetRetryStorage: s},
		nil,
		&scaleItemStorage{singleSetItemStorage: &singleSetItemStorage{singleSetRetryStorage: s}},
		&scaleRefStorage{singleSetRefStorage: &singleSetRefStorage{singleSetRetryStorage: s}},
		&scaleTurnStorage{singleSetTurnStorage: &singleSetTurnStorage{singleSetRetryStorage: s}},
		config,
		nil,
		locker,
		nil, nil, nil, nil, nil, nil, nil, nil,
		&singleSetTargetStorage{singleSetRetryStorage: s},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		port,
		&singleSetEvaluatorStorage{singleSetRetryStorage: s},
		scaleWaker{},
		func() lock.ILocker { return locker },
	)
}

type scaleSampler struct {
	stop       chan struct{}
	done       chan struct{}
	peakG      int
	peakHeapMB uint64
	peakSysMB  uint64
}

func startScaleSampler() *scaleSampler {
	s := &scaleSampler{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		var stats runtime.MemStats
		for {
			if g := runtime.NumGoroutine(); g > s.peakG {
				s.peakG = g
			}
			runtime.ReadMemStats(&stats)
			if heap := stats.HeapAlloc / 1024 / 1024; heap > s.peakHeapMB {
				s.peakHeapMB = heap
			}
			if sys := stats.Sys / 1024 / 1024; sys > s.peakSysMB {
				s.peakSysMB = sys
			}
			select {
			case <-s.stop:
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()
	return s
}

func (s *scaleSampler) finish() {
	close(s.stop)
	<-s.done
}

func mysqlQuestions(t *testing.T, conn *gorm.DB) int64 {
	t.Helper()
	var row struct{ Value int64 }
	require.NoError(t, conn.Raw("SHOW GLOBAL STATUS LIKE 'Questions'").Scan(&row).Error)
	return row.Value
}

func TestManualRetryAtomicScaleMySQL(t *testing.T) {
	n := manualRetryScaleItems(t)
	f := prepareManualRetryScaleFixture(t, n)
	port := &scaleTimedPort{IExptManualRetryRepo: f.port}
	ctrl := gomock.NewController(t)
	mock := lockmocks.NewMockILocker(ctrl)
	mock.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), 5*time.Second, 5*time.Minute).DoAndReturn(
		func(parent context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
			child, cancel := context.WithCancel(parent)
			return true, child, cancel, nil
		}).AnyTimes()
	locker := &scaleTimedLocker{ILocker: mock}
	manager := newScaleManager(t, f, port, locker)

	beforeQ := mysqlQuestions(t, f.conn)
	sampler := startScaleSampler()
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	receipt, err := manager.RetryInCurrentRun(ctx, &entity.RetryInCurrentRunRequest{
		SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, Mode: entity.EvaluationModeFailRetry,
	})
	elapsed := time.Since(start)
	sampler.finish()
	afterQ := mysqlQuestions(t, f.conn)
	require.NoError(t, err, "FailRetry must finish inside the 5m synchronous lease/deadline")
	require.NotNil(t, receipt)
	require.Equal(t, int32(n), receipt.AcceptedCount)
	require.Empty(t, receipt.RejectedItems)
	require.Less(t, elapsed, 5*time.Minute)
	keys, _, maxRunHold := locker.summary()
	require.Less(t, maxRunHold, 5*time.Minute)
	t.Logf("REAL_MYSQL_ATOMIC_SCALE items=%d turns=2 refs=1 elapsed=%s preflight=%s(n=%d,calls=%d) requeue=%s(n=%d,calls=%d) sql_questions=%d lock_keys=%d max_run_lock_hold=%s peak_goroutines=%d peak_heap=%dMB peak_sys=%dMB",
		n, elapsed, port.preflight, port.preflightN, port.preflightCnt, port.requeue, port.requeueN, port.requeueCnt,
		afterQ-beforeQ, keys, maxRunHold, sampler.peakG, sampler.peakHeapMB, sampler.peakSysMB)

	replay, err := manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{
		SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, Mode: entity.EvaluationModeFailRetry,
	})
	require.NoError(t, err)
	require.Zero(t, replay.AcceptedCount, "second FailRetry must not re-accept queued items")
}

func TestManualRetryAtomicScaleRealRedis(t *testing.T) {
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	if addr == "" {
		t.Skip("EXPT_RETRY_REAL_REDIS_ADDR not configured")
	}
	n := manualRetryScaleItems(t)
	f := prepareManualRetryScaleFixture(t, n)
	port := &scaleTimedPort{IExptManualRetryRepo: f.port}
	client, err := infraredis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	t.Cleanup(func() { _ = raw.Close() })
	base := lock.NewRedisLockerWithHolder(client, fmt.Sprintf("atomic-scale-%d", time.Now().UnixNano()))
	locker := &scaleTimedLocker{ILocker: base}
	manager := newScaleManager(t, f, port, locker)

	beforeQ := mysqlQuestions(t, f.conn)
	sampler := startScaleSampler()
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	receipt, err := manager.RetryInCurrentRun(ctx, &entity.RetryInCurrentRunRequest{
		SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, Mode: entity.EvaluationModeFailRetry,
	})
	elapsed := time.Since(start)
	sampler.finish()
	afterQ := mysqlQuestions(t, f.conn)
	require.NoError(t, err, "FailRetry with real Redis item locks must finish inside the 5m lease/deadline")
	require.NotNil(t, receipt)
	require.Equal(t, int32(n), receipt.AcceptedCount)
	require.Empty(t, receipt.RejectedItems)
	require.Less(t, elapsed, 5*time.Minute)
	keys, _, maxRunHold := locker.summary()
	require.Less(t, maxRunHold, 5*time.Minute)
	t.Logf("REAL_REDIS_ATOMIC_SCALE items=%d turns=2 refs=1 elapsed=%s preflight=%s requeue=%s sql_questions=%d lock_keys=%d max_run_lock_hold=%s peak_goroutines=%d peak_heap=%dMB peak_sys=%dMB",
		n, elapsed, port.preflight, port.requeue, afterQ-beforeQ, keys, maxRunHold, sampler.peakG, sampler.peakHeapMB, sampler.peakSysMB)

	// LockWithRenew releases asynchronously; let the renew goroutines finish
	// before asserting the duplicate-request path.
	runKey := fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		if exists, err := raw.Exists(context.Background(), runKey).Result(); err == nil && exists == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	replay, err := manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{
		SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, Mode: entity.EvaluationModeFailRetry,
	})
	require.NoError(t, err)
	require.Zero(t, replay.AcceptedCount, "second FailRetry must not re-accept queued items")
}
