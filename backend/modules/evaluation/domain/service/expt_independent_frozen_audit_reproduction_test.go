// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	redis "github.com/redis/go-redis/v9"
	"github.com/rs/xid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	quotadao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// No existing regression expectation is reused. SQL seeds represent evaluated
// results (F1) or three in-flight sources (F2); lifecycle commits, lock/CID IO,
// quota updates and terminal replay are all production calls. The guard below
// models per-call ledger latency; it is not the commercial ledger implementation.
type independentAuditFixture struct {
	t                         *testing.T
	ctx                       context.Context
	provider                  db.Provider
	conn                      *gorm.DB
	client                    infraredis.Cmdable
	raw                       *redis.Client
	manager                   *ExptMangerImpl
	quotaDAO                  quotadao.IQuotaDAO
	quota                     repo.QuotaRepo
	seq                       atomic.Int64
	logMu                     sync.Mutex
	base, space, expt, r1, r2 int64
	keys                      []string
}

func (f *independentAuditFixture) log(op string, value any) {
	f.logMu.Lock()
	defer f.logMu.Unlock()
	b, err := json.Marshal(value)
	if err != nil {
		f.t.Errorf("FIXTURE_ERROR marshal evidence: %v", err)
		return
	}
	f.t.Logf("AUDIT_EVIDENCE seq=%d at=%s op=%s data=%s", f.seq.Add(1), time.Now().UTC().Format(time.RFC3339Nano), op, b)
}

