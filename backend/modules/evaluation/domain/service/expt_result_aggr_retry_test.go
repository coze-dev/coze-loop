// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// Mirrors the DAO's task-version/status CAS and its unconditional batch upsert.
type retryAggrStore struct {
	repo.IExptAggrResultRepo
	mu   sync.Mutex
	rows map[string]*entity.ExptAggrResult
}

func retryAggrKey(fieldType int32, fieldKey string) string {
	return fmt.Sprintf("%d:%s", fieldType, fieldKey)
}

func cloneRetryAggr(row *entity.ExptAggrResult) *entity.ExptAggrResult {
	copied := *row
	copied.AggrResult = append([]byte(nil), row.AggrResult...)
	return &copied
}

func newRetryAggrStore(rows ...*entity.ExptAggrResult) *retryAggrStore {
	store := &retryAggrStore{rows: make(map[string]*entity.ExptAggrResult)}
	for _, row := range rows {
		store.rows[retryAggrKey(row.FieldType, row.FieldKey)] = cloneRetryAggr(row)
	}
	return store
}

func (s *retryAggrStore) GetExptAggrResult(_ context.Context, _ int64, fieldType int32, fieldKey string) (*entity.ExptAggrResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[retryAggrKey(fieldType, fieldKey)]
	if row == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	return cloneRetryAggr(row), nil
}

func (s *retryAggrStore) GetExptAggrResultByExperimentID(_ context.Context, _ int64) ([]*entity.ExptAggrResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]*entity.ExptAggrResult, 0, len(s.rows))
	for _, row := range s.rows {
		rows = append(rows, cloneRetryAggr(row))
	}
	return rows, nil
}

func (s *retryAggrStore) BatchGetExptAggrResultByExperimentIDs(ctx context.Context, _ []int64) ([]*entity.ExptAggrResult, error) {
	return s.GetExptAggrResultByExperimentID(ctx, 1)
}

