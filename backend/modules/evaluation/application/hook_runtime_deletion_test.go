// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHookBatchDeletionFactoryPureRejectsUnconfigured(t *testing.T) {
	var factory *HookRuntimeExecutionFactory
	_, err := factory.ForDeletion(context.Background(), []int64{20}, 1)
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
	factory = &HookRuntimeExecutionFactory{}
	_, err = factory.ForDeletion(context.Background(), []int64{20}, 1)
	require.Error(t, err)
}
