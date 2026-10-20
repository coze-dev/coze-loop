// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/require"
)

func TestManualRetryCapacityBizBoundary(t *testing.T) {
	retry := &ManualRetryContext{RunID: 8, RequestID: "request", Mode: EvaluationModeRetryItems}
	_, extErr := MergeManualRetryContext([]byte(`{"padding":"`+strings.Repeat("x", 65500)+`"}`), retry)
	for _, err := range []error{extErr} {
		require.Error(t, err)
		biz, ok := kerrors.FromBizStatusError(err)
		require.True(t, ok, "capacity must reach PacketAdapter as BizStatus")
		require.Equal(t, int32(601204104), biz.BizStatusCode())
	}
	_, err := MergeManualRetryContext([]byte(`{"manual_retry":"broken"}`), retry)
	require.Error(t, err)
	_, ok := kerrors.FromBizStatusError(err)
	require.False(t, ok, "corrupt stored source is not a business refusal")
}

func TestRunLogCapacityIsOrdinaryError(t *testing.T) {
	ids := make([]int64, 4000)
	for i := range ids {
		ids[i] = 9223372036854700000 + int64(i)
	}
	_, err := EncodeExptRunLogItems([]ExptRunLogItems{{ItemIDs: ids}})
	require.ErrorIs(t, err, ErrExptRunLogItemsCapacity)
	_, ok := kerrors.FromBizStatusError(err)
	require.False(t, ok, "ordinary run encoding must retain the internal error category")
}

func TestRunLogEncodingCauseIsOrdinaryError(t *testing.T) {
	_, err := EncodeExptRunLogItems([]ExptRunLogItems{{unknownFields: map[string]json.RawMessage{"future": json.RawMessage(`{`)}}})
	require.Error(t, err)
	var cause *json.MarshalerError
	require.ErrorAs(t, err, &cause)
	require.NotErrorIs(t, err, ErrExptRunLogItemsCapacity)
	_, ok := kerrors.FromBizStatusError(err)
	require.False(t, ok)
}
