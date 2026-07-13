// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
)

// UT-EVAL-01(domain service 层): CountEvaluationSets 将 spaceID 透传给 adapter.CountDatasets，
// 回传 total；adapter err 透传。
func TestEvaluationSetServiceImpl_CountEvaluationSets(t *testing.T) {
	t.Run("spaceID 透传 + total 回传", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
		svc := &EvaluationSetServiceImpl{datasetRPCAdapter: mockAdapter}

		mockAdapter.EXPECT().CountDatasets(gomock.Any(), int64(100)).Return(int64(9), nil)

		total, err := svc.CountEvaluationSets(context.Background(), 100)
		assert.NoError(t, err)
		assert.Equal(t, int64(9), total)
	})

	t.Run("adapter error 透传", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
		svc := &EvaluationSetServiceImpl{datasetRPCAdapter: mockAdapter}

		adapterErr := errors.New("downstream count err")
		mockAdapter.EXPECT().CountDatasets(gomock.Any(), int64(100)).Return(int64(0), adapterErr)

		total, err := svc.CountEvaluationSets(context.Background(), 100)
		assert.Error(t, err)
		assert.Equal(t, adapterErr, err)
		assert.Equal(t, int64(0), total)
	})
}
