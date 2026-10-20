// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
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

	t.Run("PO2DO_ext_unmarshal_error", func(t *testing.T) {
		po := &model.ExptTurnResultRunLog{
			ID:  1,
			Ext: []byte(`{invalid}`),
		}
		_, err := c.PO2DO(po)
		assert.Error(t, err)
	})
}

func TestExptRunLogReceiptConvertRoundTrip(t *testing.T) {
	raw := []byte(`[{"ItemIDs":[11],"CreateAt":1700000000},{"ItemIDs":null,"CreateAt":1700000001,"manual_retry_receipt":{"request_id":"rq1","payload_hash":"hash","accepted_count":1,"rejected_items":[{"item_id":12,"reason":"busy"}],"snapshot_at_ms":1700000000123},"unknown":[1,2,3]}]`)
	converter := NewExptRunLogConvertor()
	do, err := converter.PO2DO(&model.ExptRunLog{ID: 7, ExptID: 8, SpaceID: 9, ItemIds: gptr.Of(raw)})
	require.NoError(t, err)
	require.Equal(t, []int64{11}, do.GetItemIDs())
	po, err := converter.DO2PO(do)
	require.NoError(t, err)
	require.JSONEq(t, string(raw), string(*po.ItemIds))
}

func TestManualRetryTurnLogTraceSnapshot(t *testing.T) {
	converter := NewExptTurnResultRunLogConvertor()
	got, err := converter.PO2DO(&model.ExptTurnResultRunLog{TraceID: 55})
	require.NoError(t, err)
	require.Equal(t, int64(55), got.TraceID, "retry source must include the stored trace identity")
	po, err := converter.DO2PO(got)
	require.NoError(t, err)
	require.Equal(t, int64(55), po.TraceID)
}
