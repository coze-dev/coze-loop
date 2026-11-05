// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
)

// Unimplemented methods panic, including every business write and execution call.
type projectionFixture struct {
	t              *testing.T
	expt           *entity.Experiment
	turns          []*entity.ExptTurnResult
	items          []*entity.ExptItemResult
	refs           []*entity.ExptItemRef
	mappings       []*entity.ExptTurnResultFilterKeyMapping
	evaluators     []*entity.EvaluatorRecord
	evaluatorRefs  []*entity.ExptTurnEvaluatorResultRef
	targets        []*entity.EvalTargetRecord
	annotations    []*entity.AnnotateRecord
	annotationRefs []*entity.ExptTurnAnnotateRecordRef
	tags           []*entity.ExptTurnResultTagRef
	fail           string
	pages          int
}
type projectionExperimentRepo struct {
	repo.IExperimentRepo
	f *projectionFixture
}

func (r projectionExperimentRepo) GetByID(ctx context.Context, id, space int64) (*entity.Experiment, error) {
	return r.f.expt, r.f.read(ctx, "experiment")
}

type projectionTurnRepo struct {
	repo.IExptTurnResultRepo
	f *projectionFixture
}

func (r projectionTurnRepo) ListTurnResultByItemIDs(ctx context.Context, space, expt int64, ids []int64, page entity.Page, desc bool) ([]*entity.ExptTurnResult, int64, error) {
	r.f.pages++
	if err := r.f.read(ctx, "turns"); err != nil {
		return nil, 0, err
	}
	start := page.Offset()
	end := start + page.Limit()
	if end > len(r.f.turns) {
		end = len(r.f.turns)
	}
	if start > end {
		start = end
	}
	return r.f.turns[start:end], int64(len(r.f.turns)), nil
}

func (r projectionTurnRepo) BatchGetTurnEvaluatorResultRef(ctx context.Context, space int64, ids []int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	return r.f.evaluatorRefs, r.f.read(ctx, "evaluator_refs")
}

type projectionItemRepo struct {
	repo.IExptItemResultRepo
	f *projectionFixture
}

func (r projectionItemRepo) BatchGet(ctx context.Context, space, expt int64, ids []int64) ([]*entity.ExptItemResult, error) {
	return r.f.items, r.f.read(ctx, "items")
}

type projectionRefRepo struct {
	repo.IExptItemRefRepo
	f *projectionFixture
}

func (r projectionRefRepo) MGetByExptIDAndItemIDs(ctx context.Context, space, expt int64, ids []int64) ([]*entity.ExptItemRef, error) {
	return r.f.refs, r.f.read(ctx, "item_refs")
}

type projectionFilterRepo struct {
	repo.IExptTurnResultFilterRepo
	f *projectionFixture
}

func (r projectionFilterRepo) GetExptTurnResultFilterKeyMappings(ctx context.Context, space, expt int64) ([]*entity.ExptTurnResultFilterKeyMapping, error) {
	return r.f.mappings, r.f.read(ctx, "mappings")
}

type projectionAnnotateRepo struct {
	repo.IExptAnnotateRepo
	f *projectionFixture
}

func (r projectionAnnotateRepo) GetExptTurnResultTagRefs(ctx context.Context, expt, space int64) ([]*entity.ExptTurnResultTagRef, error) {
	return r.f.tags, r.f.read(ctx, "tag_refs")
}

func (r projectionAnnotateRepo) GetExptTurnAnnotateRecordRefsByTurnResultIDs(ctx context.Context, space int64, ids []int64) ([]*entity.ExptTurnAnnotateRecordRef, error) {
	return r.f.annotationRefs, r.f.read(ctx, "annotation_refs")
}

func (r projectionAnnotateRepo) GetAnnotateRecordsByIDs(ctx context.Context, space int64, ids []int64) ([]*entity.AnnotateRecord, error) {
	return r.f.annotations, r.f.read(ctx, "annotations")
}

type projectionTargetService struct {
	IEvalTargetService
	f *projectionFixture
}

func (r projectionTargetService) BatchGetRecordByIDs(ctx context.Context, space int64, ids []int64) ([]*entity.EvalTargetRecord, error) {
	require.Equal(r.f.t, int64(9), space, "frozen target source space must be used")
	return r.f.targets, r.f.read(ctx, "targets")
}

type projectionEvaluatorService struct {
	EvaluatorRecordService
	f *projectionFixture
}

