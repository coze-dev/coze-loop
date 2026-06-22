// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExptTurnResultRunLogConvertor_Ext(t *testing.T) {
	c := NewExptTurnResultRunLogConvertor()

	t.Run("DO2PO_PO2DO_ext_roundtrip", func(t *testing.T) {
		do := &entity.ExptTurnResultRunLog{
			ID:                 1,
			SpaceID:            2,
			EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{}},
			Ext:                map[string]string{"k": "v", "n": "3"},
		}
		po, err := c.DO2PO(do)
		assert.NoError(t, err)
		assert.NotNil(t, po.Ext)

		got, err := c.PO2DO(po)
		assert.NoError(t, err)
		assert.Equal(t, "v", got.Ext["k"])
		assert.Equal(t, "3", got.Ext["n"])
	})

	t.Run("DO2PO_nil_ext", func(t *testing.T) {
		do := &entity.ExptTurnResultRunLog{
			ID:                 1,
			EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{}},
		}
		po, err := c.DO2PO(do)
		assert.NoError(t, err)
		assert.Nil(t, po.Ext)

		got, err := c.PO2DO(po)
		assert.NoError(t, err)
		assert.Nil(t, got.Ext)
	})
}
