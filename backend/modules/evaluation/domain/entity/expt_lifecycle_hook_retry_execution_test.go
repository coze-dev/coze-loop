// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHookRetryExecutionSnapshotModes(t *testing.T) {
	for _, tc := range []struct {
		mode ExptRunMode
		wire spi.HookRunMode
	}{{EvaluationModeFailRetry, spi.HookRunModeFailRetry}, {EvaluationModeRetryAll, spi.HookRunModeRetryAll}} {
		t.Run(string(tc.wire), func(t *testing.T) {
			in := executionSnapshotInput()
			in.Execution.Mode = tc.mode
			in.Context.RunMode = gptr.Of(tc.wire)
			snapshot, err := NewHookRunSnapshot(in)
			require.NoError(t, err)
			require.Equal(t, tc.mode, snapshot.Input().Execution.Mode)
		})
	}
}