func (s *retryAggrStore) UpdateAndGetLatestVersion(_ context.Context, _ int64, fieldType int32, fieldKey string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[retryAggrKey(fieldType, fieldKey)]
	if row == nil {
		return 0, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	row.Version++
	row.Status = int32(2)
	return row.Version, nil
}

func (s *retryAggrStore) UpdateExptAggrResultByVersion(_ context.Context, row *entity.ExptAggrResult, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.rows[retryAggrKey(row.FieldType, row.FieldKey)]
	if stored != nil && stored.Version == version && stored.Status == int32(2) {
		stored.Score = row.Score
		stored.AggrResult = append([]byte(nil), row.AggrResult...)
		stored.Status = int32(1)
	}
	return nil
}

func (s *retryAggrStore) BatchCreateExptAggrResult(_ context.Context, rows []*entity.ExptAggrResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range rows {
		key := retryAggrKey(row.FieldType, row.FieldKey)
		if stored := s.rows[key]; stored != nil {
			stored.Score = row.Score
			stored.AggrResult = append([]byte(nil), row.AggrResult...)
		} else {
			s.rows[key] = cloneRetryAggr(row)
		}
	}
	return nil
}

func (s *retryAggrStore) CreateExptAggrResult(_ context.Context, row *entity.ExptAggrResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := retryAggrKey(row.FieldType, row.FieldKey)
	if s.rows[key] != nil {
		return fmt.Errorf("duplicate aggregate key %s", key)
	}
	s.rows[key] = cloneRetryAggr(row)
	return nil
}

func retryAggrRow(t *testing.T, fieldType entity.FieldType, key string, score float64) *entity.ExptAggrResult {
	t.Helper()
	raw, err := json.Marshal(&entity.AggregateResult{AggregatorResults: []*entity.AggregatorResult{{
		AggregatorType: entity.Average,
		Data:           &entity.AggregateData{DataType: entity.Double, Value: gptr.Of(score)},
	}}})
	require.NoError(t, err)
	return &entity.ExptAggrResult{SpaceID: 100, ExperimentID: 1, FieldType: int32(fieldType), FieldKey: key, Score: score, AggrResult: raw, Version: 3, Status: int32(1)}
}

func retryAggrData(t *testing.T, store *retryAggrStore, fieldType entity.FieldType, key string) *entity.AggregateResult {
	t.Helper()
	row, err := store.GetExptAggrResult(context.Background(), 1, int32(fieldType), key)
	require.NoError(t, err)
	var result entity.AggregateResult
	require.NoError(t, json.Unmarshal(row.AggrResult, &result))
	return &result
}

func newRetryAggrService(t *testing.T, store *retryAggrStore, score func(context.Context) *float64, frozen ...*entity.Experiment) *ExptAggrResultServiceImpl {
	t.Helper()
	ctrl := gomock.NewController(t)
	turns := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	records := svcMocks.NewMockEvaluatorRecordService(ctrl)
	expts := repoMocks.NewMockIExperimentRepo(ctrl)
	metric := metricsMocks.NewMockExptMetric(ctrl)
	locker := lockMocks.NewMockILocker(ctrl)
	refs := func(ctx context.Context) []*entity.ExptTurnEvaluatorResultRef {
		if score(ctx) == nil {
			return nil
		}
		return []*entity.ExptTurnEvaluatorResultRef{{EvaluatorVersionID: 100, EvaluatorResultID: 11, Alias: "judge"}}
	}
	turns.EXPECT().GetTurnEvaluatorResultRefByExptID(gomock.Any(), int64(100), int64(1)).DoAndReturn(
		func(ctx context.Context, _, _ int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
			return refs(ctx), nil
		}).AnyTimes()
	turns.EXPECT().GetTurnEvaluatorResultRefByEvaluatorVersionID(gomock.Any(), int64(100), int64(1), int64(100)).DoAndReturn(
		func(ctx context.Context, _, _, _ int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
			return refs(ctx), nil
		}).AnyTimes()
	records.EXPECT().BatchGetEvaluatorRecordForAggr(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ []int64) ([]*entity.EvaluatorRecordAggr, error) {
			if value := score(ctx); value != nil {
				return []*entity.EvaluatorRecordAggr{{ID: 11, Status: entity.EvaluatorRunStatusSuccess, Score: value}}, nil
			}
			return nil, nil
		}).AnyTimes()
	turns.EXPECT().ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), gomock.Any(), int64(100)).DoAndReturn(
		func(ctx context.Context, _ int64, _ []int32, _, limit, _ int64) ([]*entity.ExptTurnResult, int64, error) {
			if limit == 50 {
				assert.False(t, contexts.CtxWriteDB(ctx), "target scan should retain the caller context")
			}
			if limit == 500 {
				if value := score(ctx); value != nil {
					return []*entity.ExptTurnResult{{WeightedScore: value}}, 0, nil
				}
			}
			return nil, 0, nil
		}).AnyTimes()
	expt := &entity.Experiment{
		ID: 1, SpaceID: 100, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf: &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvaluatorConfs: []*entity.ExptEvaluatorConf{{EvaluatorVersionID: 100, Alias: "judge"}}}}},
	}
	if len(frozen) > 0 {
		expt = frozen[0]
	}
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(100)).Return(expt, nil).AnyTimes()
	metric.EXPECT().EmitCalculateExptAggrResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	locker.EXPECT().Unlock(gomock.Any()).Return(true, nil).AnyTimes()
	return &ExptAggrResultServiceImpl{exptAggrResultRepo: store, exptTurnResultRepo: turns, evaluatorRecordService: records, experimentRepo: expts, metric: metric, locker: locker}
}

