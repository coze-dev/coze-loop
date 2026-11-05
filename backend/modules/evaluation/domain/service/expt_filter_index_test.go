// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type filterIndexTestRepo struct {
	repo.IExptTurnResultFilterRepo
	plan      *entity.ExptFilterPlan
	reads     []entity.ExptFilterTarget
	rows      map[entity.ExptFilterTarget][]*entity.ExptFilterRow
	snapshots int
	err       error
}

func TestFutureTimestampCheckDoesNotRetryWholeBatch(t *testing.T) {
	want := &entity.ExptTurnResultFilterEntity{SpaceID: 1, ExptID: 2, ItemID: 3, CreatedDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	ts := uint64(time.Now().Add(time.Hour).UnixMilli())
	index := &filterIndexTestRepo{plan: &entity.ExptFilterPlan{WriteMode: "v2", ReadTarget: entity.ExptFilterTargetV2, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetV2}}, rows: map[entity.ExptFilterTarget][]*entity.ExptFilterRow{entity.ExptFilterTargetV2: {{Filter: want, WriteTimestampMS: &ts}}}}
	fixture := newProjectionFixture(t)
	svc := fixture.service()
	index.IExptTurnResultFilterRepo = svc.exptTurnResultFilterRepo
	svc.exptTurnResultFilterRepo = index
	require.NoError(t, svc.compareCurrentItemFilterLocked(context.Background(), 1, 2, 3, 0), "future values remain logged and pending for a later scan, without NACKing sibling AUTO repairs")
}

func TestV2FilterMappingKeepsCategoryAndRejectsMissingKeys(t *testing.T) {
	filters := repomocks.NewMockIExptTurnResultFilterRepo(gomock.NewController(t))
	filters.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), int64(1), int64(2)).Return([]*entity.ExptTurnResultFilterKeyMapping{{FromField: "9", ToKey: "key11", FieldType: entity.FieldTypeEvaluator}, {FromField: "9", ToKey: "tag9", FieldType: entity.FieldTypeManualAnnotation}}, nil).AnyTimes()
	svc := ExptResultServiceImpl{exptTurnResultFilterRepo: filters}
	ctx := contexts.WithExptFilterPlan(context.Background(), &entity.ExptFilterPlan{ReadTarget: entity.ExptFilterTargetV2})
	query := &entity.ExptTurnResultFilterAccelerator{MapCond: &entity.ExptTurnResultFilterMapCond{EvaluatorScoreFilters: []*entity.FieldFilter{{Key: "9", Op: "=", Values: []any{0}}}, AnnotationStringFilters: []*entity.FieldFilter{{Key: "9", Op: "=", Values: []any{""}}}}}
	require.NoError(t, svc.mapTurnResultFilterCond(ctx, query, 1, 2))
	require.Len(t, query.MapCond.EvaluatorScoreFilters, 1)
	require.Equal(t, "key11", query.MapCond.EvaluatorScoreFilters[0].Key)
	require.Equal(t, "tag9", query.MapCond.AnnotationStringFilters[0].Key)
	query.MapCond.EvaluatorScoreFilters = []*entity.FieldFilter{{Key: "missing", Op: "=", Values: []any{0}}}
	require.Error(t, svc.mapTurnResultFilterCond(ctx, query, 1, 2))
	request := &entity.ExptTurnResultFilterAccelerator{MapCond: &entity.ExptTurnResultFilterMapCond{EvaluatorScoreFilters: []*entity.FieldFilter{{Key: "9", Op: "=", Values: []any{0}}}}}
	for page := 0; page < 2; page++ {
		copied := copyFilterAccelerator(request)
		require.NoError(t, svc.mapTurnResultFilterCond(ctx, copied, 1, 2))
		require.Equal(t, "key11", copied.MapCond.EvaluatorScoreFilters[0].Key)
		require.Equal(t, "9", request.MapCond.EvaluatorScoreFilters[0].Key)
	}
}

