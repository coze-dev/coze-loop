// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
)

func TestAcceleratorScorePresenceOperators(t *testing.T) {
	for _, tc := range []struct {
		op   domain_expt.FilterOperatorType
		want string
	}{
		{domain_expt.FilterOperatorType_IsNull, "IS NULL"},
		{domain_expt.FilterOperatorType_IsNotNull, "IS NOT NULL"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			got, err := ConvertExptTurnResultFilterAccelerator(&domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{LogicOp: gptr.Of(domain_expt.FilterLogicOp_And), FilterConditions: []*domain_expt.FilterCondition{{
					Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_EvaluatorScore, FieldKey: gptr.Of("7580254027955322882")},
					Operator: tc.op,
				}}},
			})
			require.NoError(t, err)
			require.Len(t, got.MapCond.EvaluatorScoreFilters, 1)
			require.Equal(t, tc.want, got.MapCond.EvaluatorScoreFilters[0].Op)
			require.Empty(t, got.MapCond.EvaluatorScoreFilters[0].Values)
		})
	}
}
