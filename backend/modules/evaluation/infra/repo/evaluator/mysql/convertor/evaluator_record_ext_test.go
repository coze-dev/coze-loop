// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestEvaluatorOutputData_EvidenceArchiveJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"evidence_archive":{"schema_version":"v1","object_key":"evaluation/1/2/evidence.tar.gz","status":"complete","trigger":"success","size_bytes":123,"sha256":"abc","truncated_files":1,"deadline_at_unix_ms":456,"last_phase":"checkpoint","last_progress_at":400,"termination_source":"normal","archive_executor":"ago-daemon","agent_termination_result":"not_needed","error":"","fornax_evaluator_log_url":"https://must-not-be-persisted.example"}}`)

	var output entity.EvaluatorOutputData
	require.NoError(t, json.Unmarshal(raw, &output))
	assert.Empty(t, output.EvidenceArchive.FornaxEvaluatorLogURL, "read-only signed URL must not be loaded from persisted JSON")
	encoded, err := json.Marshal(&output)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(encoded, &got))
	archive, ok := got["evidence_archive"].(map[string]any)
	require.True(t, ok, "evidence_archive must remain an independent output field")
	assert.Equal(t, "evaluation/1/2/evidence.tar.gz", archive["object_key"])
	assert.NotContains(t, archive, "last_phase")
	assert.NotContains(t, archive, "deadline_at_unix_ms")
	assert.NotContains(t, archive, "last_progress_at")
	assert.NotContains(t, archive, "termination_source")
	assert.NotContains(t, archive, "archive_executor")
	assert.NotContains(t, archive, "agent_termination_result")
	_, persistedSignedURL := archive["fornax_evaluator_log_url"]
	assert.False(t, persistedSignedURL, "read-only signed URL must not be written to evaluator_record output_data")
}

func TestConvertEvaluatorRecordPO2AggrDO(t *testing.T) {
	t.Run("nil_po", func(t *testing.T) {
		assert.Nil(t, ConvertEvaluatorRecordPO2AggrDO(nil))
	})

	t.Run("maps_only_id_score_status", func(t *testing.T) {
		// 即便 PO 带了三个 blob, 聚合视图也只取 id/score/status, 完全不触碰大字段。
		po := &model.EvaluatorRecord{
			ID:         42,
			Score:      gptr.Of(0.75),
			Status:     int32(entity.EvaluatorRunStatusSuccess),
			InputData:  gptr.Of([]byte(`{"big":"blob"}`)),
			OutputData: gptr.Of([]byte(`{"evaluator_result":{"score":0.75}}`)),
			Ext:        gptr.Of([]byte(`{"k":"v"}`)),
		}
		aggr := ConvertEvaluatorRecordPO2AggrDO(po)
		assert.NotNil(t, aggr)
		assert.Equal(t, int64(42), aggr.ID)
		assert.NotNil(t, aggr.Score)
		assert.Equal(t, 0.75, *aggr.Score)
		assert.Equal(t, entity.EvaluatorRunStatusSuccess, aggr.Status)
	})

	t.Run("nil_score_preserved", func(t *testing.T) {
		po := &model.EvaluatorRecord{ID: 7, Score: nil, Status: int32(entity.EvaluatorRunStatusFail)}
		aggr := ConvertEvaluatorRecordPO2AggrDO(po)
		assert.NotNil(t, aggr)
		assert.Nil(t, aggr.Score)
		assert.Equal(t, entity.EvaluatorRunStatusFail, aggr.Status)
	})
}
