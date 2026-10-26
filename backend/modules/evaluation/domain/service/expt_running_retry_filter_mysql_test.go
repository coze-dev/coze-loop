// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	evaluatormodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentck "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

type filterSyncFixture struct {
	*lateWriteFixture
	svc    *ExptResultServiceImpl
	client infraredis.Cmdable
	server *miniredis.Miniredis
}

func newFilterSyncFixture(t *testing.T) *filterSyncFixture {
	t.Helper()
	f := newLateWriteFixture(t, "rq1")
	require.NoError(t, f.conn.Unscoped().Where("space_id=? AND turn_id<>0", f.space).Delete(&mysqlmodel.ExptTurnResultRunLog{}).Error)
	require.NoError(t, f.conn.Unscoped().Where("space_id=? AND turn_id<>0", f.space).Delete(&mysqlmodel.ExptTurnResult{}).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.TurnRunState_Success), "weighted_score": 0}).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("status", entity.TurnRunState_Success).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Success).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	ctrl := gomock.NewController(t)
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	records := svcmocks.NewMockEvaluatorRecordService(ctrl)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	experiments.EXPECT().GetByID(gomock.Any(), f.expt, f.space).Return(&entity.Experiment{ID: f.expt, SpaceID: f.space, StartAt: &now, ExptType: entity.ExptType_Offline, EvalSetID: f.base + 80, EvalSetVersionID: f.base + 81}, nil).AnyTimes()
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(ctx context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
		var rows []*evaluatormodel.EvaluatorRecord
		require.NoError(t, f.conn.Where("id IN ?", ids).Find(&rows).Error)
		result := []*entity.EvaluatorRecord{}
		for _, row := range rows {
			result = append(result, &entity.EvaluatorRecord{ID: row.ID, EvaluatorVersionID: row.EvaluatorVersionID, Alias: "alias", Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.0)}}})
		}
		return result, nil
	}).AnyTimes()
	target.EXPECT().BatchGetRecordByIDs(gomock.Any(), f.space, gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, ids []int64) ([]*entity.EvalTargetRecord, error) {
		return []*entity.EvalTargetRecord{{ID: ids[0], EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{"actual_output": {Text: gptr.Of(fmt.Sprint(ids[0]))}}}}}, nil
	}).AnyTimes()
	annotations.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), f.space, gomock.Any()).Return(nil, nil).AnyTimes()
	annotations.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), f.space, gomock.Any()).Return(nil, nil).AnyTimes()
	svc := &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), ExperimentRepo: experiments, evaluatorRecordService: records, evalTargetService: target, ExptAnnotateRepo: annotations, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil), resultPublishRepo: experimentrepo.NewExptResultPublishRepo(f.provider), exptItemRefRepo: experimentrepo.NewExptItemRefRepo(experimentmysql.NewExptItemRefDAO(f.provider)), resultPublishLocker: func() lock.ILocker { return filterRealTestLocker(client) }}
	return &filterSyncFixture{lateWriteFixture: f, svc: svc, client: client, server: server}
}

func (f *filterSyncFixture) key() string {
	return fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)
}

type filterStore struct {
	repo.IExptTurnResultFilterRepo
	mu         sync.Mutex
	rows       []*entity.ExptTurnResultFilterEntity
	fail       bool
	visible    bool
	beforeSave func([]*entity.ExptTurnResultFilterEntity)
}

func (s *filterStore) GetExptTurnResultFilterKeyMappings(context.Context, int64, int64) ([]*entity.ExptTurnResultFilterKeyMapping, error) {
	return nil, nil
}

func (s *filterStore) Save(ctx context.Context, rows []*entity.ExptTurnResultFilterEntity) error {
	if s.beforeSave != nil {
		s.beforeSave(rows)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("CK insert unavailable")
	}
	b, _ := json.Marshal(rows)
	_ = json.Unmarshal(b, &s.rows)
	return nil
}