func TestRetryAggr_ClearsScoresAndPreservesOtherFields(t *testing.T) {
	annotation := retryAggrRow(t, entity.FieldType_Annotation, "7", 0.6)
	store := newRetryAggrStore(
		retryAggrRow(t, entity.FieldType_EvaluatorScore, "100:judge", 0.25),
		retryAggrRow(t, entity.FieldType_WeightedScore, "1", 0.25), annotation,
	)
	svc := newRetryAggrService(t, store, func(context.Context) *float64 { return nil })
	require.NoError(t, svc.CreateExptAggrResult(context.Background(), 100, 1))
	assert.Empty(t, retryAggrData(t, store, entity.FieldType_EvaluatorScore, "100:judge").AggregatorResults)
	assert.Empty(t, retryAggrData(t, store, entity.FieldType_WeightedScore, "1").AggregatorResults)
	unchanged, err := store.GetExptAggrResult(context.Background(), 1, int32(entity.FieldType_Annotation), "7")
	require.NoError(t, err)
	assert.Equal(t, annotation, unchanged)
	assert.NotEmpty(t, retryAggrData(t, store, entity.FieldType_TargetLatency, entity.AggrResultFieldKey_TargetLatency).AggregatorResults)

	ctrl := gomock.NewController(t)
	evaluatorSvc := svcMocks.NewMockEvaluatorService(ctrl)
	annotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
	tagRPC := rpcMocks.NewMockITagRPCAdapter(ctrl)
	expts := svc.experimentRepo.(*repoMocks.MockIExperimentRepo)
	expts.EXPECT().MGetBasicByID(gomock.Any(), []int64{1}).Return([]*entity.Experiment{{ID: 1, SpaceID: 100}}, nil)
	expts.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.ExptEvaluatorRef{{EvaluatorID: 10, EvaluatorVersionID: 100}}, nil)
	evaluatorSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), []int64{100}, true).Return([]*entity.Evaluator{{ID: 10, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 100}}}, nil)
	annotateRepo.EXPECT().BatchGetExptTurnAnnotateRecordRefs(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.ExptTurnAnnotateRecordRef{{TagKeyID: 7}}, nil)
	tagRPC.EXPECT().BatchGetTagInfo(gomock.Any(), int64(100), []int64{7}).Return(map[int64]*entity.TagInfo{7: {TagKeyId: 7, TagKeyName: "annotation"}}, nil)
	svc.evaluatorService, svc.exptAnnotateRepo, svc.tagRPCAdapter = evaluatorSvc, annotateRepo, tagRPC
	readback, err := svc.BatchGetExptAggrResultByExperimentIDs(context.Background(), 100, []int64{1})
	require.NoError(t, err)
	require.Len(t, readback, 1)
	assert.Empty(t, readback[0].EvaluatorResults["100:judge"].AggregatorResults)
	assert.Empty(t, readback[0].WeightedResults)
	assert.NotEmpty(t, readback[0].AnnotationResults[7].AggregatorResults)
}

func TestRetryAggr_InlineSentinelAndValidZeroRemainDistinct(t *testing.T) {
	store := newRetryAggrStore(retryAggrRow(t, entity.FieldType_EvaluatorScore, "0", 0.25))
	svc := newRetryAggrService(t, store, func(context.Context) *float64 { return gptr.Of(0.0) })
	require.NoError(t, svc.CreateExptAggrResult(context.Background(), 100, 1))
	assert.Empty(t, retryAggrData(t, store, entity.FieldType_EvaluatorScore, "0").AggregatorResults)
	for _, key := range []struct {
		fieldType entity.FieldType
		key       string
	}{{entity.FieldType_EvaluatorScore, "100:judge"}, {entity.FieldType_WeightedScore, "1"}} {
		data := retryAggrData(t, store, key.fieldType, key.key)
		require.NotEmpty(t, data.AggregatorResults)
		assert.Equal(t, 0.0, data.AggregatorResults[0].GetScore())
	}
}

type retryAggrTaskKey struct{}

func TestRetryAggr_DelayedTaskCannotOverwriteNewResult(t *testing.T) {
	for _, tc := range []struct {
		name        string
		old, latest *float64
		firstCreate bool
	}{
		{name: "old_score_cannot_restore_cleared_score", old: gptr.Of(0.25)},
		{name: "old_empty_cannot_clear_new_score", latest: gptr.Of(0.9)},
		{name: "old_score_cannot_replace_new_score", old: gptr.Of(0.25), latest: gptr.Of(0.9)},
		{name: "concurrent_first_create", old: gptr.Of(0.25), latest: gptr.Of(0.9), firstCreate: true},
		{name: "first_empty_result_rejects_old_score", old: gptr.Of(0.25), firstCreate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newRetryAggrStore()
			if !tc.firstCreate {
				store = newRetryAggrStore(retryAggrRow(t, entity.FieldType_EvaluatorScore, "100:judge", 0.5), retryAggrRow(t, entity.FieldType_WeightedScore, "1", 0.5))
			}
			started, resume := make(chan struct{}), make(chan struct{})
			var once sync.Once
			svc := newRetryAggrService(t, store, func(ctx context.Context) *float64 {
				if ctx.Value(retryAggrTaskKey{}) == "old" {
					once.Do(func() { close(started); <-resume })
					return tc.old
				}
				return tc.latest
			})
			done := make(chan error, 1)
			go func() {
				done <- svc.CreateExptAggrResult(context.WithValue(context.Background(), retryAggrTaskKey{}, "old"), 100, 1)
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("old calculation did not start")
			}
			latestErr := svc.CreateExptAggrResult(context.Background(), 100, 1)
			close(resume)
			require.NoError(t, latestErr)
			select {
			case oldErr := <-done:
				assert.NoError(t, oldErr)
			case <-time.After(5 * time.Second):
				t.Fatal("old calculation did not finish")
			}
			for _, key := range []struct {
				fieldType entity.FieldType
				key       string
			}{{entity.FieldType_EvaluatorScore, "100:judge"}, {entity.FieldType_WeightedScore, "1"}} {
				data := retryAggrData(t, store, key.fieldType, key.key)
				if tc.latest == nil {
					assert.Empty(t, data.AggregatorResults)
				} else {
					require.NotEmpty(t, data.AggregatorResults)
					assert.Equal(t, *tc.latest, data.AggregatorResults[0].GetScore())
				}
			}
		})
	}
}

