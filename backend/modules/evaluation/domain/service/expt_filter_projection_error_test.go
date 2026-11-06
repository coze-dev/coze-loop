// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type projectionUnexpectedSourceCalls struct {
	t       *testing.T
	mu      sync.Mutex
	counts  map[string]int
	methods []string
}

func (r *projectionUnexpectedSourceCalls) Helper()           { r.t.Helper() }
func (r *projectionUnexpectedSourceCalls) Cleanup(fn func()) { r.t.Cleanup(fn) }
func (r *projectionUnexpectedSourceCalls) Errorf(format string, args ...any) {
	r.t.Errorf(format, args...)
}

func (r *projectionUnexpectedSourceCalls) Fatalf(format string, args ...any) {
	if strings.HasPrefix(format, "Unexpected call to") && len(args) > 1 {
		method := fmt.Sprint(args[1])
		lower := strings.ToLower(method)
		kind := "other_unexpected"
		switch {
		case strings.Contains(lower, "mapping") && (strings.Contains(lower, "init") || strings.Contains(lower, "create")):
			kind = "InitMapping"
		case strings.Contains(lower, "delete") || strings.Contains(lower, "remove"):
			kind = "Delete"
		case strings.Contains(lower, "update") || strings.Contains(lower, "patch"):
			kind = "Update"
		case strings.Contains(lower, "insert") || strings.Contains(lower, "create") || strings.Contains(lower, "save") || strings.Contains(lower, "upsert"):
			kind = "Insert"
		case strings.Contains(lower, "execute") || strings.Contains(lower, "invoke") || strings.Contains(lower, "evaluate") || strings.Contains(lower, "run"):
			kind = "model_execution"
		}
		r.mu.Lock()
		r.counts[kind]++
		r.methods = append(r.methods, method)
		r.mu.Unlock()
	}
	r.t.Fatalf(format, args...)
}

type projectionFailureSink struct {
	calls int
	keys  []entity.ExptFilterKey
}

func (s *projectionFailureSink) send(row *entity.ExptTurnResultFilterEntity) {
	s.calls++
	s.keys = append(s.keys, entity.ExptFilterKey{SpaceID: row.SpaceID, ExptID: row.ExptID, ItemID: row.ItemID, TurnID: row.TurnID})
}

type projectionFailureIndex struct {
	*repomocks.MockIExptTurnResultFilterRepo
	saveCalls  int
	legacy, v2 projectionFailureSink
}

func (r *projectionFailureIndex) SnapshotPlan(context.Context, []int64) (*entity.ExptFilterPlan, error) {
	return &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: entity.ExptFilterTargetV2, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}}, nil
}

func (r *projectionFailureIndex) SaveIndex(_ context.Context, rows []*entity.ExptTurnResultFilterEntity, plan *entity.ExptFilterPlan, _ entity.ExptFilterOperation) error {
	r.saveCalls++
	for _, row := range rows {
		for _, target := range plan.RequiredTargets {
			if target == entity.ExptFilterTargetLegacy {
				r.legacy.send(row)
			} else {
				r.v2.send(row)
			}
		}
	}
	return nil
}

func (r *projectionFailureIndex) ReadIndex(context.Context, entity.ExptFilterTarget, entity.ExptFilterReadQuery) ([]*entity.ExptFilterRow, error) {
	return nil, errors.New("unexpected index read")
}

func (r *projectionFailureIndex) ScanIndexKeys(context.Context, entity.ExptFilterTarget, entity.ExptFilterIndexScanQuery) ([]entity.ExptFilterStoredKey, *entity.ExptFilterTargetCursor, error) {
	return nil, nil, errors.New("unexpected index scan")
}

func projectionSourceDigest(t *testing.T, f *projectionFixture) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"experiment": f.expt, "turns": f.turns, "items": f.items, "item_refs": f.refs, "mappings": f.mappings, "evaluators": f.evaluators, "evaluator_refs": f.evaluatorRefs, "targets": f.targets, "annotations": f.annotations, "annotation_refs": f.annotationRefs, "tag_refs": f.tags})
	require.NoError(t, err)
	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])
	return hash
}

