// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func keywordMappingFilter(keys ...string) *entity.ExptTurnResultFilterAccelerator {
	fields := make([]*entity.FieldFilter, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, &entity.FieldFilter{Key: key, Op: "LIKE", Values: []any{"miss"}})
	}
	return &entity.ExptTurnResultFilterAccelerator{KeywordSearch: &entity.KeywordFilter{Keyword: gptr.Of("miss"), ItemSnapshotFilter: &entity.ItemSnapshotFilter{StringMapFilters: fields}, EvalTargetDataFilters: []*entity.FieldFilter{{Key: "actual_output", Op: "LIKE", Values: []any{"miss"}}}}}
}

func TestKeywordMappingKeepsMappedORFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	sets := svcmocks.NewMockIEvaluationSetService(ctrl)
	sets.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any()).Return([]*entity.ItemSnapshotFieldMapping{
		{FieldKey: "fixture_label", MappingKey: "string_map", MappingSubKey: "string_key_0"},
		{FieldKey: "actual_output", MappingKey: "string_map", MappingSubKey: "string_key_1"},
		{FieldKey: "group", MappingKey: "string_map", MappingSubKey: "string_key_2"},
	}, "2026-11-06", nil)
	svc := ExptResultServiceImpl{evaluationSetService: sets}
	filter := keywordMappingFilter("fixture_label", "actual_output", "group", "trait")
	expt := &entity.Experiment{ID: 1, SpaceID: 100, EvalSetID: 10, EvalSetVersionID: 20, ExptType: entity.ExptType_Offline}
	require.NoError(t, svc.mapItemSnapshotFilter(context.Background(), filter, expt))
	require.False(t, filter.ItemSnapshotCondUnmatched, "missing one OR field must not suppress the other fields and target output")
	require.Len(t, filter.KeywordItemSnapshotCondBySet, 1)
	mapped := filter.KeywordItemSnapshotCondBySet[0]
	require.Equal(t, int64(20), mapped.EvalSetVersionID)
	require.Equal(t, "2026-11-06", mapped.SyncCkDate)
	require.Equal(t, []string{"string_key_0", "string_key_1", "string_key_2"}, []string{mapped.Cond.StringMapFilters[0].Key, mapped.Cond.StringMapFilters[1].Key, mapped.Cond.StringMapFilters[2].Key})
	require.Equal(t, "actual_output", filter.KeywordSearch.EvalTargetDataFilters[0].Key)
}

func TestKeywordMappingKeepsTargetWhenNoDatasetFieldExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	sets := svcmocks.NewMockIEvaluationSetService(ctrl)
	sets.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any()).Return([]*entity.ItemSnapshotFieldMapping{}, "2026-11-06", nil)
	svc := ExptResultServiceImpl{evaluationSetService: sets}
	filter := keywordMappingFilter("trait")
	require.NoError(t, svc.mapItemSnapshotFilter(context.Background(), filter, &entity.Experiment{SpaceID: 100, EvalSetID: 10, EvalSetVersionID: 20, ExptType: entity.ExptType_Offline}))
	require.False(t, filter.ItemSnapshotCondUnmatched)
	require.Empty(t, filter.KeywordItemSnapshotCondBySet)
	require.Empty(t, filter.KeywordSearch.ItemSnapshotFilter.StringMapFilters)
	require.Len(t, filter.KeywordSearch.EvalTargetDataFilters, 1)
}

func TestKeywordMappingRespectsHeterogeneousSetSchemas(t *testing.T) {
	for _, andCondition := range []bool{false, true} {
		name := "OR"
		if andCondition {
			name = "AND"
		}
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			sets := svcmocks.NewMockIEvaluationSetService(ctrl)
			sets.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *rpc.QueryItemSnapshotMappingRequest) ([]*entity.ItemSnapshotFieldMapping, string, error) {
				if req.DatasetID == 10 {
					require.Equal(t, int64(100), req.SpaceID)
					return []*entity.ItemSnapshotFieldMapping{{FieldKey: "a", MappingKey: "string_map", MappingSubKey: "string_key_1"}}, "2026-11-05", nil
				}
				require.Equal(t, int64(99), req.SpaceID)
				return []*entity.ItemSnapshotFieldMapping{{FieldKey: "b", MappingKey: "string_map", MappingSubKey: "string_key_2"}}, "2026-11-06", nil
			}).Times(2)
			svc := ExptResultServiceImpl{evaluationSetService: sets}
			filter := keywordMappingFilter("a", "b")
			if andCondition {
				filter.ItemSnapshotCond = filter.KeywordSearch.ItemSnapshotFilter
				filter.KeywordSearch.ItemSnapshotFilter = &entity.ItemSnapshotFilter{}
			}
			expt := &entity.Experiment{SpaceID: 100, EvalSetID: 10, EvalSetVersionID: 20, ExptType: entity.ExptType_Offline, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalConf: &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 20}, {EvalSetID: 11, EvalSetVersionID: 21, SourceSpaceID: 99}}}}
			require.NoError(t, svc.mapItemSnapshotFilter(context.Background(), filter, expt))
			if andCondition {
				require.True(t, filter.ItemSnapshotCondUnmatched)
				require.Empty(t, filter.ItemSnapshotCondBySet)
				return
			}
			require.False(t, filter.ItemSnapshotCondUnmatched)
			require.Len(t, filter.KeywordItemSnapshotCondBySet, 2)
			require.Equal(t, "string_key_1", filter.KeywordItemSnapshotCondBySet[0].Cond.StringMapFilters[0].Key)
			require.Equal(t, "string_key_2", filter.KeywordItemSnapshotCondBySet[1].Cond.StringMapFilters[0].Key)
			require.Equal(t, int64(99), filter.KeywordItemSnapshotCondBySet[1].SourceSpaceID)
		})
	}
}

func TestKeywordMappingDoesNotSwallowMappingFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	sets := svcmocks.NewMockIEvaluationSetService(ctrl)
	failure := errors.New("mapping RPC unavailable")
	sets.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any()).Return(nil, "", failure)
	svc := ExptResultServiceImpl{evaluationSetService: sets}
	require.ErrorIs(t, svc.mapItemSnapshotFilter(context.Background(), keywordMappingFilter("a"), &entity.Experiment{SpaceID: 100, EvalSetID: 10, EvalSetVersionID: 20, ExptType: entity.ExptType_Offline}), failure)
}