func TestRetryAggr_UpdateClearsScoresAndRejectsStaleWeightedResult(t *testing.T) {
	for _, tc := range []struct {
		name            string
		old, latest     *float64
		missingWeighted bool
	}{
		{name: "clear_both_scores"},
		{name: "old_score_cannot_restore_cleared_score", old: gptr.Of(0.25)},
		{name: "old_empty_cannot_clear_new_score", latest: gptr.Of(0.9)},
		{name: "old_score_cannot_replace_new_score", old: gptr.Of(0.25), latest: gptr.Of(0.9)},
		{name: "concurrent_first_weighted_create", old: gptr.Of(0.25), latest: gptr.Of(0.9), missingWeighted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []*entity.ExptAggrResult{retryAggrRow(t, entity.FieldType_EvaluatorScore, "100:judge", 0.5)}
			if !tc.missingWeighted {
				rows = append(rows, retryAggrRow(t, entity.FieldType_WeightedScore, "1", 0.5))
			}
			store := newRetryAggrStore(rows...)
			started, resume := make(chan struct{}), make(chan struct{})
			var once sync.Once
			svc := newRetryAggrService(t, store, func(ctx context.Context) *float64 {
				if ctx.Value(retryAggrTaskKey{}) == "old" {
					once.Do(func() { close(started); <-resume })
					return tc.old
				}
				return tc.latest
			})
			param := &entity.UpdateExptAggrResultParam{SpaceID: 100, ExperimentID: 1, FieldType: entity.FieldType_EvaluatorScore, FieldKey: "100:judge"}
			done := make(chan error, 1)
			go func() {
				done <- svc.UpdateExptAggrResult(context.WithValue(context.Background(), retryAggrTaskKey{}, "old"), param)
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("old calculation did not start")
			}
			latestErr := svc.UpdateExptAggrResult(context.Background(), param)
			close(resume)
			require.NoError(t, latestErr)
			select {
			case oldErr := <-done:
				assert.NoError(t, oldErr)
			case <-time.After(5 * time.Second):
				t.Fatal("old calculation did not finish")
			}
			for _, key := range []struct {
				fieldType entity.FieldType
				key       string
			}{{entity.FieldType_EvaluatorScore, "100:judge"}, {entity.FieldType_WeightedScore, "1"}} {
				data := retryAggrData(t, store, key.fieldType, key.key)
				if tc.latest == nil {
					assert.Empty(t, data.AggregatorResults)
				} else {
					require.NotEmpty(t, data.AggregatorResults)
					assert.Equal(t, *tc.latest, data.AggregatorResults[0].GetScore())
				}
			}
		})
	}
}

