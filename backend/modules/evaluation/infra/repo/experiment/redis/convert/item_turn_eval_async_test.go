// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
