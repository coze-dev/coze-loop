// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

type processingCASPort interface {
	MarkProcessingIfCurrent(context.Context, int64, int64, int64, int64, string) (bool, error)
}

func TestCurrentRequestMySQLProcessingCAS(t *testing.T) {
	f := newManualRetryFixture(t)
	_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	dao := NewExptItemResultDAO(f.provider)
	port, ok := any(dao).(processingCASPort)
	require.True(t, ok, "current request CAS transaction is missing")
	before := f.snapshot()
	changed, err := port.MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 11, "rq1")
	require.NoError(t, err)
	require.True(t, changed)
	after := f.snapshot()
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_stats"} {
		require.NotEqual(t, before[table], after[table], table)
	}
	changed, err = port.MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 11, "rq1")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, after, f.snapshot())
}
