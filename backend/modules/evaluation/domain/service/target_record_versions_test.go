// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSandboxVersionsUseEachResourceSource(t *testing.T) {
	controller := gomock.NewController(t)
	store := repomocks.NewMockIEvalTargetRepo(controller)
	service := &EvalTargetServiceImpl{evalTargetRepo: store}
	first := &entity.EvalTargetRecord{ID: 1, TargetVersionID: 101, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)}
	first.BindExecutionSpace(10, 99)
	second := &entity.EvalTargetRecord{ID: 2, TargetVersionID: 202, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)}
	second.BindExecutionSpace(20, 99)
	completed := &entity.EvalTargetRecord{ID: 3, TargetVersionID: 303, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}
	completed.BindExecutionSpace(30, 99)
	store.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), int64(10), []int64{101}).Return([]*entity.EvalTarget{{EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 101}}}, nil)
	store.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), int64(20), []int64{202}).Return([]*entity.EvalTarget{{EvalTargetType: entity.EvalTargetTypeCozeBot, EvalTargetVersion: &entity.EvalTargetVersion{ID: 202}}}, nil)
	versions, err := service.sandboxVersionIDsForRecords(context.Background(), []*entity.EvalTargetRecord{first, first, second, completed, nil})
	require.NoError(t, err)
	require.Equal(t, map[int64]struct{}{101: {}}, versions)
}

func TestSandboxVersionsRejectInvalidPersistedSource(t *testing.T) {
	controller := gomock.NewController(t)
	store := repomocks.NewMockIEvalTargetRepo(controller)
	service := &EvalTargetServiceImpl{evalTargetRepo: store}
	record := &entity.EvalTargetRecord{ID: 1, TargetVersionID: 101, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)}
	record.BindExecutionSpace(0, 99)
	versions, err := service.sandboxVersionIDsForRecords(context.Background(), []*entity.EvalTargetRecord{record})
	require.Error(t, err)
	require.Nil(t, versions)
}

func TestSandboxVersionsPropagateSourceReadFailure(t *testing.T) {
	controller := gomock.NewController(t)
	store := repomocks.NewMockIEvalTargetRepo(controller)
	service := &EvalTargetServiceImpl{evalTargetRepo: store}
	record := &entity.EvalTargetRecord{ID: 1, TargetVersionID: 101, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)}
	record.BindExecutionSpace(10, 99)
	failure := errors.New("source unavailable")
	store.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), int64(10), []int64{101}).Return(nil, failure)
	versions, err := service.sandboxVersionIDsForRecords(context.Background(), []*entity.EvalTargetRecord{record})
	require.ErrorIs(t, err, failure)
	require.Nil(t, versions)
}