func TestRetryAggr_FirstEmptyCalculationReservesConfiguredFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		expt *entity.Experiment
		keys []string
	}{
		{name: "single_set", expt: &entity.Experiment{ID: 1, SpaceID: 100, EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorVersionID: 100}}}, keys: []string{"100"}},
		{name: "multi_set_aliases", expt: &entity.Experiment{
			ID: 1, SpaceID: 100, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
			EvalConf: &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvaluatorConfs: []*entity.ExptEvaluatorConf{
				{EvaluatorVersionID: 100, Alias: "judge"}, {EvaluatorVersionID: 100, Alias: "other"}, {EvaluatorVersionID: 100, Alias: "judge"},
			}}}},
		}, keys: []string{"100:judge", "100:other"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newRetryAggrStore()
			svc := newRetryAggrService(t, store, func(context.Context) *float64 { return nil }, tc.expt)
			require.NoError(t, svc.CreateExptAggrResult(context.Background(), 100, 1))
			for _, key := range tc.keys {
				assert.Empty(t, retryAggrData(t, store, entity.FieldType_EvaluatorScore, key).AggregatorResults)
				row, err := store.GetExptAggrResult(context.Background(), 1, int32(entity.FieldType_EvaluatorScore), key)
				require.NoError(t, err)
				assert.Equal(t, int64(1), row.Version)
				assert.Equal(t, int32(1), row.Status)
			}
			assert.Empty(t, retryAggrData(t, store, entity.FieldType_WeightedScore, "1").AggregatorResults)
		})
	}
}

func TestRetryAggr_InlineRefsKeepExistingSentinelBucket(t *testing.T) {
	refs := NewTurnEvaluatorResultRefs(1, 1, 10, 100, &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{
		{InlineKey: "a", RecordID: 11}, {InlineKey: "b", RecordID: 12},
	}})
	require.Len(t, refs, 2)
	for _, ref := range refs {
		assert.Equal(t, int64(0), ref.EvaluatorVersionID)
		assert.Empty(t, ref.Alias)
	}
	ctrl := gomock.NewController(t)
	turns := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	records := svcMocks.NewMockEvaluatorRecordService(ctrl)
	turns.EXPECT().GetTurnEvaluatorResultRefByExptID(gomock.Any(), int64(100), int64(1)).Return(refs, nil)
	records.EXPECT().BatchGetEvaluatorRecordForAggr(gomock.Any(), []int64{11, 12}).Return([]*entity.EvaluatorRecordAggr{
		{ID: 11, Status: entity.EvaluatorRunStatusSuccess, Score: gptr.Of(0.2)},
		{ID: 12, Status: entity.EvaluatorRunStatusSuccess, Score: gptr.Of(0.6)},
	}, nil)
	svc := &ExptAggrResultServiceImpl{exptTurnResultRepo: turns, evaluatorRecordService: records}
	groups, err := svc.computeEvaluatorAggrGroup(context.Background(), 100, 1)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Contains(t, groups, "0")
	assert.InDelta(t, 0.4, groups["0"].Result().AggregatorResults[0].GetScore(), 1e-9)
}

type retryAggrStaleList struct{ *retryAggrStore }

func (s *retryAggrStaleList) GetExptAggrResultByExperimentID(context.Context, int64) ([]*entity.ExptAggrResult, error) {
	return nil, nil
}

func TestRetryAggr_InitializationConflictPreservesScoreUntilCAS(t *testing.T) {
	store := newRetryAggrStore(retryAggrRow(t, entity.FieldType_EvaluatorScore, "100:judge", 0.9), retryAggrRow(t, entity.FieldType_WeightedScore, "1", 0.9))
	var once sync.Once
	svc := newRetryAggrService(t, store, func(context.Context) *float64 {
		once.Do(func() {
			assert.Equal(t, 0.9, retryAggrData(t, store, entity.FieldType_EvaluatorScore, "100:judge").AggregatorResults[0].GetScore())
			assert.Equal(t, 0.9, retryAggrData(t, store, entity.FieldType_WeightedScore, "1").AggregatorResults[0].GetScore())
		})
		return nil
	})
	svc.exptAggrResultRepo = &retryAggrStaleList{store}
	require.NoError(t, svc.CreateExptAggrResult(context.Background(), 100, 1))
	assert.Empty(t, retryAggrData(t, store, entity.FieldType_EvaluatorScore, "100:judge").AggregatorResults)
	assert.Empty(t, retryAggrData(t, store, entity.FieldType_WeightedScore, "1").AggregatorResults)
}