func newIndependentAuditFixture(t *testing.T, initialModes ...entity.ExptRunMode) *independentAuditFixture {
	t.Helper()
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	require.Contains(t, dsn, "@tcp(127.0.0.1:3306)/expt_running_retry_test?", "FIXTURE_ERROR unauthorized or missing local DSN")
	redisAddr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	require.Contains(t, []string{"127.0.0.1:26379", "127.0.0.1:6379"}, redisAddr, "FIXTURE_ERROR authorized local or existing CI Redis endpoint")
	provider, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err, "FIXTURE_ERROR MySQL connection")
	client, err := infraredis.NewClient(&redis.Options{Addr: redisAddr, ContextTimeoutEnabled: true})
	require.NoError(t, err, "FIXTURE_ERROR Redis connection")
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	ctx := context.Background()
	base := int64(860000000000000000) + time.Now().UnixNano()%1000000000000
	f := &independentAuditFixture{t: t, ctx: ctx, provider: provider, conn: provider.NewSession(ctx, db.WithMaster()), client: client, raw: raw, base: base, space: base + 1, expt: base + 2, r1: base + 3, r2: base + 4}
	var identity struct{ Version, Schema string }
	require.NoError(t, f.conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	info, err := raw.Info(ctx, "server").Result()
	require.NoError(t, err)
	f.log("infrastructure", map[string]any{"sql": identity, "redis_server": info, "space": f.space, "expt": f.expt, "r1": f.r1, "r2": f.r2})
	f.keys = []string{fmt.Sprintf("expt_run_mutex_lock:%d", f.expt), fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1), fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r2), fmt.Sprintf("[experiment]quota_space_expt:%d", f.space), fmt.Sprintf("lock:quota_space_expt:%d", f.space), f.cidKey()}
	f.quotaDAO = quotadao.NewQuotaDAO(client)
	f.quota = exptrepo.NewQuotaService(f.quotaDAO, lock.NewRedisLocker(client))
	ctrl := gomock.NewController(t)
	config := confmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ZombieIntervalSecond: 36 * 3600, SpaceExptConcurLimit: 10}).AnyTimes()
	metric := metricmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).Do(func(space, mode int64) { f.log("metric_run", []int64{space, mode}) }).AnyTimes()
	metric.EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(space, kind, status int64, _ time.Time) { f.log("metric_complete", []int64{space, kind, status}) }).AnyTimes()
	items := exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(provider))
	turns := exptrepo.NewExptTurnResultRepo(nil, exptmysql.NewExptTurnResultDAO(provider), exptmysql.NewExptTurnEvaluatorResultRefDAO(provider))
	finish := exptrepo.NewExptRunFinishRepo(provider)
	termination, ok := finish.(repo.IExptRunTerminationRepo)
	require.True(t, ok)
	f.manager = &ExptMangerImpl{exptRepo: &independentAuditExperimentRepo{IExperimentRepo: exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), nil), f: f}, runLogRepo: exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider)), runFinishRepo: &independentAuditFinishRepo{IExptRunFinishRepo: finish, IExptRunTerminationRepo: termination, f: f}, itemResultRepo: items, turnResultRepo: turns, mutex: lock.NewRedisLockerWithHolder(client, "evaluation"), manualRetryLocker: func() lock.ILocker { return lock.NewRedisLocker(client) }, quotaRepo: &independentAuditQuotaRepo{QuotaRepo: f.quota, f: f}, configer: config, mtr: metric, idem: idemrepo.NewIdempotentService(idemredis.NewIdemDAO(client)), exptResultService: &ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns}, centralGuard: component.NewNoopCentralReservationGuard()}
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_run_log", "expt_stats"} {
			assert.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error, "FIXTURE_ERROR cleanup")
		}
		assert.NoError(t, f.conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", f.space, f.expt).Error)
		assert.NoError(t, raw.Del(ctx, f.keys...).Err())
		sqlDB, err := f.conn.DB()
		if assert.NoError(t, err) {
			assert.NoError(t, sqlDB.Close())
		}
		assert.NoError(t, raw.Close())
	})
	require.NoError(t, f.conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type) VALUES(?,?,?,?,?)", f.expt, f.space, 0, entity.ExptStatus_Pending, entity.ExptType_Offline).Error)
	require.NoError(t, f.conn.Create(&model.ExptStats{ID: base + 5, SpaceID: f.space, ExptID: f.expt}).Error)
	if len(initialModes) > 0 && initialModes[0] == entity.EvaluationModeRetryItems {
		ids := idgenmocks.NewMockIIDGenerator(ctrl)
		ids.EXPECT().GenID(gomock.Any()).Return(f.r1, nil)
		f.manager.idgenerator = ids
		run, joined, err := f.manager.LogRetryItemsRun(ctx, f.expt, entity.EvaluationModeRetryItems, f.space, nil, &entity.Session{UserID: "independent-frozen-audit"})
		require.NoError(t, err)
		require.False(t, joined)
		require.Equal(t, f.r1, run)
	} else {
		require.NoError(t, f.manager.LogRun(ctx, f.expt, f.r1, entity.EvaluationModeSubmit, f.space, nil, &entity.Session{UserID: "independent-frozen-audit"}))
	}
	require.NoError(t, f.manager.exptRepo.Update(ctx, &entity.Experiment{ID: f.expt, SpaceID: f.space, Status: entity.ExptStatus_Processing}))
	require.NoError(t, f.manager.runLogRepo.Update(ctx, f.expt, f.r1, f.space, map[string]any{"status": int64(entity.ExptStatus_Processing)}))
	return f
}

func (f *independentAuditFixture) cidKey() string {
	return fmt.Sprintf("[idempotent]idem_key:CompleteExpt:exptexec:onend:%d", f.r1)
}

func (f *independentAuditFixture) admission() string {
	f.t.Helper()
	value, err := f.raw.Get(f.ctx, f.keys[0]).Result()
	if errors.Is(err, redis.Nil) {
		return "<absent>"
	}
	require.NoError(f.t, err, "FIXTURE_ERROR admission read")
	return value
}

func (f *independentAuditFixture) slot() bool {
	f.t.Helper()
	q, err := f.quotaDAO.GetQuotaSpaceExpt(f.ctx, f.space)
	require.NoError(f.t, err, "FIXTURE_ERROR quota read")
	if q == nil {
		return false
	}
	_, ok := q.ExptID2RunTime[f.expt]
	return ok
}

