// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"testing"

	redismocks "github.com/coze-dev/coze-loop/backend/infra/redis/mocks"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEvalAsyncDAO_GetEvalAsyncCtxStrongReadsThroughMasterRoutedScript(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	cmdable := redismocks.NewMockPersistentCmdable(ctrl)
	dao := NewEvalAsyncDAO(cmdable)
	key := "[experiment]item_turn_eval_async_ctx:evaluator:1"
	payload := `{"RecordID":1,"resume_ready":true}`

	cmdable.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{key}).DoAndReturn(
		func(ctx context.Context, _ string, _ []string, _ ...any) *redis.Cmd {
			return redis.NewCmdResult(payload, nil)
		},
	)

	got, err := dao.GetEvalAsyncCtxStrong(context.Background(), "evaluator:1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.ResumeReady)
}

func TestEvalAsyncDAO_GetEvalAsyncCtxStrongRetriesPrimaryScriptOnMissing(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	cmdable := redismocks.NewMockPersistentCmdable(ctrl)
	dao := NewEvalAsyncDAO(cmdable)
	key := "[experiment]item_turn_eval_async_ctx:evaluator:2"
	payload := `{"RecordID":2,"resume_barrier_enabled":true,"resume_ready":true}`
	calls := 0
	cmdable.EXPECT().Eval(gomock.Any(), gomock.Any(), []string{key}).DoAndReturn(
		func(ctx context.Context, _ string, _ []string, _ ...any) *redis.Cmd {
			calls++
			if calls < 3 {
				return redis.NewCmdResult(nil, redis.Nil)
			}
			return redis.NewCmdResult(payload, nil)
		},
	).Times(3)

	got, err := dao.GetEvalAsyncCtxStrong(context.Background(), "evaluator:2")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 3, calls)
	assert.True(t, got.CanResumeExperiment())
}