func TestRetryAggr_ReplicaLagCannotRestoreClearedScores(t *testing.T) {
	for _, mode := range []string{"create_all", "specific_field"} {
		t.Run(mode, func(t *testing.T) {
			store := newRetryAggrStore(retryAggrRow(t, entity.FieldType_EvaluatorScore, "100:judge", 0.25), retryAggrRow(t, entity.FieldType_WeightedScore, "1", 0.25))
			svc := newRetryAggrService(t, store, func(ctx context.Context) *float64 {
				if contexts.CtxWriteDB(ctx) {
					return nil
				}
				return gptr.Of(0.25)
			})
			var err error
			if mode == "create_all" {
				err = svc.CreateExptAggrResult(context.Background(), 100, 1)
			} else {
				err = svc.UpdateExptAggrResult(context.Background(), &entity.UpdateExptAggrResultParam{SpaceID: 100, ExperimentID: 1, FieldType: entity.FieldType_EvaluatorScore, FieldKey: "100:judge"})
			}
			require.NoError(t, err)
			assert.Empty(t, retryAggrData(t, store, entity.FieldType_EvaluatorScore, "100:judge").AggregatorResults)
			assert.Empty(t, retryAggrData(t, store, entity.FieldType_WeightedScore, "1").AggregatorResults)
		})
	}
}

func TestRetryAggr_PrimaryRecordScoreOverridesStaleReplica(t *testing.T) {
	for _, mode := range []string{"create_all", "specific_field"} {
		t.Run(mode, func(t *testing.T) {
			store := newRetryAggrStore(retryAggrRow(t, entity.FieldType_EvaluatorScore, "100:judge", 0.25))
			svc := newRetryAggrService(t, store, func(context.Context) *float64 { return gptr.Of(0.25) })
			records := svcMocks.NewMockEvaluatorRecordService(gomock.NewController(t))
			records.EXPECT().BatchGetEvaluatorRecordForAggr(gomock.Any(), []int64{11}).DoAndReturn(func(ctx context.Context, _ []int64) ([]*entity.EvaluatorRecordAggr, error) {
				score := gptr.Of(0.25)
				if contexts.CtxWriteDB(ctx) {
					score = nil
				}
				return []*entity.EvaluatorRecordAggr{{ID: 11, Status: entity.EvaluatorRunStatusSuccess, Score: score}}, nil
			})
			svc.evaluatorRecordService = records
			var err error
			if mode == "create_all" {
				err = svc.CreateExptAggrResult(context.Background(), 100, 1)
			} else {
				err = svc.UpdateExptAggrResult(context.Background(), &entity.UpdateExptAggrResultParam{SpaceID: 100, ExperimentID: 1, FieldType: entity.FieldType_EvaluatorScore, FieldKey: "100:judge"})
			}
			require.NoError(t, err)
			assert.Empty(t, retryAggrData(t, store, entity.FieldType_EvaluatorScore, "100:judge").AggregatorResults)
		})
	}
}

type retryAggrLaggedMetadataRepo struct{ repo.IExperimentRepo }

func (r retryAggrLaggedMetadataRepo) GetByID(ctx context.Context, id, space int64) (*entity.Experiment, error) {
	if !contexts.CtxWriteDB(ctx) {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	return r.IExperimentRepo.GetByID(ctx, id, space)
}

func TestRetryAggr_FirstEmptyWithLaggedMetadataRejectsOldScore(t *testing.T) {
	store := newRetryAggrStore()
	started, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	svc := newRetryAggrService(t, store, func(ctx context.Context) *float64 {
		if ctx.Value(retryAggrTaskKey{}) == "old" {
			once.Do(func() { close(started); <-resume })
			return gptr.Of(0.25)
		}
		return nil
	})
	svc.experimentRepo = retryAggrLaggedMetadataRepo{svc.experimentRepo}
	done := make(chan error, 1)
	go func() {
		done <- svc.CreateExptAggrResult(context.WithValue(context.Background(), retryAggrTaskKey{}, "old"), 100, 1)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("old calculation did not reach source read")
	}
	latestErr := svc.CreateExptAggrResult(context.Background(), 100, 1)
	close(resume)
	require.NoError(t, latestErr)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("old calculation did not finish")
	}
	require.Empty(t, retryAggrData(t, store, entity.FieldType_EvaluatorScore, "100:judge").AggregatorResults, "a delayed score must not appear after the newer empty calculation, even when replica metadata is missing")
}