func (s *filterStore) GetByExptIDItemIDs(ctx context.Context, _, _, _ string, _ []string) ([]*entity.ExptTurnResultFilterEntity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.visible {
		return nil, nil
	}
	b, _ := json.Marshal(s.rows)
	var out []*entity.ExptTurnResultFilterEntity
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (s *filterStore) GetMaterializedByExptIDItemIDs(ctx context.Context, a, b, c string, d []string) ([]*entity.ExptTurnResultFilterEntity, error) {
	return s.GetByExptIDItemIDs(ctx, a, b, c, d)
}

// Requeue between the candidate message and acquisition must replace all RQ1
// data. The actual OSS repository/DAO emits CK INSERT while the item is held.
func TestRunningRetryFilterSync_RealMySQLCurrentSourceCKInsert(t *testing.T) {
	f := newFilterSyncFixture(t)
	ctrl := gomock.NewController(t)
	sqlDB, mock, err := sqlmock.New(sqlmock.ValueConverterOption(filterSQLConverter{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := gorm.Open(clickhouse.New(clickhouse.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, conn.Callback().Create().Before("gorm:create").Register("assert_filter_lock", func(tx *gorm.DB) { require.True(t, f.server.Exists(f.key()), "CK INSERT escaped item mutex") }))
	provider := &filterCKProvider{conn: conn}
	mapping := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	mapping.EXPECT().GetByExptID(gomock.Any(), f.space, f.expt).Return(nil, nil)
	f.svc.exptTurnResultFilterRepo = experimentrepo.NewExptTurnResultFilterRepo(experimentck.NewExptTurnResultFilterDAO(provider, nil), mapping)
	// A stale ID-only event is prepared now; no RQ1 payload is supplied.
	holder := filterRealTestLocker(f.client)
	ok, err := holder.Lock(context.Background(), f.key(), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, errno.NeedMQRetry(f.svc.UpsertExptTurnResultFilter(context.Background(), f.space, f.expt, []int64{f.item})))
	f.token("rq2")
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=?", f.space).Updates(map[string]any{"target_result_id": f.base + 91, "item_version_id": f.version + 1}).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", f.version+1).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"item_version_id": f.version + 1, "eval_set_id": f.base + 92, "eval_set_version_id": f.base + 93}).Error)
	_, err = holder.Unlock(f.key())
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO `expt_turn_result_filter`").ExpectExec().WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), filterLockedSQLArg{f.server, f.key(), fmt.Sprint(f.item)}, fmt.Sprint(f.version+1), int64(0), "0", int64(entity.ItemRunState_Success), fmt.Sprintf("{\"actual_output\":\"%d\"}", f.base+91), "{}", "{}", "{}", "{}", "{}", int64(0), fmt.Sprint(f.base+92), float64(0), fmt.Sprint(f.base+93), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, f.svc.UpsertExptTurnResultFilter(context.Background(), f.space, f.expt, []int64{f.item}))
	require.NoError(t, mock.ExpectationsWereMet())
	t.Log("REAL_MYSQL_SOURCE_OSS_CK_INSERT_SQLMOCK_EXECUTED; no real CK server")
}

type filterCKProvider struct{ conn *gorm.DB }

func (p *filterCKProvider) NewSession(ctx context.Context) *gorm.DB { return p.conn.WithContext(ctx) }

func TestRunningRetryFilterSync_ResultedInvisibleWriteNextTick(t *testing.T) {
	f := newFilterSyncFixture(t)
	store := &filterStore{}
	f.svc.exptTurnResultFilterRepo = store
	store.beforeSave = func(rows []*entity.ExptTurnResultFilterEntity) {
		require.True(t, f.server.Exists(f.key()))
		require.Len(t, rows, 1)
		require.Equal(t, f.item, rows[0].ItemID)
	}
	for _, state := range []string{"save_error", "missing", "equal"} {
		store.fail = state == "save_error"
		store.visible = state == "equal"
		ready, err := f.svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), f.space, f.expt, f.run)
		if state != "save_error" {
			require.NoError(t, err)
			require.True(t, ready, "Save ACK permits finish even when physical rows are missing")
		} else {
			require.True(t, errno.NeedMQRetry(err))
			require.False(t, ready)
		}
	}
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptRunLog{}).Where("space_id=?", f.space).Update("status", entity.ExptStatus_Success).Error)
	ready, err := f.svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), f.space, f.expt, f.run)
	require.NoError(t, err)
	require.True(t, ready, "immutable source remains readable after run ends")
}