func TestBuildProjectionDependencyErrorsKeepVerifiedKey(t *testing.T) {
	stages := []string{"experiment", "turns", "items", "item_refs", "mappings", "evaluator_refs", "evaluators", "targets", "tag_refs", "annotation_refs", "annotations", "missing_target", "missing_evaluator", "missing_annotation", "foreign_target", "foreign_item", "foreign_experiment", "foreign_turn", "foreign_item_ref", "foreign_mapping", "foreign_target_unrequested", "foreign_evaluator_unrequested", "foreign_target_identity", "foreign_evaluator_identity", "multiple_turns", "multiple_items"}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			f := newProjectionFixture(t)
			f.turns[0].TurnID = 7
			f.targets[0].TurnID = 7
			for _, record := range f.evaluators {
				record.TurnID = 7
			}
			wantedIDs := []int64{3}
			failedRead := stage
			wantKey := true
			switch stage {
			case "experiment", "turns", "foreign_experiment", "foreign_turn", "foreign_item", "foreign_item_ref", "foreign_mapping", "foreign_target_unrequested", "foreign_evaluator_unrequested":
				wantKey = false
			case "multiple_turns":
				wantKey, failedRead = false, "mappings"
				other := *f.turns[0]
				other.ID, other.TurnID, other.TargetResultID = 12, 8, 0
				f.turns = append(f.turns, &other)
			case "multiple_items":
				wantKey, failedRead = false, "items"
				wantedIDs = []int64{3, 4}
			}
			reporter := &projectionUnexpectedSourceCalls{t: t, counts: map[string]int{"Insert": 0, "Update": 0, "Delete": 0, "InitMapping": 0, "model_execution": 0, "other_unexpected": 0}}
			ctrl := gomock.NewController(reporter)
			svc := f.service()
			fault := errors.New("injected " + stage)
			var reads []string
			read := func(ctx context.Context, current string) error {
				require.True(t, contexts.CtxWriteDB(ctx))
				reads = append(reads, current)
				if current == failedRead {
					return fault
				}
				return nil
			}
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.Experiment, error) {
				return f.expt, read(ctx, "experiment")
			}).AnyTimes()
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(1), int64(2), wantedIDs, gomock.Any(), false).DoAndReturn(func(ctx context.Context, _, _ int64, _ []int64, _ entity.Page, _ bool) ([]*entity.ExptTurnResult, int64, error) {
				if err := read(ctx, "turns"); err != nil {
					return nil, 0, err
				}
				return f.turns, int64(len(f.turns)), nil
			}).AnyTimes()
			turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(1), []int64{10}).DoAndReturn(func(ctx context.Context, _ int64, _ []int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
				return f.evaluatorRefs, read(ctx, "evaluator_refs")
			}).AnyTimes()
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			items.EXPECT().BatchGet(gomock.Any(), int64(1), int64(2), wantedIDs).DoAndReturn(func(ctx context.Context, _, _ int64, _ []int64) ([]*entity.ExptItemResult, error) {
				return f.items, read(ctx, "items")
			}).AnyTimes()
			refs := repomocks.NewMockIExptItemRefRepo(ctrl)
			refs.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), int64(1), int64(2), []int64{3}).DoAndReturn(func(ctx context.Context, _, _ int64, _ []int64) ([]*entity.ExptItemRef, error) {
				return f.refs, read(ctx, "item_refs")
			}).AnyTimes()
			annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
			annotations.EXPECT().GetExptTurnResultTagRefs(gomock.Any(), int64(2), int64(1)).DoAndReturn(func(ctx context.Context, _, _ int64) ([]*entity.ExptTurnResultTagRef, error) {
				return f.tags, read(ctx, "tag_refs")
			}).AnyTimes()
			annotations.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), int64(1), []int64{10}).DoAndReturn(func(ctx context.Context, _ int64, _ []int64) ([]*entity.ExptTurnAnnotateRecordRef, error) {
				return f.annotationRefs, read(ctx, "annotation_refs")
			}).AnyTimes()
			annotations.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), int64(1), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, _ []int64) ([]*entity.AnnotateRecord, error) {
				return f.annotations, read(ctx, "annotations")
			}).AnyTimes()
			targets := svcmocks.NewMockIEvalTargetService(ctrl)
			targets.EXPECT().BatchGetRecordByIDs(gomock.Any(), int64(9), []int64{50}).DoAndReturn(func(ctx context.Context, _ int64, _ []int64) ([]*entity.EvalTargetRecord, error) {
				return f.targets, read(ctx, "targets")
			}).AnyTimes()
			evaluators := svcmocks.NewMockEvaluatorRecordService(ctrl)
			evaluators.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(ctx context.Context, _ []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
				return f.evaluators, read(ctx, "evaluators")
			}).AnyTimes()
			filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
			filters.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), int64(1), int64(2)).DoAndReturn(func(ctx context.Context, _, _ int64) ([]*entity.ExptTurnResultFilterKeyMapping, error) {
				return f.mappings, read(ctx, "mappings")
			}).AnyTimes()
			index := &projectionFailureIndex{MockIExptTurnResultFilterRepo: filters}
			svc.ExperimentRepo = experiments
			svc.ExptTurnResultRepo = turns
			svc.ExptItemResultRepo = items
			svc.exptItemRefRepo = refs
			svc.ExptAnnotateRepo = annotations
			svc.evalTargetService = targets
			svc.evaluatorRecordService = evaluators
			svc.exptTurnResultFilterRepo = index
			expectedStage := failedRead
			switch stage {
			case "missing_target":
				f.targets = nil
				expectedStage = "targets"
			case "missing_evaluator":
				f.evaluators = f.evaluators[:1]
				expectedStage = "evaluators"
			case "missing_annotation":
				f.annotations = f.annotations[:2]
				expectedStage = "annotations"
			case "foreign_target":
				f.targets[0].SpaceID = 99
				expectedStage = "targets"
			case "foreign_item":
				f.items[0].SpaceID = 99
				expectedStage = "items"
			case "foreign_experiment":
				f.expt.SpaceID = 99
				expectedStage = "experiment"
			case "foreign_turn":
				f.turns[0].SpaceID = 99
				expectedStage = "turns"
			case "foreign_item_ref":
				f.refs[0].SpaceID = 99
				expectedStage = "item_refs"
			case "foreign_mapping":
				f.mappings[0].SpaceID = 99
				expectedStage = "mappings"
			case "foreign_target_unrequested":
				f.targets[0].ID, f.targets[0].ItemID, f.targets[0].TurnID = 999, 998, 997
				expectedStage = "targets"
			case "foreign_evaluator_unrequested":
				f.evaluators[0].ID, f.evaluators[0].ItemID, f.evaluators[0].TurnID = 999, 998, 997
				expectedStage = "evaluators"
			case "foreign_target_identity":
				f.targets[0].ItemID, f.targets[0].TurnID = 998, 997
				expectedStage = "targets"
			case "foreign_evaluator_identity":
				f.evaluators[0].ItemID, f.evaluators[0].TurnID = 998, 997
				expectedStage = "evaluators"
			}
			before := projectionSourceDigest(t, f)
			rows, buildErr := svc.BuildExptFilterProjection(context.Background(), 1, 2, wantedIDs)
			builderReads := append([]string(nil), reads...)
			reads = nil
			upsertErr := svc.UpsertExptTurnResultFilter(context.Background(), 1, 2, wantedIDs)
			after := projectionSourceDigest(t, f)
			record := map[string]any{"variant": stage, "expected_stage": expectedStage, "request_scope": map[string]any{"space": 1, "expt": 2, "item_ids": wantedIDs}, "unique_validated_key_expected": wantKey, "source_before_sha256": before, "source_after_sha256": after, "builder_reads": builderReads, "upsert_reads": reads, "builder_error": fmt.Sprint(buildErr), "direct_upsert_error": fmt.Sprint(upsertErr), "returned_rows": rows, "source_call_counts": reporter.counts, "unexpected_methods": reporter.methods, "index_save_calls": index.saveCalls, "legacy_send_calls": index.legacy.calls, "v2_send_calls": index.v2.calls}
			if wantKey {
				record["known_key"] = entity.ExptFilterKey{SpaceID: 1, ExptID: 2, ItemID: 3, TurnID: 7}
			} else {
				record["scope_only"] = true
			}
			encodedEvidence, encodeErr := json.Marshal(record)
			require.NoError(t, encodeErr)
			t.Logf("PROJECTION_FAILURE_EVIDENCE %s", encodedEvidence)
			require.Error(t, buildErr)
			require.Error(t, upsertErr)
			require.Empty(t, rows)
			require.Equal(t, before, after)
			for name, count := range reporter.counts {
				require.Zero(t, count, name)
			}
			require.Zero(t, index.saveCalls)
			require.Zero(t, index.legacy.calls)
			require.Zero(t, index.v2.calls)
			require.Contains(t, buildErr.Error(), "space=1 expt=2")
			require.Contains(t, buildErr.Error(), "stage="+expectedStage)
			if failedRead == expectedStage {
				require.ErrorIs(t, buildErr, fault)
				require.ErrorIs(t, upsertErr, fault)
			}
			if wantKey {
				assert.Contains(t, buildErr.Error(), "item=3 turn=7", "known source key must be retained by the original contract")
				assert.Contains(t, upsertErr.Error(), "item=3 turn=7", "direct upsert did not restore the known key")
			} else {
				assert.Contains(t, buildErr.Error(), "item=0 turn=0", "unknown, ambiguous or foreign data must not supply a key")
				assert.Contains(t, upsertErr.Error(), "item=0 turn=0")
			}
			assert.NotContains(t, buildErr.Error(), "item=998 turn=997")
			assert.NotContains(t, upsertErr.Error(), "item=998 turn=997")
		})
	}
}