func (f *independentAuditFixture) snapshot(stage string) {
	f.t.Helper()
	sql := map[string]any{}
	for _, table := range []string{"experiment", "expt_run_log", "expt_stats", "expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log"} {
		var rows []map[string]any
		require.NoError(f.t, f.conn.Table(table).Where("space_id=?", f.space).Order("id").Find(&rows).Error, "FIXTURE_ERROR snapshot")
		sql[table] = rows
	}
	redisState := map[string]any{}
	for _, key := range f.keys {
		value, err := f.raw.Get(f.ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			value = "<absent>"
		} else {
			require.NoError(f.t, err)
		}
		ttl, err := f.raw.PTTL(f.ctx, key).Result()
		require.NoError(f.t, err)
		redisState[key] = map[string]any{"value": value, "pttl_ns": ttl.Nanoseconds()}
	}
	f.log(stage, map[string]any{"sql": sql, "redis": redisState})
}

func (f *independentAuditFixture) seedSources(n int, success bool) {
	f.t.Helper()
	for i := 0; i < n; i++ {
		item := f.base + 100 + int64(i)
		version := f.base + 200 + int64(i)
		ext := []byte(`{}`)
		state, turnState, result := entity.ItemRunState_Processing, entity.TurnRunState_Processing, entity.ExptItemResultStateDefault
		if success {
			state, turnState, result = entity.ItemRunState_Success, entity.TurnRunState_Success, entity.ExptItemResultStateResulted
		} else {
			manual, err := json.Marshal(&entity.ManualRetryContext{RunID: f.r1, RequestID: fmt.Sprintf("original-%d-%s", i, xid.New().String()), Mode: entity.EvaluationModeFailRetry, Operator: "fixture", AcceptedAtMS: time.Now().UnixMilli()})
			require.NoError(f.t, err)
			ext, err = json.Marshal(map[string]string{entity.ManualRetryExtKey: string(manual)})
			require.NoError(f.t, err)
		}
		for _, row := range []any{&model.ExptItemResult{ID: f.base + 300 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, ItemID: item, ItemVersionID: version, Status: int32(state), Ext: &ext}, &model.ExptItemResultRunLog{ID: f.base + 400 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, ItemID: item, ItemVersionID: version, Status: int32(state), ResultState: gptr.Of(int32(result))}, &model.ExptTurnResult{ID: f.base + 500 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, ItemID: item, ItemVersionID: version, TurnID: 0, Status: int32(turnState)}, &model.ExptTurnResultRunLog{ID: f.base + 600 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, ItemID: item, ItemVersionID: version, TurnID: 0, Status: int32(turnState), Ext: []byte(`{}`)}} {
			require.NoError(f.t, f.conn.Create(row).Error, "FIXTURE_ERROR seed source")
		}
		f.keys = append(f.keys, fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, item))
	}
	require.NoError(f.t, NewQuotaService(f.quota, f.manager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}))
	port := f.manager.runFinishRepo.(repo.IExptRunTerminationRepo)
	for i := 0; i < n; i++ {
		s, err := port.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, f.base+100+int64(i))
		require.NoError(f.t, err, "FIXTURE_ERROR source binding")
		require.NotNil(f.t, s)
		require.Equal(f.t, f.r1, s.Current.RunID)
		require.Len(f.t, s.Turns, 1)
		if !success {
			require.NotEmpty(f.t, s.Current.RequestID)
		}
		f.log("original_source", s)
	}
}

type independentAuditExperimentRepo struct {
	repo.IExperimentRepo
	f *independentAuditFixture
}

func (r *independentAuditExperimentRepo) GetByID(ctx context.Context, expt, space int64) (*entity.Experiment, error) {
	x, err := r.IExperimentRepo.GetByID(ctx, expt, space)
	r.f.log("GetByID", map[string]any{"experiment": x, "error": fmt.Sprint(err)})
	return x, err
}

func (r *independentAuditExperimentRepo) Update(ctx context.Context, x *entity.Experiment) error {
	err := r.IExperimentRepo.Update(ctx, x)
	r.f.log("UpdateExperiment", map[string]any{"input": x, "error": fmt.Sprint(err)})
	return err
}

type independentAuditFinishRepo struct {
	repo.IExptRunFinishRepo
	repo.IExptRunTerminationRepo
	f *independentAuditFixture
}

