// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestGetAnalysisRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("成功", func(t *testing.T) {
		res, err := NewEvaluationAnalysisService().GetAnalysisRecord(context.Background(), 1, 2)
		assert.NoError(t, err)
		assert.Nil(t, res)
	})
}