func (r projectionEvaluatorService) BatchGetEvaluatorRecord(ctx context.Context, ids []int64, deleted, full bool) ([]*entity.EvaluatorRecord, error) {
	require.False(r.f.t, deleted)
	require.False(r.f.t, full, "index must use stored preview")
	return r.f.evaluators, r.f.read(ctx, "evaluators")
}

func (f *projectionFixture) read(ctx context.Context, stage string) error {
	require.True(f.t, contexts.CtxWriteDB(ctx), "projection must read primary: "+stage)
	if stage == f.fail {
		return errors.New("injected " + stage)
	}
	return nil
}

func (f *projectionFixture) service() ExptResultServiceImpl {
	return ExptResultServiceImpl{ExperimentRepo: projectionExperimentRepo{f: f}, ExptTurnResultRepo: projectionTurnRepo{f: f}, ExptItemResultRepo: projectionItemRepo{f: f}, exptItemRefRepo: projectionRefRepo{f: f}, exptTurnResultFilterRepo: projectionFilterRepo{f: f}, ExptAnnotateRepo: projectionAnnotateRepo{f: f}, evalTargetService: projectionTargetService{f: f}, evaluatorRecordService: projectionEvaluatorService{f: f}, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
}

func newProjectionFixture(t *testing.T) *projectionFixture {
	date := time.Date(2025, 2, 3, 23, 30, 0, 0, time.FixedZone("test", 8*3600))
	empty := ""
	text := "preview"
	zero := 0.0
	one := 1.0
	latency := int64(1501)
	return &projectionFixture{
		t:              t,
		expt:           &entity.Experiment{ID: 2, SpaceID: 1, StartAt: &date, ExptType: entity.ExptType_Offline, EvalSetID: 20, EvalSetVersionID: 21, TargetSpaceID: 9, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalConf: &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 20, EvalSetVersionID: 21, SourceSpaceID: 8}, {EvalSetID: 30, EvalSetVersionID: 31, SourceSpaceID: 8}}}},
		turns:          []*entity.ExptTurnResult{{ID: 10, SpaceID: 1, ExptID: 2, ItemID: 3, TurnID: 0, ExptRunID: 4, TargetResultID: 50}},
		items:          []*entity.ExptItemResult{{ID: 11, SpaceID: 1, ExptID: 2, ItemID: 3, ItemIdx: 0, Status: entity.ItemRunState_Success}},
		refs:           []*entity.ExptItemRef{{SpaceID: 1, ExptID: 2, ItemID: 3, ItemVersionID: 32, EvalSetID: 30, EvalSetVersionID: 31, EvalSetSourceSpaceID: 8}},
		targets:        []*entity.EvalTargetRecord{{ID: 50, SpaceID: 9, ItemID: 3, TurnID: 0, ExperimentRunID: 4, EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{"actual_output": {Text: &empty}, "custom": {Text: &text}}, EvalTargetUsage: &entity.EvalTargetUsage{}, TimeConsumingMS: &latency}}},
		evaluators:     []*entity.EvaluatorRecord{{ID: 60, SpaceID: 1, ExperimentID: 2, ExperimentRunID: 4, ItemID: 3, TurnID: 0, EvaluatorVersionID: 70, Alias: "a", EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: &zero}}}, {ID: 61, SpaceID: 1, ExperimentID: 2, ExperimentRunID: 4, ItemID: 3, TurnID: 0, EvaluatorVersionID: 70, Alias: "b", EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: &one}}}},
		evaluatorRefs:  []*entity.ExptTurnEvaluatorResultRef{{SpaceID: 1, ExptID: 2, ExptTurnResultID: 10, EvaluatorVersionID: 70, Alias: "a", EvaluatorResultID: 60}, {SpaceID: 1, ExptID: 2, ExptTurnResultID: 10, EvaluatorVersionID: 70, Alias: "b", EvaluatorResultID: 61}},
		mappings:       []*entity.ExptTurnResultFilterKeyMapping{{SpaceID: 1, ExptID: 2, FieldType: entity.FieldTypeEvaluator, FromField: "70:a", ToKey: "key11"}, {SpaceID: 1, ExptID: 2, FieldType: entity.FieldTypeEvaluator, FromField: "70:b", ToKey: "key12"}, {SpaceID: 1, ExptID: 2, FieldType: entity.FieldTypeManualAnnotation, FromField: "80", ToKey: "key1"}, {SpaceID: 1, ExptID: 2, FieldType: entity.FieldTypeManualAnnotation, FromField: "81", ToKey: "81"}, {SpaceID: 1, ExptID: 2, FieldType: entity.FieldTypeManualAnnotation, FromField: "82", ToKey: "82"}},
		tags:           []*entity.ExptTurnResultTagRef{{SpaceID: 1, ExptID: 2, TagKeyID: 80}, {SpaceID: 1, ExptID: 2, TagKeyID: 81}, {SpaceID: 1, ExptID: 2, TagKeyID: 82}},
		annotationRefs: []*entity.ExptTurnAnnotateRecordRef{{SpaceID: 1, ExptID: 2, ExptTurnResultID: 10, TagKeyID: 80, AnnotateRecordID: 90}, {SpaceID: 1, ExptID: 2, ExptTurnResultID: 10, TagKeyID: 81, AnnotateRecordID: 91}, {SpaceID: 1, ExptID: 2, ExptTurnResultID: 10, TagKeyID: 82, AnnotateRecordID: 92}},
		annotations:    []*entity.AnnotateRecord{{ID: 90, SpaceID: 1, ExperimentID: 2, TagKeyID: 80, AnnotateData: &entity.AnnotateData{TagContentType: entity.TagContentTypeContinuousNumber, Score: &zero}}, {ID: 91, SpaceID: 1, ExperimentID: 2, TagKeyID: 81, AnnotateData: &entity.AnnotateData{TagContentType: entity.TagContentTypeFreeText, TextValue: &empty}}, {ID: 92, SpaceID: 1, ExperimentID: 2, TagKeyID: 82, TagValueID: 83, AnnotateData: &entity.AnnotateData{TagContentType: entity.TagContentTypeBoolean}}},
	}
}

