// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// TestConvert_EvalSetSourceTypes 验证 eval_set_source_types 与 fuzzy_name 同级 (走 ExptFilterOption 顶层, 不进 filters):
// 未指定 → 留空透传 (由 DAO 层默认排除 MultiSetConfig(2) 含旧数据 NULL)，显式指定才按白名单。
func TestConvert_EvalSetSourceTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	conv := NewExptFilterConvertor(svcmocks.NewMockIEvalTargetService(ctrl))
	ctx := context.Background()

	t.Run("不传 filter_option → 留空, 交由 DAO 默认排除 MultiSet", func(t *testing.T) {
		got, err := conv.Convert(ctx, nil, 100)
		assert.NoError(t, err)
		assert.Empty(t, got.EvalSetSourceTypes)
	})

	t.Run("传 filter_option 但不传 source_types → 留空, 交由 DAO 默认排除 MultiSet", func(t *testing.T) {
		efo := &domain_expt.ExptFilterOption{FuzzyName: ptrStr("abc")}
		got, err := conv.Convert(ctx, efo, 100)
		assert.NoError(t, err)
		assert.Equal(t, "abc", got.FuzzyName)
		assert.Empty(t, got.EvalSetSourceTypes)
	})

	t.Run("显式 [SingleSet, MultiSetConfig] → 返回两者", func(t *testing.T) {
		efo := &domain_expt.ExptFilterOption{
			EvalSetSourceTypes: []domain_expt.ExptEvalSetSourceType{
				domain_expt.ExptEvalSetSourceType_SingleSet,
				domain_expt.ExptEvalSetSourceType_MultiSetConfig,
			},
		}
		got, err := conv.Convert(ctx, efo, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2}, got.EvalSetSourceTypes)
	})

	t.Run("显式 [MultiSetConfig] → 仅新实验", func(t *testing.T) {
		efo := &domain_expt.ExptFilterOption{
			EvalSetSourceTypes: []domain_expt.ExptEvalSetSourceType{domain_expt.ExptEvalSetSourceType_MultiSetConfig},
		}
		got, err := conv.Convert(ctx, efo, 100)
		assert.NoError(t, err)
		assert.Equal(t, []int64{2}, got.EvalSetSourceTypes)
	})
}

func ptrStr(s string) *string { return &s }
