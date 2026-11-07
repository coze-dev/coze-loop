// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEvaluatorColumnsDedupeWithinExperimentAndAlias(t *testing.T) {
	for _, tc := range []struct {
		name string
		refs []*entity.ExptEvaluatorRef
		want map[int64]int
	}{
		{"empty", nil, nil},
		{"single_set", []*entity.ExptEvaluatorRef{{ExptID: 1, EvalSetID: 10, EvaluatorID: 4, EvaluatorVersionID: 42}}, map[int64]int{1: 1, 2: 0}},
		{"empty_alias_across_sets", []*entity.ExptEvaluatorRef{{ExptID: 1, EvalSetID: 10, EvaluatorID: 4, EvaluatorVersionID: 42}, {ExptID: 1, EvalSetID: 11, EvaluatorID: 4, EvaluatorVersionID: 42}}, map[int64]int{1: 1, 2: 0}},
		{"same_alias_across_sets", []*entity.ExptEvaluatorRef{{ExptID: 1, EvalSetID: 10, EvaluatorID: 4, EvaluatorVersionID: 42, Alias: "a", BindingConfig: []byte(`{"score_weight":1,"from_eval_set":[{"field_name":"x","from_field_name":"a"}]}`)}, {ExptID: 1, EvalSetID: 11, EvaluatorID: 4, EvaluatorVersionID: 42, Alias: "a", BindingConfig: []byte(`{"score_weight":2,"from_eval_set":[{"field_name":"x","from_field_name":"b"}]}`)}}, map[int64]int{1: 1, 2: 0}},
		{"different_aliases", []*entity.ExptEvaluatorRef{{ExptID: 1, EvalSetID: 10, EvaluatorID: 4, EvaluatorVersionID: 42, Alias: "a"}, {ExptID: 1, EvalSetID: 11, EvaluatorID: 4, EvaluatorVersionID: 42, Alias: "b"}}, map[int64]int{1: 2, 2: 0}},
		{"different_experiments", []*entity.ExptEvaluatorRef{{ExptID: 1, EvalSetID: 10, EvaluatorID: 4, EvaluatorVersionID: 42, Alias: "a"}, {ExptID: 2, EvalSetID: 11, EvaluatorID: 4, EvaluatorVersionID: 42, Alias: "a"}}, map[int64]int{1: 1, 2: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := repomocks.NewMockIExperimentRepo(ctrl)
			evals := svcmocks.NewMockEvaluatorService(ctrl)
			repo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1, 2}, int64(100)).Return(tc.refs, nil)
			if len(tc.refs) > 0 {
				evals.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Nil(), []int64{42}, true).Return([]*entity.Evaluator{{ID: 4, Name: "score", EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 42, Version: "0.0.1"}}}, nil)
			}
			before, err := json.Marshal(tc.refs)
			require.NoError(t, err)
			svc := ExptResultServiceImpl{ExperimentRepo: repo, evaluatorService: evals}
			_, columns, err := svc.getColumnEvaluators(context.Background(), 100, []int64{1, 2})
			require.NoError(t, err)
			got := map[int64]int{}
			for _, group := range columns {
				got[group.ExptID] = len(group.ColumnEvaluators)
			}
			if tc.want == nil {
				require.Empty(t, got)
			} else {
				require.Equal(t, tc.want, got)
			}
			after, err := json.Marshal(tc.refs)
			require.NoError(t, err)
			require.Equal(t, before, after, "column dedupe must retain per-set bindings and weights")
		})
	}
}

func TestEvaluatorResultsKeepPerItemRecordAndWeight(t *testing.T) {
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	records := svcmocks.NewMockEvaluatorRecordService(ctrl)
	turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{101, 102}).Return([]*entity.ExptTurnEvaluatorResultRef{{ExptTurnResultID: 101, EvaluatorVersionID: 42, EvaluatorResultID: 1001}, {ExptTurnResultID: 102, EvaluatorVersionID: 42, EvaluatorResultID: 1002}}, nil)
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{1001, 1002}, false, false).Return([]*entity.EvaluatorRecord{
		{ID: 1001, ItemID: 10, EvaluatorVersionID: 42, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.2)}}},
		{ID: 1002, ItemID: 11, EvaluatorVersionID: 42, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.8)}}},
	}, nil)
	builder := &ExptResultBuilder{SpaceID: 100, ExptTurnResultRepo: turns, evaluatorRecordService: records, turnResultDO: []*entity.ExptTurnResult{{ID: 101, ItemID: 10, WeightedScore: gptr.Of(0.2)}, {ID: 102, ItemID: 11, WeightedScore: gptr.Of(0.8)}}, ItemIDTurnID2TurnResultID: map[int64]map[int64]int64{10: {0: 101}, 11: {0: 102}}}
	require.NoError(t, builder.buildEvaluatorResult(context.Background()))
	a, b := builder.getTurnEvaluatorResult(context.Background(), 10, 0), builder.getTurnEvaluatorResult(context.Background(), 11, 0)
	require.Equal(t, int64(1001), a.EvaluatorRecords[42].ID)
	require.Equal(t, 0.2, *a.EvaluatorRecords[42].EvaluatorOutputData.EvaluatorResult.Score)
	require.Equal(t, 0.2, *a.WeightedScore)
	require.Equal(t, int64(1002), b.EvaluatorRecords[42].ID)
	require.Equal(t, 0.8, *b.EvaluatorRecords[42].EvaluatorOutputData.EvaluatorResult.Score)
	require.Equal(t, 0.8, *b.WeightedScore)
}