func TestBuildExptFilterProjectionKeepsCompleteInstancesAndPresentZero(t *testing.T) {
	f := newProjectionFixture(t)
	s := f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	r := rows[0]
	require.Equal(t, map[string]string{"actual_output": "", "custom": "preview"}, r.EvalTargetData)
	require.Equal(t, map[string]float64{"key11": 0, "key12": 1}, r.EvaluatorScore)
	require.Equal(t, map[string]float64{"key1": 0}, r.AnnotationFloat)
	require.Equal(t, map[string]string{"81": "", "82": "83"}, r.AnnotationString)
	require.Equal(t, map[string]int64{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "total_latency": 1501}, r.EvalTargetMetrics)
	require.Equal(t, int64(30), r.EvalSetID)
	require.Equal(t, int64(31), r.EvalSetVersionID)
	require.Equal(t, int64(32), r.ItemVersionID)
	require.Equal(t, "2025-02-03", r.CreatedDate.Format("2006-01-02"))
	require.Equal(t, 0.5, *r.EvaluatorWeightedScore)
	require.Equal(t, entity.ItemRunState_Success, r.Status)
	*f.targets[0].EvalTargetOutputData.OutputFields["custom"].Text = "changed"
	*f.evaluators[1].EvaluatorOutputData.EvaluatorResult.Score = 7
	require.Equal(t, "preview", r.EvalTargetData["custom"])
	require.Equal(t, 1.0, r.EvaluatorScore["key12"])
}

func TestBuildExptFilterProjectionRejectsIncompleteSources(t *testing.T) {
	cases := map[string]func(*projectionFixture){
		"missing experiment": func(f *projectionFixture) { f.expt = nil }, "missing start date": func(f *projectionFixture) { f.expt.StartAt = nil }, "missing item": func(f *projectionFixture) { f.items = nil }, "wrong item ownership": func(f *projectionFixture) { f.items[0].SpaceID = 99 },
		"missing multiset ref": func(f *projectionFixture) { f.refs = nil }, "wrong set ref": func(f *projectionFixture) { f.refs[0].EvalSetVersionID = 999 }, "missing target": func(f *projectionFixture) { f.targets = nil }, "stub target": func(f *projectionFixture) { f.targets[0].EvalTargetOutputData = nil }, "nil output value": func(f *projectionFixture) { f.targets[0].EvalTargetOutputData.OutputFields["custom"] = nil },
		"missing evaluator": func(f *projectionFixture) { f.evaluators = f.evaluators[:1] }, "wrong evaluator alias": func(f *projectionFixture) { f.evaluators[1].Alias = "c" }, "missing evaluator mapping": func(f *projectionFixture) { f.mappings = f.mappings[1:] }, "duplicate mapping target": func(f *projectionFixture) { f.mappings[1].ToKey = "key11" }, "duplicate evaluator instance": func(f *projectionFixture) { f.evaluatorRefs = append(f.evaluatorRefs, f.evaluatorRefs[0]) },
		"missing annotation": func(f *projectionFixture) { f.annotations = f.annotations[:2] }, "nil numeric annotation": func(f *projectionFixture) { f.annotations[0].AnnotateData.Score = nil }, "nil text annotation": func(f *projectionFixture) { f.annotations[1].AnnotateData.TextValue = nil }, "nonfinite score": func(f *projectionFixture) { *f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Score = math.NaN() }, "status overflow": func(f *projectionFixture) { f.items[0].Status = entity.ItemRunState(int64(math.MaxInt32) + 1) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newProjectionFixture(t)
			change(f)
			s := f.service()
			rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.Error(t, err)
			require.Empty(t, rows)
		})
	}
}

