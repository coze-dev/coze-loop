// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package errno

import (
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestWithExtra(t *testing.T) {
	err := errorx.NewByCode(CommonInternalErrorCode)

	statusErr, ok := errorx.FromStatusError(err)
	assert.True(t, ok)

	statusErr.WithAffectStability(false)
	bizErr, ok := kerrors.FromBizStatusError(statusErr)
	assert.True(t, ok)
	assert.Equal(t, "0", bizErr.BizExtra()["biz_err_affect_stability"])

	statusErr.WithAffectStability(true)
	bizErr, ok = kerrors.FromBizStatusError(statusErr)
	assert.True(t, ok)
	assert.Equal(t, "1", bizErr.BizExtra()["biz_err_affect_stability"])
}
