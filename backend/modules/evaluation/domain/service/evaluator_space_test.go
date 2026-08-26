// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// TestEvaluatorServiceImpl_AsyncRunEvaluator_CrossSpace 钉住跨空间调用的承重不变量:
// 记录落资源空间、传给 provider 的空间也是资源空间(两者必须一致, 否则沙箱回写的
// workspace_id 与记录空间不符, 记录会永久停在 AsyncInvoking), 且两个空间各限流一次。
func TestEvaluatorServiceImpl_AsyncRunEvaluator_CrossSpace(t *testing.T) {
	const callerSpaceID, resourceSpaceID = int64(200), int64(100)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	limiter := repomocks.NewMockRateLimiter(ctrl)
	plainLimiter := repomocks.NewMockIPlainRateLimiter(ctrl)
	idGen := idgenmocks.NewMockIIDGenerator(ctrl)
	recordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
	sourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	cConfiger := componentMocks.NewMockIConfiger(ctrl)
	cConfiger.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	s := &EvaluatorServiceImpl{
		evaluatorRepo:       evaluatorRepo,
		limiter:             limiter,
		idgen:               idGen,
		evaluatorRecordRepo: recordRepo,
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypeAgent: sourceService,
		},
		plainRateLimiter: plainLimiter,
		evalAsyncRepo:    asyncRepo,
		cConfiger:        cConfiger,
	}

	evaluatorDO := &entity.Evaluator{
		ID:            300,
		SpaceID:       resourceSpaceID,
		EvaluatorType: entity.EvaluatorTypeAgent,
		AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
			ID:          301,
			EvaluatorID: 300,
			SpaceID:     resourceSpaceID,
		},
	}
	evaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Nil(), []int64{301}, false, false).
		Return([]*entity.Evaluator{evaluatorDO}, nil)
	limiter.EXPECT().AllowInvoke(gomock.Any(), callerSpaceID).Return(true)
	limiter.EXPECT().AllowInvoke(gomock.Any(), resourceSpaceID).Return(true)
	plainLimiter.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
	idGen.EXPECT().GenID(gomock.Any()).Return(int64(999), nil)
	asyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), "evaluator:999", gomock.Any()).Return(nil)

	var persisted *entity.EvaluatorRecord
	recordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record *entity.EvaluatorRecord) error {
			persisted = record
			return nil
		})
	sourceService.EXPECT().AsyncRun(gomock.Any(), evaluatorDO, gomock.Any(), gomock.Any(), resourceSpaceID, int64(999)).
		Return(nil, "", nil)

	got, err := s.AsyncRunEvaluator(context.Background(), &entity.AsyncRunEvaluatorRequest{
		SpaceID:            callerSpaceID,
		EvaluatorVersionID: 301,
		InputData:          &entity.EvaluatorInputData{},
		Ext:                map[string]string{"k": "v"},
		SharedOption:       &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(resourceSpaceID)},
	})

	assert.NoError(t, err)
	assert.Equal(t, resourceSpaceID, got.SpaceID)
	assert.Equal(t, "200", got.Ext[consts.EvaluatorRecordExtKeyCallerSpaceID])
	assert.NotNil(t, persisted)
	assert.Equal(t, resourceSpaceID, persisted.SpaceID)
	assert.Equal(t, "200", persisted.Ext[consts.EvaluatorRecordExtKeyCallerSpaceID])
}

func TestResolveEvaluatorSpaceID(t *testing.T) {
	const callerSpaceID, resourceSpaceID = int64(200), int64(100)

	t.Run("非 builtin 用评估器自己的空间", func(t *testing.T) {
		got := resolveEvaluatorSpaceID(&entity.Evaluator{SpaceID: resourceSpaceID}, callerSpaceID)
		assert.Equal(t, resourceSpaceID, got)
	})
	t.Run("builtin 用调用方空间", func(t *testing.T) {
		got := resolveEvaluatorSpaceID(&entity.Evaluator{SpaceID: resourceSpaceID, Builtin: true}, callerSpaceID)
		assert.Equal(t, callerSpaceID, got)
	})
	t.Run("同空间取值不变", func(t *testing.T) {
		got := resolveEvaluatorSpaceID(&entity.Evaluator{SpaceID: callerSpaceID}, callerSpaceID)
		assert.Equal(t, callerSpaceID, got)
	})
	t.Run("evaluator 为 nil 回落调用方空间", func(t *testing.T) {
		assert.Equal(t, callerSpaceID, resolveEvaluatorSpaceID(nil, callerSpaceID))
	})
}

func TestCheckEvaluatorSpaceAccess(t *testing.T) {
	const callerSpaceID, sourceSpaceID, otherSpaceID = int64(200), int64(100), int64(300)

	t.Run("未声明共享且同空间放行", func(t *testing.T) {
		assert.NoError(t, checkEvaluatorSpaceAccess(&entity.Evaluator{SpaceID: callerSpaceID}, callerSpaceID, nil))
	})
	t.Run("未声明共享且跨空间拒绝", func(t *testing.T) {
		assert.Error(t, checkEvaluatorSpaceAccess(&entity.Evaluator{SpaceID: sourceSpaceID}, callerSpaceID, nil))
	})
	t.Run("声明共享且与真实空间一致放行", func(t *testing.T) {
		opt := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(sourceSpaceID)}
		assert.NoError(t, checkEvaluatorSpaceAccess(&entity.Evaluator{SpaceID: sourceSpaceID}, callerSpaceID, opt))
	})
	t.Run("声明的来源空间与真实空间不一致拒绝", func(t *testing.T) {
		opt := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(sourceSpaceID)}
		assert.Error(t, checkEvaluatorSpaceAccess(&entity.Evaluator{SpaceID: otherSpaceID}, callerSpaceID, opt))
	})
	t.Run("builtin 不受空间约束", func(t *testing.T) {
		assert.NoError(t, checkEvaluatorSpaceAccess(&entity.Evaluator{SpaceID: sourceSpaceID, Builtin: true}, callerSpaceID, nil))
	})
}

func TestWithCallerSpaceExt(t *testing.T) {
	const callerSpaceID, resourceSpaceID = int64(200), int64(100)

	t.Run("同空间不写入且不复制", func(t *testing.T) {
		src := map[string]string{"k": "v"}
		got := withCallerSpaceExt(src, callerSpaceID, callerSpaceID)
		assert.Equal(t, src, got)
		assert.NotContains(t, got, consts.EvaluatorRecordExtKeyCallerSpaceID)
	})
	t.Run("跨空间写入调用方空间且不改动入参", func(t *testing.T) {
		src := map[string]string{"k": "v"}
		got := withCallerSpaceExt(src, callerSpaceID, resourceSpaceID)
		assert.Equal(t, "200", got[consts.EvaluatorRecordExtKeyCallerSpaceID])
		assert.Equal(t, "v", got["k"])
		assert.NotContains(t, src, consts.EvaluatorRecordExtKeyCallerSpaceID)
	})
	t.Run("入参为 nil 时也能写入", func(t *testing.T) {
		got := withCallerSpaceExt(nil, callerSpaceID, resourceSpaceID)
		assert.Equal(t, "200", got[consts.EvaluatorRecordExtKeyCallerSpaceID])
	})
}

func TestMapAuthEntityType_Evaluator(t *testing.T) {
	got, err := mapAuthEntityType(entity.SharedResourceTypeEvaluator)
	assert.NoError(t, err)
	assert.EqualValues(t, "Evaluator", got)

	_, err = mapAuthEntityType("unknown_resource")
	assert.Error(t, err)
}
