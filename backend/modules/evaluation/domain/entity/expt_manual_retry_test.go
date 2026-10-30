// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExptRunLogItemsPreserveOptionalMetadata(t *testing.T) {
	raw := `[{"ItemIDs":[11],"CreateAt":1700000000,"future":{"id":9223372036854775806}},{"ItemIDs":null,"CreateAt":1700000001,"manual_retry_receipt":{"request_id":"rq1","payload_hash":"hash","accepted_count":1,"rejected_items":[{"item_id":12,"reason":"processing","future_reason":{"id":9223372036854775806}}],"snapshot_at_ms":1700000000123,"future_receipt":true}}]`
	var chunks []ExptRunLogItems
	require.NoError(t, json.Unmarshal([]byte(raw), &chunks))
	run := &ExptRunLog{ItemIds: chunks}
	require.Equal(t, []int64{11}, run.GetItemIDs())
	encoded, err := json.Marshal(run.ItemIds)
	require.NoError(t, err)
	require.JSONEq(t, raw, string(encoded))
	var fields []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Equal(t, `{"id":9223372036854775806}`, string(fields[0]["future"]))
	require.NoError(t, run.AppendItemIDs([]int64{13}))
	require.Equal(t, []int64{11, 13}, run.GetItemIDs())
	require.Error(t, run.AppendItemIDs([]int64{11}))
}

func TestManualRetryExtStrictMerge(t *testing.T) {
	ctx := &ManualRetryContext{RunID: 7, RequestID: "rq1", Mode: EvaluationModeFailRetry, Operator: "user", AcceptedAtMS: 1700000000123}
	raw, err := MergeManualRetryContext([]byte(`{"other":"keep","retry_yield_enabled":"true"}`), ctx)
	require.NoError(t, err)
	ext, err := ParseExptItemResultExt(raw)
	require.NoError(t, err)
	require.Equal(t, "keep", ext["other"])
	got, err := ParseManualRetryContext(ext)
	require.NoError(t, err)
	require.Equal(t, ctx, got)
	require.Equal(t, map[string]string{"other": "keep", "retry_yield_enabled": "true"}, WithoutManualRetryExt(ext))
	require.Contains(t, ext, "manual_retry")
	for _, raw := range []string{`{`, `[]`, `{"other":1}`, `{"other":null}`, `{"manual_retry":"{"}`, `{"manual_retry":"null"}`, `{"manual_retry":"{}"}`, `{"manual_retry":"{\"run_id\":7,\"request_id\":\"\",\"mode\":2}"}`} {
		_, err := ParseExptItemResultExt([]byte(raw))
		require.Error(t, err, raw)
		_, err = MergeManualRetryContext([]byte(raw), ctx)
		require.Error(t, err, raw)
	}
	for _, raw := range [][]byte{nil, []byte(`null`), []byte(`{}`)} {
		ext, err := ParseExptItemResultExt(raw)
		require.NoError(t, err)
		got, err := ParseManualRetryContext(ext)
		require.NoError(t, err)
		require.Nil(t, got)
	}
	_, err = MergeManualRetryContext(nil, &ManualRetryContext{RunID: 7, RequestID: "rq1", Mode: EvaluationModeSubmit})
	require.Error(t, err)
}

func TestRunLogMetadataCapacity(t *testing.T) {
	var chunks []ExptRunLogItems
	base := `[{"ItemIDs":null,"CreateAt":null,"future":"` + strings.Repeat("x", 65489) + `"}]`
	require.NoError(t, json.Unmarshal([]byte(base), &chunks))
	encoded, err := EncodeExptRunLogItems(chunks)
	require.NoError(t, err)
	require.Len(t, encoded, 65535)
	chunks = append(chunks, ExptRunLogItems{ManualRetryReceipt: &ManualRetryReceipt{RequestID: "rq2", PayloadHash: "hash", AcceptedCount: 0, RejectedItems: []*ManualRetryRejectedItem{{ItemID: 9223372036854775806, Reason: "reservation_mismatch"}}, SnapshotAtMS: 1700000000123}})
	_, err = EncodeExptRunLogItems(chunks)
	require.Error(t, err)
}

func TestExptTurnResultRunLogFreezesItemVersion(t *testing.T) {
	result := &ExptTurnResult{ID: 1, ItemID: 2, ItemVersionID: 1234567890123456789, TurnID: 3}
	require.Equal(t, int64(1234567890123456789), result.ToRunLogDO().ItemVersionID)
}