func (r *independentAuditFinishRepo) FinishCurrentRun(ctx context.Context, q *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	x, err := r.IExptRunFinishRepo.FinishCurrentRun(ctx, q)
	r.f.log("FinishCurrentRun_SQL_committed", map[string]any{"request": q, "result": x, "error": fmt.Sprint(err)})
	return x, err
}

func (r *independentAuditFinishRepo) TerminateCurrentRun(ctx context.Context, q *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
	x, err := r.IExptRunTerminationRepo.TerminateCurrentRun(ctx, q)
	r.f.log("TerminateCurrentRun_SQL_committed", map[string]any{"request": q, "result": x, "error": fmt.Sprint(err)})
	return x, err
}

// This barrier is BEFORE the real quota repository acquires its lock. It never
// waits while holding that quota lock, so R2's Allow can make real progress.
type independentAuditQuotaRepo struct {
	repo.QuotaRepo
	f                *independentAuditFixture
	entered, proceed chan struct{}
	once             sync.Once
}

func (r *independentAuditQuotaRepo) CreateOrUpdate(ctx context.Context, space int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), session *entity.Session) error {
	r.f.log("quota_before_real_repository", map[string]any{"space": space, "barrier": r.entered != nil})
	if r.entered != nil {
		r.once.Do(func() { close(r.entered) })
		select {
		case <-r.proceed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	err := r.QuotaRepo.CreateOrUpdate(ctx, space, update, session)
	r.f.log("quota_after_real_repository", map[string]any{"error": fmt.Sprint(err)})
	return err
}

type independentAuditCreateBarrier struct {
	repo.IExptRunLogRepo
	f                *independentAuditFixture
	entered, proceed chan struct{}
}

func (r *independentAuditCreateBarrier) Create(ctx context.Context, x *entity.ExptRunLog) error {
	r.f.log("R2_Create_before_write", x)
	close(r.entered)
	select {
	case <-r.proceed:
	case <-ctx.Done():
		return ctx.Err()
	}
	err := r.IExptRunLogRepo.Create(ctx, x)
	r.f.log("R2_Create_real_completed", map[string]any{"run": x, "error": fmt.Sprint(err)})
	return err
}

type independentAuditSetFailure struct {
	idem.IdempotentService
	f      *independentAuditFixture
	failed atomic.Int32
}

func (i *independentAuditSetFailure) Set(ctx context.Context, key string, ttl time.Duration) error {
	if key == fmt.Sprintf("CompleteExpt:exptexec:onend:%d", i.f.r1) && i.failed.CompareAndSwap(0, 1) {
		i.f.log("final_CID_Set_injected_failure", map[string]any{"key": key, "ttl_ns": ttl.Nanoseconds()})
		return errors.New("injected once: final CID Redis SET failed before write")
	}
	err := i.IdempotentService.Set(ctx, key, ttl)
	i.f.log("CID_Set_real", map[string]any{"key": key, "ttl_ns": ttl.Nanoseconds(), "error": fmt.Sprint(err)})
	return err
}

func independentAuditWait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(15 * time.Second):
		t.Fatal("FIXTURE_ERROR barrier timeout")
	}
}

func independentAuditResult(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("FIXTURE_ERROR goroutine did not complete")
		return context.DeadlineExceeded
	}
}

func (f *independentAuditFixture) replay(manager *ExptMangerImpl) error {
	f.log("HandleEventCheck_terminal_replay_begin", map[string]any{"run": f.r1, "mode": entity.EvaluationModeSubmit})
	scheduler := &ExptSchedulerImpl{Manager: manager, Configer: manager.configer}
	err := scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		return errors.New("FIXTURE_ERROR terminal message reached execution")
	})(f.ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, ExptRunMode: entity.EvaluationModeSubmit, ExptType: entity.ExptType_Offline, CreatedAt: time.Now().Add(-time.Hour).Unix(), Session: &entity.Session{}})
	f.log("HandleEventCheck_terminal_replay_end", map[string]any{"error": fmt.Sprint(err), "mq_retry": errno.NeedMQRetry(err)})
	return err
}