func TestBuildExptFilterProjectionReturnsReadFailure(t *testing.T) {
	for _, stage := range []string{"experiment", "turns", "items", "item_refs", "mappings", "evaluator_refs", "evaluators", "targets", "tag_refs", "annotation_refs", "annotations"} {
		t.Run(stage, func(t *testing.T) {
			f := newProjectionFixture(t)
			f.fail = stage
			s := f.service()
			rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.ErrorContains(t, err, "injected "+stage)
			require.Empty(t, rows)
		})
	}
}

func TestBuildExptFilterProjectionDistinguishesAbsentFromEmptyAndUnboundTags(t *testing.T) {
	f := newProjectionFixture(t)
	f.targets[0].EvalTargetOutputData.OutputFields = nil
	f.targets[0].EvalTargetOutputData.EvalTargetUsage = nil
	f.targets[0].EvalTargetOutputData.TimeConsumingMS = nil
	f.tags = f.tags[:1]
	f.annotations[0].AnnotateData = nil
	score := 0.0
	f.turns[0].WeightedScore = &score
	s := f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	r := rows[0]
	require.Empty(t, r.EvalTargetData)
	require.Empty(t, r.EvalTargetMetrics)
	require.Empty(t, r.AnnotationString)
	require.Empty(t, r.AnnotationFloat)
	require.Equal(t, 0.0, *r.EvaluatorWeightedScore)
	*f.turns[0].WeightedScore = 8
	require.Equal(t, 0.0, *r.EvaluatorWeightedScore)
}

func TestBuildExptFilterProjectionInputAndEmptyBoundaries(t *testing.T) {
	s := ExptResultServiceImpl{}
	for _, ids := range [][]int64{nil, {0}, make([]int64, 101)} {
		rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, ids)
		require.Error(t, err)
		require.Empty(t, rows)
	}
	_, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.Error(t, err)
	f := newProjectionFixture(t)
	f.turns = nil
	f.expt.StartAt = nil
	s = f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestBuildExptFilterProjectionReadsAllTurnsWithoutMutatingSource(t *testing.T) {
	f := newProjectionFixture(t)
	f.targets = nil
	f.evaluators = nil
	f.evaluatorRefs = nil
	f.annotations = nil
	f.annotationRefs = nil
	f.tags = nil
	f.turns = nil
	for i := 0; i < 201; i++ {
		f.turns = append(f.turns, &entity.ExptTurnResult{ID: int64(1000 + i), SpaceID: 1, ExptID: 2, ItemID: 3, TurnID: int64(i)})
	}
	before := *f.turns[0]
	s := f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 201)
	require.Equal(t, 2, f.pages)
	require.Equal(t, before, *f.turns[0])
}

func TestBuildExptFilterProjectionRetainsValidReusedAndSharedRecords(t *testing.T) {
	f := newProjectionFixture(t)
	f.turns[0].ExptRunID = 40
	f.evaluators[0].SpaceID = 8
	f.evaluators[0].Ext = map[string]string{"caller_space_id": "1"}
	s := f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"key11": 0, "key12": 1}, rows[0].EvaluatorScore)
	f.evaluators[0].Ext["caller_space_id"] = "999"
	_, err = s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.Error(t, err)
}

func TestBuildExptFilterProjectionDoesNotInventInlineMappings(t *testing.T) {
	f := newProjectionFixture(t)
	score := 0.9
	f.evaluatorRefs = append(f.evaluatorRefs, &entity.ExptTurnEvaluatorResultRef{SpaceID: 1, ExptID: 2, ExptTurnResultID: 10, EvaluatorResultID: 62, SourceType: 2, InlineKey: "inline"})
	f.evaluators = append(f.evaluators, &entity.EvaluatorRecord{ID: 62, SpaceID: 1, ExperimentID: 2, ExperimentRunID: 4, ItemID: 3, SourceType: entity.EvaluatorRecordSourceTypeInline, InlineKey: "inline", TargetRecordID: 50, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: &score}}})
	s := f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"key11": 0, "key12": 1}, rows[0].EvaluatorScore)
	require.Equal(t, 0.5, *rows[0].EvaluatorWeightedScore)
}

