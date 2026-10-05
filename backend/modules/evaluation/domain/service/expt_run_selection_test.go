// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReadSetSelectionFilterBoundaryAndInputIsolation(t *testing.T) {
	for _, count := range []int{1000, 1001} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			values := make([]string, count)
			for i := range values {
				values[i] = strconv.Itoa(i + 1)
			}
			sc := &entity.EvalSetConfig{EvalSetID: 10, EvalSetVersionID: 10, ItemFilter: &entity.ExptItemFilter{QueryAndOr: "and", FilterFields: []*entity.ExptItemFilterField{
				{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: values},
				{FieldName: "topic", FieldType: "string", QueryType: "eq", Values: []string{"math"}},
				{FieldName: "tag", FieldType: "tag", QueryType: "in", Values: []string{"smoke"}},
			}}}
			raw := []*entity.EvaluationSetItem{{ItemID: 2000}, {ItemID: 1}}
			svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
				require.Equal(t, int64(3), p.SpaceID)
				require.Nil(t, p.VersionID)
				require.Empty(t, p.OrderBys)
				require.Equal(t, []string{"smoke"}, p.TagFilter.TagNames)
				require.Equal(t, entity.TagFilterRelationAnd, p.TagFilter.Relation)
				fields := p.Filter.FilterFields
				if count == 1000 {
					require.Len(t, fields, 2)
					require.Equal(t, "item_id", fields[0].FieldName)
					require.Len(t, fields[0].Values, 1000)
				} else {
					require.Len(t, fields, 1)
				}
				require.Equal(t, "topic", fields[len(fields)-1].FieldName)
				fields[len(fields)-1].Values[0] = "changed by dependency"
				return raw, gptr.Of(int64(2)), nil, nil, nil
			})
			f, err := newSetSelectionFilter(sc.ItemFilter)
			require.NoError(t, err)
			items, _, _, rawCount, err := readSetSelectionPage(context.Background(), svc, 3, sc, 100, nil, f)
			require.NoError(t, err)
			require.Equal(t, 2, rawCount)
			if count == 1000 {
				require.Len(t, items, 2)
			} else {
				require.Len(t, items, 1)
				require.Equal(t, int64(1), items[0].ItemID)
			}
			require.Equal(t, int64(2000), raw[0].ItemID, "filter must not overwrite the returned source slice")
			require.Equal(t, "math", sc.ItemFilter.FilterFields[1].Values[0], "do not expose caller-owned filter values")
		})
	}
}

func TestReadSetSelectionLegacyRetainsFilteredEmptyTermination(t *testing.T) {
	ctrl := gomock.NewController(t)
	legacy, svc, refs := newExptStartMultiSetTestExec(ctrl)
	refs.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).Times(0)
	values := make([]string, 1001)
	for i := range values {
		values[i] = strconv.Itoa(1000 + i)
	}
	expt := &entity.Experiment{ID: 1, SpaceID: 3, EvalConf: &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 10, ItemFilter: &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: values}}}}}}}
	svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
		require.Equal(t, int64(3), p.SpaceID)
		require.Nil(t, p.VersionID)
		require.Nil(t, p.Filter)
		require.Nil(t, p.PageToken)
		return []*entity.EvaluationSetItem{{ItemID: 1}}, gptr.Of(int64(2)), nil, gptr.Of("next"), nil
	}).Times(1)
	err := legacy.exptStartMultiSet(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"}}, expt)
	require.NoError(t, err)
}