func (f *independentAuditFixture) finishR1WithoutCID() {
	f.t.Helper()
	f.seedSources(1, true)
	fault := &independentAuditSetFailure{IdempotentService: f.manager.idem, f: f}
	f.manager.idem = fault
	f.snapshot("R1_before_standard_CompleteExpt")
	err := f.manager.CompleteExpt(f.ctx, f.expt, &f.r1, f.space, &entity.Session{})
	f.log("R1_standard_CompleteExpt_return", map[string]any{"error": fmt.Sprint(err)})
	require.NoError(f.t, err, "FIXTURE_ERROR initial R1 must actually finish")
	require.Equal(f.t, int32(1), fault.failed.Load())
	require.Equal(f.t, "<absent>", f.admission())
	require.False(f.t, f.slot())
	exists, err := f.raw.Exists(f.ctx, f.cidKey()).Result()
	require.NoError(f.t, err)
	require.Zero(f.t, exists)
	x, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(f.t, err)
	require.Equal(f.t, entity.ExptStatus_Success, x.Status)
	require.Equal(f.t, f.r1, x.LatestRunID)
	require.NotNil(f.t, x.EndAt)
	run, err := f.manager.runLogRepo.Get(f.ctx, f.expt, f.r1)
	require.NoError(f.t, err)
	require.Equal(f.t, int64(entity.ExptStatus_Success), run.Status)
	f.snapshot("R1_double_terminal_original_admission_released_CID_absent")
}

// Every Release, including absent keys, incurs the same 3s simulated IO. The
// select respects the *production* context. The subsequent Redis Lua performs
// actual run/token CAS, and is never executed using an uncancelled substitute.
type independentAuditSlowGuard struct {
	component.ICentralReservationGuard
	f          *independentAuditFixture
	round, seq int
	records    []map[string]any
	prefix     string
	before     func(context.Context, int)
}

func (g *independentAuditSlowGuard) key(run, item int64) string {
	return fmt.Sprintf("%s:%d:%d", g.prefix, run, item)
}

func (g *independentAuditSlowGuard) Release(ctx context.Context, scope string, run, item int64, token, reason string) error {
	g.seq++
	if g.before != nil {
		g.before(ctx, g.seq)
	}
	start := time.Now()
	deadline, hasDeadline := ctx.Deadline()
	key := g.key(run, item)
	absent, readErr := g.f.raw.Exists(g.f.ctx, key).Result()
	if readErr != nil {
		return readErr
	}
	r := map[string]any{"round": g.round, "seq": g.seq, "scope": scope, "run": run, "item": item, "token": token, "reason": reason, "key": key, "start": start.UTC().Format(time.RFC3339Nano), "deadline": deadline.UTC().Format(time.RFC3339Nano), "has_deadline": hasDeadline, "budget_ns": time.Until(deadline).Nanoseconds(), "already_absent": absent == 0, "simulated_IO_delay_ns": int64(3 * time.Second), "cas": "not_executed"}
	g.f.log("guard_release_start", r)
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	var err error
	select {
	case <-timer.C:
		const script = `local v=redis.call('GET',KEYS[1]); if not v then return 0 end; if v~=ARGV[1] then return -1 end; redis.call('DEL',KEYS[1]); return 1`
		cas, casErr := g.f.raw.Eval(ctx, script, []string{key}, fmt.Sprintf("%d/%s", run, token)).Int64()
		r["cas"] = cas
		err = casErr
		if casErr == nil && cas < 0 {
			err = errors.New("reservation run/token mismatch")
		}
	case <-ctx.Done():
		err = ctx.Err()
	}
	r["end"] = time.Now().UTC().Format(time.RFC3339Nano)
	r["elapsed_ns"] = time.Since(start).Nanoseconds()
	r["error"] = fmt.Sprint(err)
	r["ctx_error"] = fmt.Sprint(ctx.Err())
	g.records = append(g.records, r)
	g.f.log("guard_release_end", r)
	return err
}

