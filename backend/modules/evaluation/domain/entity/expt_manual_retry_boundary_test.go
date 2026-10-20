// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"strings"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/require"
)

func TestManualRetryCapacityBizBoundary(t *testing.T) {
	retry := &ManualRetryContext{RunID: 8, RequestID: "request", Mode: EvaluationModeRetryItems}
	_, extErr := MergeManualRetryContext([]byte(`{"padding":"`+strings.Repeat("x", 65500)+`"}`), retry)
	_, receiptErr := EncodeExptRunLogItems([]ExptRunLogItems{{ManualRetryReceipt: &ManualRetryReceipt{RequestID: strings.Repeat("x", 65536)}}})
	for _, err := range []error{extErr, receiptErr} {
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
