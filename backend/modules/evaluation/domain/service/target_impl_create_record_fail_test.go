// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	trajectorymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// createRecordFailFixture 组一个 asyncExecuteTarget 会走到 CreateEvalTargetRecord 失败的场景：
// operator 已成功返回（沙箱确定在跑）、ext 已算好，但落库失败。
//
// targetType 决定类型守卫走哪条分支；ext 是 operator 回传、落在 outputData.Ext 的值。
// 返回的 destroyed 记录 Destroy 实际收到的请求（可能来自异步 goroutine，故加锁）。
func createRecordFailFixture(
	t *testing.T, ctrl *gomock.Controller,
	targetType entity.EvalTargetType, ext map[string]string,
) (*EvalTargetServiceImpl, *entity.EvalTarget, *entity.EvalTargetInputData, *[]*rpc.SandboxDestroyRequest, *sync.Mutex, chan struct{}) {
	t.Helper()

	repo := repomocks.NewMockIEvalTargetRepo(ctrl)
	metric := metricsmocks.NewMockEvalTargetMetrics(ctrl)
	operator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)
	configer := componentmocks.NewMockIConfiger(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	sched := trajectorymocks.NewMockISandboxSchedulerAdapter(ctrl)

	configer.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	metric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	target := &entity.EvalTarget{
		ID:             1,
		SpaceID:        1,
		SourceTargetID: "source",
		EvalTargetType: targetType,
		EvalTargetVersion: &entity.EvalTargetVersion{
			ID:                  2,
			SourceTargetVersion: "v1",
			InputSchema:         []*entity.ArgsSchema{{Key: gptr.Of("a")}},
		},
	}
	input := &entity.EvalTargetInputData{
		InputFields: map[string]*entity.Content{"a": {ContentType: gptr.Of(entity.ContentTypeText)}},
	}

	operator.EXPECT().ValidateInput(gomock.Any(), target.SpaceID, target.EvalTargetVersion.InputSchema, input).Return(nil)
	// operator 成功返回：invokeID=999，ext 由 operator 回传。
	operator.EXPECT().AsyncExecute(gomock.Any(), target.SpaceID, gomock.Any()).
		Return(int64(999), "callee", ext, nil)
	// emitTargetTrace 侧会回查 version（与既有 "success" 用例同款），与本用例断言无关。
	repo.EXPECT().GetEvalTargetVersion(gomock.Any(), target.SpaceID, target.EvalTargetVersion.ID).
		AnyTimes().Return(target, nil)
	// 落库失败 —— 本用例的全部前提。
	repo.EXPECT().CreateEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(0), errors.New("db write fail"))

	var mu sync.Mutex
	got := make([]*rpc.SandboxDestroyRequest, 0, 1)
	done := make(chan struct{}, 1)
	sched.EXPECT().Destroy(gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(_ context.Context, req *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
			mu.Lock()
			got = append(got, req)
			mu.Unlock()
			select {
			case done <- struct{}{}:
			default:
			}
			return &rpc.SandboxDestroyResponse{}, nil
		})

	svc := &EvalTargetServiceImpl{
		evalTargetRepo:          repo,
		idgen:                   idgen,
		metric:                  metric,
		configer:                configer,
		sandboxSchedulerAdapter: sched,
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			targetType: operator,
		},
	}
	return svc, target, input, &got, &mu, done
}

// 落库失败必须就地销毁沙箱：此刻是唯一清理机会 —— operator 已成功返回、沙箱确定在跑，
// 但 record 没落库，而平台侧三条兜底销毁链全部以"按 recordID 从库里查出 record 再读 ext"
// 为前提，库里没这行就一条都命中不到，沙箱只能等 patrol、并发名额一直不归还。
//
// 钉两件事：① Destroy 必须真的被调；② executeIDs 必须与 operator 回传的 ext 一致
// （证明用的是内存里那份 record，而不是回头查库拿到空值）。
func TestAsyncExecuteTarget_CreateRecordFail_DestroysSandbox(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const exptID = int64(7788)
	ext := map[string]string{
		consts.OutputDataExtKeySandboxExecuteIDs: `["999-agent","999-orch"]`,
	}
	svc, target, input, got, mu, done := createRecordFailFixture(
		t, ctrl, entity.EvalTargetTypeSandboxAgent, ext)

	record, _, err := svc.asyncExecuteTarget(context.Background(), target.SpaceID, target,
		&entity.ExecuteTargetCtx{ItemID: 1, TurnID: 2, ExperimentID: gptr.Of(exptID)}, input)

	require.Error(t, err, "落库失败必须照常返回错误，销毁不改变返回语义")
	assert.Nil(t, record)

	// destroySandboxExecute 内部异步，等它真正发出请求。
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("落库失败后没有销毁沙箱：三条兜底链都以库里有 record 为前提，这里不销就是确定性泄漏")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, *got, 1, "必须销毁且只销毁一次")
	req := (*got)[0]
	assert.Equal(t, []string{"999-agent", "999-orch"}, req.ExecuteIDs,
		"execute id 必须来自 operator 回传的 ext（内存里那份 record），不能退化成裸 invokeID")
	assert.Equal(t, strconv.FormatInt(exptID, 10), req.TaskID,
		"taskID 直接用手上的 ExperimentID，与 operator 侧一致；不查 expt_run_log")
	assert.Equal(t, target.SpaceID, req.WorkspaceID)
	assert.False(t, req.ZombieTimeout, "这不是 zombie 超时路径")
}

// 反向用例：非 SandboxAgent 类型落库失败**不得**销毁 —— destroySandboxExecute 自己不检查
// target 类型，少了这道守卫就会给非沙箱场景平白发一次 Destroy（过度杀伤）。
func TestAsyncExecuteTarget_CreateRecordFail_NonSandboxTypeNoDestroy(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, target, input, got, mu, done := createRecordFailFixture(
		t, ctrl, entity.EvalTargetTypeCustomRPCServer, nil)

	record, _, err := svc.asyncExecuteTarget(context.Background(), target.SpaceID, target,
		&entity.ExecuteTargetCtx{ItemID: 1, TurnID: 2, ExperimentID: gptr.Of(int64(7788))}, input)

	require.Error(t, err)
	assert.Nil(t, record)

	// ⚠️ 必须等一个窗口再断言"没销毁"：destroySandboxExecute 内部是 goroutine.Go 异步发的，
	// 落库失败后立刻读 got 必然是空的 —— 那样即使把类型守卫整个删掉本用例也照样绿
	// （已用 mutation 验证过这一点）。等到超时才能证明"确实一次都没发"。
	select {
	case <-done:
		t.Fatal("非 SandboxAgent 类型不该销毁任何沙箱：destroySandboxExecute 自己不检查 target 类型，守卫丢了就是过度杀伤")
	case <-time.After(2 * time.Second):
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Empty(t, *got, "非 SandboxAgent 类型不该销毁任何沙箱")
}
