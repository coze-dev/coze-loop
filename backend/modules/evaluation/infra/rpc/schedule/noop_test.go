// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package schedule

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
)

func TestNoopExptScheduleAdapter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := NewNoopExptScheduleAdapter()

	t.Run("CreatePeriodicJob returns nil", func(t *testing.T) {
		err := adapter.CreatePeriodicJob(ctx, &rpc.CreatePeriodicJobParam{
			BizKey: "test_key",
		})
		assert.NoError(t, err)
	})

	t.Run("CloseJob returns nil", func(t *testing.T) {
		err := adapter.CloseJob(ctx, "test_key")
		assert.NoError(t, err)
	})

	t.Run("GetJob returns nil, nil", func(t *testing.T) {
		job, err := adapter.GetJob(ctx, "test_key")
		assert.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("implements IExptScheduleAdapter interface", func(t *testing.T) {
		var _ rpc.IExptScheduleAdapter = adapter
	})
}
