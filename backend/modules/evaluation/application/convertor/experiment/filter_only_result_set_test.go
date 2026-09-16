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

// TestConvert_OnlyResultSetEval 验证 only_result_set_eval 与 fuzzy_name 同级 (走 ExptFilterOption 顶层):
// true → 透传, 由 DAO 层落 target_id = 0; 缺省/false 不过滤。
func TestConvert_OnlyResultSetEval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	conv := NewExptFilterConvertor(svcmocks.NewMockIEvalTargetService(ctrl))
	ctx := context.Background()

	t.Run("不传 → 不过滤", func(t *testing.T) {
		got, err := conv.Convert(ctx, nil, 100)
		assert.NoError(t, err)
		assert.False(t, got.OnlyResultSetEval)
	})

	t.Run("显式 true → 透传", func(t *testing.T) {
		efo := &domain_expt.ExptFilterOption{OnlyResultSetEval: ptrBool(true)}
		got, err := conv.Convert(ctx, efo, 100)
		assert.NoError(t, err)
		assert.True(t, got.OnlyResultSetEval)
	})

	t.Run("显式 false → 不过滤", func(t *testing.T) {
		efo := &domain_expt.ExptFilterOption{OnlyResultSetEval: ptrBool(false)}
		got, err := conv.Convert(ctx, efo, 100)
		assert.NoError(t, err)
		assert.False(t, got.OnlyResultSetEval)
	})
}

func ptrBool(b bool) *bool { return &b }