func TestBuildExptFilterProjectionCorrectionAndTargetVersion(t *testing.T) {
	t.Run("explanation-only correction retains missing score", func(t *testing.T) {
		f := newProjectionFixture(t)
		explanation := "reviewed explanation"
		f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Correction = &entity.Correction{Explain: explanation}
		rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
		require.NoError(t, err)
		require.NotContains(t, rows[0].EvaluatorScore, "key11")
		require.Equal(t, 1.0, rows[0].EvaluatorScore["key12"])
		require.True(t, rows[0].EvaluatorScoreCorrected)
	})
	t.Run("target version mismatch", func(t *testing.T) {
		f := newProjectionFixture(t)
		f.expt.TargetVersionID = 5
		f.targets[0].TargetVersionID = 6
		s := f.service()
		_, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
		require.Error(t, err)
	})
}

type projectionHookConfig struct {
	component.IConfiger
	hit bool
}

func (c projectionHookConfig) GetExptTurnScoreHookConf(context.Context, int64, int64, []*entity.ExptEvaluatorVersionRef) (*entity.ExptTurnScoreHookConf, bool) {
	return &entity.ExptTurnScoreHookConf{URL: "unused"}, c.hit
}

func TestBuildExptFilterProjectionNeverRunsExternalScoreHook(t *testing.T) {
	f := newProjectionFixture(t)
	s := f.service()
	s.scoreCalculator = NewEvaluatorScoreCalculator(projectionHookConfig{hit: true}, nil)
	_, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.ErrorContains(t, err, "external score hook requires a persisted weighted score")
	value := 0.7
	f.turns[0].WeightedScore = &value
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Equal(t, 0.7, *rows[0].EvaluatorWeightedScore)
	f.turns[0].WeightedScore = nil
	f.evaluators[0].EvaluatorOutputData = nil
	f.evaluators[1].EvaluatorOutputData = nil
	rows, err = s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Nil(t, rows[0].EvaluatorWeightedScore)
}

func TestBuildExptFilterProjectionUsesInstanceWeights(t *testing.T) {
	f := newProjectionFixture(t)
	three, one := 3.0, 1.0
	f.expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EnableScoreWeight: true}
	f.expt.EvalConf.EvalSetConfigs[0].EvaluatorConfs = []*entity.ExptEvaluatorConf{{EvaluatorVersionID: 70, Alias: "a", ScoreWeight: &three}, {EvaluatorVersionID: 70, Alias: "b", ScoreWeight: &one}}
	f.expt.EvalConf.EvalSetConfigs[1].EvaluatorConfs = []*entity.ExptEvaluatorConf{{EvaluatorVersionID: 70, Alias: "a", ScoreWeight: &three}, {EvaluatorVersionID: 70, Alias: "b", ScoreWeight: &one}}
	s := f.service()
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Equal(t, 0.25, *rows[0].EvaluatorWeightedScore)
}

func TestBuildExptFilterProjectionOnlineAndLegacyDatasetVersions(t *testing.T) {
	t.Run("online always draft", func(t *testing.T) {
		f := newProjectionFixture(t)
		f.expt.ExptType = entity.ExptType_Online
		s := f.service()
		rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
		require.NoError(t, err)
		require.Zero(t, rows[0].EvalSetVersionID)
		require.Empty(t, rows[0].EvalTargetData)
	})
	t.Run("single set no item ref", func(t *testing.T) {
		f := newProjectionFixture(t)
		f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
		f.refs = nil
		s := f.service()
		s.exptItemRefRepo = nil
		rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
		require.NoError(t, err)
		require.Equal(t, int64(20), rows[0].EvalSetID)
		require.Equal(t, int64(21), rows[0].EvalSetVersionID)
	})
}

func TestBuildExptFilterProjectionRejectsMixedItemVersions(t *testing.T) {
	cases := map[string]func(*projectionFixture){
		"turn":      func(f *projectionFixture) { f.turns[0].ItemVersionID = 33 },
		"item":      func(f *projectionFixture) { f.items[0].ItemVersionID = 33 },
		"target":    func(f *projectionFixture) { f.targets[0].ItemVersionID = 33 },
		"evaluator": func(f *projectionFixture) { f.evaluators[0].ItemVersionID = 33 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newProjectionFixture(t)
			change(f)
			s := f.service()
			_, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.ErrorContains(t, err, "item version mismatch")
		})
	}
}