type filterSQLConverter struct{}

func (filterSQLConverter) ConvertValue(value any) (driver.Value, error) {
	switch value.(type) {
	case map[string]string, map[string]float64, map[string]int8, map[string]int64:
		raw, err := json.Marshal(value)
		return string(raw), err
	}
	return driver.DefaultParameterConverter.ConvertValue(value)
}

type filterLockedSQLArg struct {
	server     *miniredis.Miniredis
	key, value string
}

func (a filterLockedSQLArg) Match(value driver.Value) bool {
	return a.server.Exists(a.key) && value == a.value
}

func TestRunningRetryFilterSync_EnumerationAcrossPages(t *testing.T) {
	f := newFilterSyncFixture(t)
	scanner := f.itemRepo().(repo.IExptManualRetryFilterRepo)
	ext, err := entity.MergeManualRetryContext(nil, &entity.ManualRetryContext{RunID: f.run, RequestID: "rq1", Mode: entity.EvaluationModeFailRetry})
	require.NoError(t, err)
	rows := make([]*mysqlmodel.ExptItemResult, 0, 405)
	for i := 0; i < 405; i++ {
		rows = append(rows, &mysqlmodel.ExptItemResult{ID: f.base + 10000 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1000 + int64(i), Status: int32(entity.ItemRunState_Success), Ext: &ext})
	}
	require.NoError(t, f.conn.CreateInBatches(rows, 100).Error)
	var ids []int64
	cursor := int64(0)
	pages := 0
	for {
		page, next, err := scanner.ScanManualRetryFilterItems(context.Background(), f.space, f.expt, f.run, cursor, 200)
		require.NoError(t, err)
		ids = append(ids, page...)
		pages++
		if next == 0 {
			break
		}
		require.Greater(t, next, cursor)
		cursor = next
	}
	require.Len(t, ids, 406)
	require.Equal(t, 3, pages)
	require.Contains(t, ids, f.item+1404, "last page Resulted candidates must not disappear")
	oldExt, err := entity.MergeManualRetryContext(nil, &entity.ManualRetryContext{RunID: f.run - 1, RequestID: "old", Mode: entity.EvaluationModeFailRetry})
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, rows[0].ItemID).Updates(map[string]any{"expt_run_id": f.run - 1, "ext": oldExt}).Error)
	_, _, err = scanner.ScanManualRetryFilterItems(context.Background(), f.space, f.expt, f.run, 0, 200)
	require.NoError(t, err, "old run is outside current enumeration")
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, rows[1].ItemID).Update("ext", []byte(`{"manual_retry":"broken"}`)).Error)
	_, _, err = scanner.ScanManualRetryFilterItems(context.Background(), f.space, f.expt, f.run, 0, 200)
	require.Error(t, err, "damaged context must not become an empty set")
}

func TestRunningRetryFilterSync_NoManualNoFilterIO(t *testing.T) {
	f := newFilterSyncFixture(t)
	f.token("")
	f.svc.exptTurnResultFilterRepo = nil
	f.svc.resultPublishRepo = nil
	ready, err := f.svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), f.space, f.expt, f.run)
	require.NoError(t, err)
	require.True(t, ready)
}

func TestRunningRetryFilterSync_PendingPublicationNeverFinishes(t *testing.T) {
	for _, status := range []entity.ItemRunState{entity.ItemRunState_Queueing, entity.ItemRunState_Processing, entity.ItemRunState_Success} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newFilterSyncFixture(t)
			store := &filterStore{visible: true}
			f.svc.exptTurnResultFilterRepo = store
			result := entity.ExptItemResultStateDefault
			if status == entity.ItemRunState_Success {
				result = entity.ExptItemResultStateLogged
			}
			require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(status), "result_state": int32(result)}).Error)
			ready, err := f.svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), f.space, f.expt, f.run)
			require.True(t, errno.NeedMQRetry(err))
			require.False(t, ready)
			require.Nil(t, store.rows, "unfinished source must not be saved or marked complete")
		})
	}
}