func TestIndependentFrozenAuditReproduction(t *testing.T) {
	t.Run("F1A_PreCreateAdmission", func(t *testing.T) {
		f := newIndependentAuditFixture(t)
		f.finishR1WithoutCID()
		newManager := *f.manager
		b := &independentAuditCreateBarrier{IExptRunLogRepo: f.manager.runLogRepo, f: f, entered: make(chan struct{}), proceed: make(chan struct{})}
		newManager.runLogRepo = b
		ctx, stop := context.WithTimeout(f.ctx, 30*time.Second)
		defer stop()
		done := make(chan error, 1)
		var once sync.Once
		resume := func() { once.Do(func() { close(b.proceed) }) }
		defer resume()
		go func() {
			done <- newManager.LogRun(ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
		}()
		independentAuditWait(t, b.entered)
		require.Equal(t, "evaluation", f.admission())
		require.False(t, f.slot(), "pre-Create R2 must not have quota")
		var latest, count int64
		require.NoError(t, f.conn.Raw("SELECT latest_run_id FROM experiment WHERE id=?", f.expt).Scan(&latest).Error)
		require.Equal(t, f.r1, latest)
		require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log WHERE space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.r2).Scan(&count).Error)
		require.Zero(t, count)
		f.snapshot("F1A_R2_SETNX_evaluation_latest_R1_R2_row_absent_no_R2_quota")
		err := f.replay(f.manager)
		require.True(t, err == nil || errno.NeedMQRetry(err), "terminal replay may retry while R2 holds the original tick")
		owner := f.admission()
		f.snapshot("F1A_after_old_terminal_replay_before_R2_Create")
		resume()
		require.NoError(t, independentAuditResult(t, done), "FIXTURE_ERROR real R2 LogRun completion")
		f.snapshot("F1A_R2_real_Create_and_latest_completed")
		assert.Equal(t, "evaluation", owner, "BUSINESS_RED F1A: old R1 replay must preserve R2 pre-Create admission")
		require.NoError(t, f.replay(f.manager))
		require.Equal(t, "evaluation", f.admission(), "non-current R1 replay preserves committed R2")
	})
	t.Run("F1B_PostLatestRealQuota", func(t *testing.T) {
		f := newIndependentAuditFixture(t)
		f.finishR1WithoutCID()
		oldManager, newManager := *f.manager, *f.manager
		b := &independentAuditQuotaRepo{QuotaRepo: f.quota, f: f, entered: make(chan struct{}), proceed: make(chan struct{})}
		oldManager.quotaRepo = b
		var once sync.Once
		resume := func() { once.Do(func() { close(b.proceed) }) }
		defer resume()
		done := make(chan error, 1)
		go func() { done <- f.replay(&oldManager) }()
		independentAuditWait(t, b.entered)
		f.snapshot("F1B_old_AlreadyFinished_after_last_current_check_before_quota_lock")
		admitErr := newManager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeFailRetry, f.space, nil, &entity.Session{})
		if admitErr != nil {
			require.True(t, errno.NeedMQRetry(admitErr), "replacement retries before SETNX while old completion owns tick")
			require.Equal(t, "<absent>", f.admission())
			require.False(t, f.slot())
			f.snapshot("F1B_R2_blocked_before_admission_by_original_tick")
			resume()
			require.NoError(t, independentAuditResult(t, done))
			require.NoError(t, newManager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeFailRetry, f.space, nil, &entity.Session{}))
		}
		x, err := newManager.exptRepo.GetByID(f.ctx, f.expt, f.space)
		require.NoError(t, err)
		require.Equal(t, f.r2, x.LatestRunID)
		run, err := newManager.runLogRepo.Get(f.ctx, f.expt, f.r2)
		require.NoError(t, err)
		require.Equal(t, int64(entity.ExptStatus_Pending), run.Status)
		require.NoError(t, NewQuotaService(f.quota, newManager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}), "FIXTURE_ERROR R2 actual AllowExptRun")
		require.True(t, f.slot())
		require.Equal(t, "evaluation", f.admission())
		f.snapshot("F1B_R2_Create_latest_R2_and_actual_Allow_slot_acquired")
		resume()
		if admitErr == nil {
			require.NoError(t, independentAuditResult(t, done), "FIXTURE_ERROR old replay continuation")
		} else {
			require.NoError(t, f.replay(&oldManager), "old replay observes new Latest and cannot release R2")
		}
		retained := f.slot()
		f.snapshot("F1B_after_old_real_ReleaseExptRun")
		assert.True(t, retained, "BUSINESS_RED F1B: old R1 release must preserve the actual R2 space slot")
	})
	t.Run("F2_TwoWholeTerminalReplays", func(t *testing.T) {
		f := newIndependentAuditFixture(t)
		f.seedSources(3, false)
		require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": fmt.Sprintf("independent-%d", f.expt)}).Error)
		// CK and business MQ are outside this local test. Record the post-release
		// projection port if reached; all canonical/raw SQL termination is real.
		result := svcmocks.NewMockExptResultService(gomock.NewController(t))
		result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), f.space, f.expt, gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, ids []int64) error {
			f.log("projection_port_after_release", ids)
			return nil
		}).AnyTimes()
		result.EXPECT().GetIncompleteTurns(gomock.Any(), f.expt, f.space, gomock.Any()).Return(nil, nil).AnyTimes()
		f.manager.exptResultService = result
		g := &independentAuditSlowGuard{ICentralReservationGuard: component.NewNoopCentralReservationGuard(), f: f, prefix: fmt.Sprintf("independent-audit:%d:reservation", f.expt)}
		f.manager.centralGuard = g
		g.before = func(ctx context.Context, sequence int) {
			if sequence != 3 {
				return
			}
			key := fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)
			holder, err := f.raw.Get(f.ctx, key).Result()
			require.NoError(t, err, "real tick remains held after initial 5s TTL")
			ttl, err := f.raw.PTTL(f.ctx, key).Result()
			require.NoError(t, err)
			require.Greater(t, ttl, time.Duration(0))
			admitErr := f.manager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
			require.Error(t, admitErr, "slow whole completion excludes new admission")
			require.Equal(t, "<absent>", f.admission(), "initial terminal SQL already paired admission release")
			f.log("F2_actual_tick_renewed_after_6s_new_run_excluded", map[string]any{"holder": holder, "pttl_ns": ttl.Nanoseconds(), "new_admission_error": fmt.Sprint(admitErr), "ctx_error": fmt.Sprint(ctx.Err())})
		}
		port := f.manager.runFinishRepo.(repo.IExptRunTerminationRepo)
		tokens := map[int64]string{}
		for i := 0; i < 3; i++ {
			item := f.base + 100 + int64(i)
			s, err := port.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, item)
			require.NoError(t, err)
			tokens[item] = s.Current.RequestID
			key := g.key(f.r1, item)
			f.keys = append(f.keys, key)
			require.NoError(t, f.raw.Set(f.ctx, key, fmt.Sprintf("%d/%s", f.r1, s.Current.RequestID), time.Hour).Err())
		}
		f.snapshot("F2_three_original_SQL_sources_and_real_reservations")
		// Initial whole termination commits SQL; both further calls are legal
		// terminal MQ replays. Collect *all three* batches before business asserts.
		errorsByRound := []error{}
		for round := 0; round < 3; round++ {
			g.round = round
			var err error
			if round == 0 {
				_, err = f.manager.TerminateCurrentRun(f.ctx, f.expt, f.r1, f.space, &entity.Session{}, nil, entity.WithStatus(entity.ExptStatus_Terminated))
			} else {
				err = f.replay(f.manager)
			}
			errorsByRound = append(errorsByRound, err)
			f.log("F2_round_return", map[string]any{"round": round, "error": fmt.Sprint(err), "mq_retry": errno.NeedMQRetry(err)})
			f.snapshot(fmt.Sprintf("F2_after_round_%d", round))
		}
		// Structural checks classify fixture/routing defects separately from RED.
		require.GreaterOrEqual(t, len(g.records), 3)
		require.LessOrEqual(t, len(g.records), 9)
		require.Zero(t, len(g.records)%3, "every actual batch includes all three original sources")
		for i, r := range g.records {
			item := f.base + 100 + int64(i%3)
			require.Equal(t, item, r["item"])
			require.Equal(t, f.r1, r["run"])
			require.Equal(t, tokens[item], r["token"])
		}
		x, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
		require.NoError(t, err)
		require.Equal(t, entity.ExptStatus_Terminated, x.Status)
		require.Equal(t, f.r1, x.LatestRunID)
		require.NotNil(t, x.EndAt)
		run, err := f.manager.runLogRepo.Get(f.ctx, f.expt, f.r1)
		require.NoError(t, err)
		require.Equal(t, int64(entity.ExptStatus_Terminated), run.Status)
		for i := 0; i < 3; i++ {
			s, err := port.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, f.base+100+int64(i))
			require.NoError(t, err)
			require.Equal(t, entity.ItemRunState_Terminal, s.Current.Status)
			require.Equal(t, tokens[s.Current.ItemID], s.Current.RequestID)
		}
		tail, err := f.raw.Exists(f.ctx, g.key(f.r1, f.base+102)).Result()
		require.NoError(t, err)
		done, err := f.raw.Exists(f.ctx, f.cidKey()).Result()
		require.NoError(t, err)
		f.log("F2_final_progress", map[string]any{"tail_reservation_exists": tail, "space_slot": f.slot(), "admission": f.admission(), "Done_CID_exists": done, "replays_collected": 2})
		assert.NoError(t, errorsByRound[2], "BUSINESS_RED F2: legal replay must finish the bounded 3x3s batch")
		assert.Zero(t, tail, "BUSINESS_RED F2: repeated successful prefix must allow tail release")
		assert.False(t, f.slot(), "BUSINESS_RED F2: completed whole termination must release space quota")
		assert.Equal(t, "<absent>", f.admission(), "BUSINESS_RED F2: completed whole termination must release admission")
		assert.Equal(t, int64(1), done, "BUSINESS_RED F2: legal replay must advance Done")
	})
	t.Run("F2_BaseReleaseAPI_Diagnostic", func(t *testing.T) {
		// This API's complete function body is byte-identical at fixed badf and
		// head (baseline-api-proof.json). This runs that retained API on head,
		// not the full base whole-termination entry, and is diagnostic only.
		f := newIndependentAuditFixture(t)
		f.seedSources(3, false)
		require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": fmt.Sprintf("independent-%d", f.expt)}).Error)
		g := &independentAuditSlowGuard{ICentralReservationGuard: component.NewNoopCentralReservationGuard(), f: f, prefix: fmt.Sprintf("independent-audit:%d:reservation", f.expt)}
		f.manager.centralGuard = g
		port := f.manager.runFinishRepo.(repo.IExptRunTerminationRepo)
		sources, err := f.manager.captureTerminationSources(f.ctx, port, f.space, f.expt, f.r1, []int64{f.base + 100, f.base + 101, f.base + 102})
		require.NoError(t, err)
		for _, source := range sources {
			key := g.key(f.r1, source.Current.ItemID)
			f.keys = append(f.keys, key)
			require.NoError(t, f.raw.Set(f.ctx, key, fmt.Sprintf("%d/%s", f.r1, source.Current.RequestID), time.Hour).Err())
		}
		err = f.manager.withTerminationRunLock(f.ctx, f.expt, f.r1, func(ctx context.Context) error {
			committed, err := f.manager.commitTerminationItems(ctx, port, &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.r1, Finalize: true, Status: entity.ExptStatus_Terminated, Items: sources})
			if err != nil {
				return err
			}
			_, err = f.manager.releaseTerminationResourcesWithDebt(ctx, committed.Experiment, f.r1, sources, true, &entity.Session{})
			return err
		})
		require.NoError(t, err, "DIAGNOSTIC: unchanged base API gives each release its own 8s budget")
		require.Len(t, g.records, 3)
		for _, record := range g.records {
			require.Equal(t, int64(1), record["cas"])
			require.Greater(t, record["budget_ns"].(int64), int64(7*time.Second))
		}
		require.False(t, f.slot())
		f.snapshot("DIAGNOSTIC_unchanged_base_API_completed_3x3s_not_full_base_GREEN")
	})
}