func TestV2InvalidItemKeyDoesNotDropRequiredCondition(t *testing.T) {
	ctx := contexts.WithExptFilterPlan(context.Background(), &entity.ExptFilterPlan{ReadTarget: entity.ExptFilterTargetV2})
	query := &entity.ExptTurnResultFilterAccelerator{ItemSnapshotCond: &entity.ItemSnapshotFilter{ItemKey: &entity.FieldFilter{Op: "=", Values: []any{"name"}}}}
	require.Error(t, (ExptResultServiceImpl{}).mapItemSnapshotFilter(ctx, query, &entity.Experiment{}))
	require.NotNil(t, query.ItemSnapshotCond.ItemKey)
}

func (r *filterIndexTestRepo) SnapshotPlan(context.Context, []int64) (*entity.ExptFilterPlan, error) {
	r.snapshots++
	return r.plan.Clone(), r.err
}

func (r *filterIndexTestRepo) SaveIndex(context.Context, []*entity.ExptTurnResultFilterEntity, *entity.ExptFilterPlan, entity.ExptFilterOperation) error {
	return r.err
}

func (r *filterIndexTestRepo) ReadIndex(_ context.Context, target entity.ExptFilterTarget, _ entity.ExptFilterReadQuery) ([]*entity.ExptFilterRow, error) {
	r.reads = append(r.reads, target)
	return r.rows[target], r.err
}

func (r *filterIndexTestRepo) ScanIndexKeys(context.Context, entity.ExptFilterTarget, *entity.ExptFilterTargetCursor, int) ([]entity.ExptFilterStoredKey, *entity.ExptFilterTargetCursor, error) {
	return nil, nil, r.err
}

func TestFilterPlanIsCapturedOnceForWholeRead(t *testing.T) {
	r := &filterIndexTestRepo{plan: &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: entity.ExptFilterTargetV2, ExptIDs: []int64{1, 2}, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}}}
	svc := ExptResultServiceImpl{exptTurnResultFilterRepo: r}
	ctx, err := svc.freezeFilterPlan(context.Background(), []int64{1, 2})
	require.NoError(t, err)
	r.plan.ReadTarget = entity.ExptFilterTargetLegacy
	ctx, err = svc.freezeFilterPlan(ctx, []int64{1})
	require.NoError(t, err)
	require.Equal(t, 1, r.snapshots)
	require.Equal(t, entity.ExptFilterTargetV2, contexts.ExptFilterPlanFromContext(ctx).ReadTarget)
	r.err = errors.New("configuration unavailable")
	_, err = svc.freezeFilterPlan(context.Background(), []int64{1, 2})
	require.ErrorContains(t, err, "configuration unavailable")
}

func TestFilterCheckReadsEveryRequiredWriteTarget(t *testing.T) {
	ts := uint64(time.Now().UnixMilli() - 1000)
	want := &entity.ExptTurnResultFilterEntity{SpaceID: 1, ExptID: 2, ItemID: 3, CreatedDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EvalTargetData: map[string]string{"actual_output": "fresh"}}
	r := &filterIndexTestRepo{plan: &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: entity.ExptFilterTargetV2, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}}, rows: map[entity.ExptFilterTarget][]*entity.ExptFilterRow{entity.ExptFilterTargetLegacy: {{Filter: want}}, entity.ExptFilterTargetV2: {{Filter: want, WriteTimestampMS: &ts}}}}
	svc := ExptResultServiceImpl{exptTurnResultFilterRepo: r}
	ok, err := svc.compareFilterIndex(context.Background(), r, 1, 2, 3, []*entity.ExptTurnResultFilterEntity{want})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}, r.reads)
	r.rows[entity.ExptFilterTargetLegacy] = nil
	ok, err = svc.compareFilterIndex(context.Background(), r, 1, 2, 3, []*entity.ExptTurnResultFilterEntity{want})
	require.NoError(t, err)
	require.False(t, ok)
	r.rows[entity.ExptFilterTargetLegacy] = []*entity.ExptFilterRow{{Filter: want}}
	ts = uint64(time.Now().Add(time.Hour).UnixMilli())
	_, err = svc.compareFilterIndex(context.Background(), r, 1, 2, 3, []*entity.ExptTurnResultFilterEntity{want})
	require.ErrorContains(t, err, "future_timestamp")
}