// Match the current asynchronous COM-style repo contract: Save can succeed
// while its incomplete comparison reader cannot establish full materialization.
func TestRunningRetryFilterSync_AsyncPartialReaderCannotPass(t *testing.T) {
	f := newFilterSyncFixture(t)
	store := &filterStore{visible: true}
	f.svc.exptTurnResultFilterRepo = &partialFilterStore{store: store}
	require.NoError(t, f.svc.UpsertExptTurnResultFilter(context.Background(), f.space, f.expt, []int64{f.item}))
	ready, err := f.svc.materializedFilterMatchesLocked(context.Background(), f.space, f.expt, f.item)
	require.NoError(t, err)
	require.False(t, ready)
}

type partialFilterStore struct {
	repo.IExptTurnResultFilterRepo
	store *filterStore
}

func (s *partialFilterStore) Save(ctx context.Context, rows []*entity.ExptTurnResultFilterEntity) error {
	return s.store.Save(ctx, rows)
}

func (s *partialFilterStore) GetByExptIDItemIDs(ctx context.Context, a, b, c string, d []string) ([]*entity.ExptTurnResultFilterEntity, error) {
	return s.store.GetByExptIDItemIDs(ctx, a, b, c, d)
}

func (s *partialFilterStore) GetExptTurnResultFilterKeyMappings(ctx context.Context, a, b int64) ([]*entity.ExptTurnResultFilterKeyMapping, error) {
	return s.store.GetExptTurnResultFilterKeyMappings(ctx, a, b)
}

// The current payload and primary source must be re-read when an old source
// cache is present; it is useful only as an ID selection, never as projection.
func TestRunningRetryFilterSync_CachedOldSourceIgnored(t *testing.T) {
	f := newFilterSyncFixture(t)
	old := f.source()
	f.token("rq2")
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=?", f.space).Update("target_result_id", f.base+99).Error)
	store := &filterStore{visible: true}
	f.svc.exptTurnResultFilterRepo = store
	ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: old})
	require.NoError(t, f.svc.UpsertExptTurnResultFilter(ctx, f.space, f.expt, []int64{f.item}))
	require.Len(t, store.rows, 1)
	require.Equal(t, fmt.Sprint(f.base+99), store.rows[0].EvalTargetData["actual_output"])
}

// Source fields are read beneath the item holder, including refs and scores;
// absence remains absence, and zero scores remain explicitly mapped.
func TestRunningRetryFilterSync_ScorePresenceAliasInline(t *testing.T) {
	for _, score := range []*float64{nil, gptr.Of(0.0)} {
		t.Run(fmt.Sprint(score == nil), func(t *testing.T) {
			f := newFilterSyncFixture(t)
			ctrl := gomock.NewController(t)
			records := svcmocks.NewMockEvaluatorRecordService(ctrl)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(ctx context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
				require.True(t, f.server.Exists(f.key()))
				return []*entity.EvaluatorRecord{{ID: f.base + 61, EvaluatorVersionID: f.base + 51, Alias: "alias", Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: score}}}, {ID: f.base + 62, EvaluatorVersionID: 0, Status: entity.EvaluatorRunStatusSuccess}}, nil
			})
			require.NoError(t, f.conn.Create(&mysqlmodel.ExptTurnEvaluatorResultRef{ID: f.base + 802, SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: f.base + 400, SourceType: 2, InlineKey: "inline", EvaluatorResultID: f.base + 62}).Error)
			filterRepo := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
			filterRepo.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), f.space, f.expt).Return([]*entity.ExptTurnResultFilterKeyMapping{{FieldType: entity.FieldTypeEvaluator, FromField: entity.EncodeEvaluatorInstanceKey(f.base+51, "alias"), ToKey: "key1"}}, nil)
			filterRepo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, rows []*entity.ExptTurnResultFilterEntity) error {
				require.True(t, f.server.Exists(f.key()))
				require.Len(t, rows, 1)
				if score == nil {
					require.Empty(t, rows[0].EvaluatorScore)
				} else {
					require.Equal(t, map[string]float64{"key1": 0}, rows[0].EvaluatorScore)
				}
				return nil
			})
			f.svc.evaluatorRecordService = records
			f.svc.exptTurnResultFilterRepo = filterRepo
			require.NoError(t, f.svc.UpsertExptTurnResultFilter(context.Background(), f.space, f.expt, []int64{f.item}))
		})
	}
}
