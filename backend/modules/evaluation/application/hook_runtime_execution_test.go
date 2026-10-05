// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookRuntimeExecutionFactoryMissingDependencies(t *testing.T) {
	factory, err := NewHookRuntimeExecutionFactory(HookRuntimeExecutionFactoryDependencies{Scope: "local"})
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
	require.Nil(t, factory)
	_, err = factory.ForRun(context.Background(), entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3})
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
}
