// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	lwtMocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestWindowsRunReachesQuota(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(map[bool]string{false: "run", true: "retry"}[retry], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mgr := newTestExptManager(ctrl)
			expt := &entity.Experiment{ID: 1, SpaceID: 2, TargetID: 3, TargetVersionID: 4, Target: &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{
				SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountMode("windows_plus_sandbox")},
			}}}
			mgr.lwt.(*lwtMocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(false).AnyTimes()
			mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(2)).Return([]*entity.Experiment{expt}, nil).AnyTimes()
			mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil()).Return(&entity.EvaluationSet{}, nil).AnyTimes()
			mgr.evalTargetService.(*svcMocks.MockIEvalTargetService).EXPECT().GetEvalTargetVersion(gomock.Any(), int64(2), int64(4), true).Return(expt.Target, nil)
			mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptStats{}, nil).AnyTimes()
			mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()
			mgr.userProvider.(*rpcMocks.MockIUserProvider).EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*entity.UserInfo{}, nil).AnyTimes()
			mgr.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(2), gomock.Any(), gomock.Any()).Return(errors.New("quota reached"))
			var err error
			if retry {
				err = mgr.RetryItems(context.Background(), 1, 0, 2, 0, nil, &entity.Session{}, nil)
			} else {
				err = mgr.Run(context.Background(), 1, 0, 2, 0, &entity.Session{}, entity.EvaluationModeFailRetry, nil)
			}
			assert.ErrorContains(t, err, "quota reached")
		})
	}
}
