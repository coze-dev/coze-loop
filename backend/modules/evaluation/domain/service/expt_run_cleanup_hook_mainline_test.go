// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestMainlineCleanupKeepsHookOwnedRun(t *testing.T) {
	m := newTestExptManager(gomock.NewController(t))
	m.hooks = &ExptManagerHookDependencies{}
	// Hook initialization/finalization owns compare-release; legacy cleanup must neither rewrite nor DEL it.
	// No RunLog.Update or UnlockForce expectation: either side effect fails this test.
	m.cleanupUnscheduledRun(context.Background(), 20, 30, errors.New("admission rejected"))
}
