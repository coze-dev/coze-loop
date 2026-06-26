// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
)

func TestConvertEvaluatorRecord_Ext(t *testing.T) {
	t.Run("ext_roundtrip", func(t *testing.T) {
		do := &entity.EvaluatorRecord{
			ID:      1,
			SpaceID: 2,
			Ext:     map[string]string{"s": "str", "k": "v"},
		}
		po := ConvertEvaluatorRecordDO2PO(do)
		assert.NotNil(t, po)
		assert.NotNil(t, po.Ext)

		got, err := ConvertEvaluatorRecordPO2DO(po)
		assert.NoError(t, err)
		assert.Equal(t, "str", got.Ext["s"])
		assert.Equal(t, "v", got.Ext["k"])
	})

	t.Run("nil_ext", func(t *testing.T) {
		do := &entity.EvaluatorRecord{ID: 1}
		po := ConvertEvaluatorRecordDO2PO(do)
		assert.NotNil(t, po)
		assert.Nil(t, po.Ext)

		got, err := ConvertEvaluatorRecordPO2DO(po)
		assert.NoError(t, err)
		assert.Nil(t, got.Ext)
	})

	t.Run("po2do_ext_unmarshal_error", func(t *testing.T) {
		po := &model.EvaluatorRecord{
			ID:  1,
			Ext: gptr.Of([]byte(`{invalid}`)),
		}
		_, err := ConvertEvaluatorRecordPO2DO(po)
		assert.Error(t, err)
	})
}
