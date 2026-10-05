// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestHookFinalizationManagerRejectsExplicitZeroRun(t *testing.T) {
	for _, method := range []string{"CompleteRun", "CompleteExpt"} {
		t.Run(method, func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			ctx := context.Background()
			var err error
			if method == "CompleteRun" {
				err = f.manager.CompleteRun(ctx, f.expt, 0, f.space, nil)
			} else {
				err = f.manager.CompleteExpt(ctx, f.expt, gptr.Of(int64(0)), f.space, nil)
			}
			require.Error(t, err, "only a nil CompleteExpt RunID may resolve Latest")
			require.Equal(t, entity.HookFinalizeNone, finalizationRead(t, f).State.Finalize)
		})
	}
}

func TestHookFinalizationConstructorDoesNotMutateExistingManager(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	require.Nil(t, f.base.(*ExptMangerImpl).finalization)
	require.Nil(t, f.manager.hooks)
	_, err := NewExptManagerWithHookFinalization(f.manager, f.deps)
	require.Error(t, err)
	bad := f.deps
	bad.Repository = nil
	_, err = NewExptManagerWithHookFinalization(f.base, bad)
	require.Error(t, err)
}
