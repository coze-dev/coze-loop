// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExptItemTurnEvalAsyncCtx_CallbackURLRoundTrip(t *testing.T) {
	c := NewExptItemTurnEvalAsyncCtx()
	in := &entity.EvalAsyncCtx{
		RecordID:           123,
		EvaluatorVersionID: 456,
		CallbackURL:        "https://example.com/hook",
	}
	b, err := c.FromDO(in)
	assert.NoError(t, err)

	out, err := c.ToDO(b)
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/hook", out.CallbackURL)
	assert.Equal(t, int64(123), out.RecordID)
}

func TestExptItemTurnEvalAsyncCtx_ResumeReadyRoundTrip(t *testing.T) {
	c := NewExptItemTurnEvalAsyncCtx()
	in := &entity.EvalAsyncCtx{RecordID: 123, ResumeBarrierEnabled: true, ResumeReady: true}
	b, err := c.FromDO(in)
	assert.NoError(t, err)

	out, err := c.ToDO(b)
	assert.NoError(t, err)
	assert.True(t, out.ResumeBarrierEnabled)
	assert.True(t, out.ResumeReady)
}

func TestExptItemTurnEvalAsyncCtx_LegacyPayloadDefaultsToResumeAllowed(t *testing.T) {
	c := NewExptItemTurnEvalAsyncCtx()
	out, err := c.ToDO([]byte(`{"RecordID":123}`))
	require.NoError(t, err)
	assert.True(t, out.CanResumeExperiment())
}

func TestExptItemTurnEvalAsyncCtx_NewBarrierRequiresReady(t *testing.T) {
	c := NewExptItemTurnEvalAsyncCtx()
	out, err := c.ToDO([]byte(`{"resume_barrier_enabled":true}`))
	require.NoError(t, err)
	assert.False(t, out.CanResumeExperiment())

	out, err = c.ToDO([]byte(`{"resume_barrier_enabled":true,"resume_ready":true}`))
	require.NoError(t, err)
	assert.True(t, out.CanResumeExperiment())
}
