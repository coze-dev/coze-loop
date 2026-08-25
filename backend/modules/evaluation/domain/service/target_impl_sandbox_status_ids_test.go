// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	trajectorymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// sandboxStatusIDsFixture 组一条 SandboxAgent + AsyncInvoking 的 record（带指定 output ext），
// 并把 Get 收到的 ExecuteID 逐个记下来，供用例断言"到底问了哪些 execution"。
//
// 返回的 got 是并发写入的（CheckSandboxTerminated 内部并发查），故加锁；调用方在
// CheckSandboxTerminated 返回后（wg.Wait 之后）读取即安全。
func sandboxStatusIDsFixture(
	t *testing.T, ctrl *gomock.Controller, ext map[string]string,
	statusOf func(executeID string) rpc.SandboxExecuteStatus,
) (*EvalTargetServiceImpl, *[]string) {
	t.Helper()
	mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
	mockSched := trajectorymocks.NewMockISandboxSchedulerAdapter(ctrl)
	svc := &EvalTargetServiceImpl{sandboxSchedulerAdapter: mockSched, evalTargetRepo: mockRepo}

	record := &entity.EvalTargetRecord{
		ID: 7590116637119703042, TargetVersionID: 20, SpaceID: 1,
		Status:               gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
		EvalTargetOutputData: &entity.EvalTargetOutputData{Ext: ext},
	}
	mockRepo.EXPECT().ListEvalTargetRecordByIDsAndSpaceID(gomock.Any(), int64(1), gomock.Any()).
		Return([]*entity.EvalTargetRecord{record}, nil)
	mockRepo.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), int64(1), gomock.Any()).
		Return([]*entity.EvalTarget{{
			EvalTargetType:    entity.EvalTargetTypeSandboxAgent,
			EvalTargetVersion: &entity.EvalTargetVersion{ID: 20, EvalTargetType: entity.EvalTargetTypeSandboxAgent},
		}}, nil)

	var mu sync.Mutex
	asked := make([]string, 0, 2)
	mockSched.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(_ context.Context, req *rpc.SandboxGetRequest) (*rpc.SandboxGetResponse, error) {
			mu.Lock()
			asked = append(asked, req.ExecuteID)
			mu.Unlock()
			return &rpc.SandboxGetResponse{
				ExecuteInfo: &rpc.SandboxExecuteInfo{Status: statusOf(req.ExecuteID)},
			}, nil
		})
	return svc, &asked
}

// 双沙箱"完整列表"形态（ext 只有 sandbox_execute_ids、没有 extra key）必须被巡检看见。
//
// 这是线上真实泄漏形态（execute_id 形如 <invokeID>-agent / -orch）：编排全程成功、无 panic 无
// 报错，但沙箱一直挂着。原实现把主沙箱硬编码成裸 record.ID 去 Get，沙箱侧只回
// "execution not found"（按契约返回 (false,_)），而 extra key 不存在导致从沙箱查询整段跳过 ——
// 于是这条 record 永远判不出终态，拿不到本巡检的快速兜底，只能等 3h item zombie。
//
// 钉两件事：① 问的必须是 ext 里的真实 id，裸 record.ID 绝不出现；② 任一 id 终态即命中。
func TestCheckSandboxTerminated_DualSandboxListExt_QueriesRealIDs(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		bareID  = "7590116637119703042"
		agentID = bareID + "-agent"
		orchID  = bareID + "-orch"
	)

	svc, asked := sandboxStatusIDsFixture(t, ctrl,
		map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: `["` + agentID + `","` + orchID + `"]`},
		func(executeID string) rpc.SandboxExecuteStatus {
			// agent 侧已 Failed，orch 侧还在跑：单边终态就该命中。
			if executeID == agentID {
				return rpc.SandboxExecuteStatusFailed
			}
			return rpc.SandboxExecuteStatusRunning
		})

	got, statuses := svc.CheckSandboxTerminated(context.Background(), 1, []int64{7590116637119703042})

	assert.ElementsMatch(t, []string{agentID, orchID}, *asked,
		"必须按 ext 回传的真实 execute id 查询；裸 record.ID 在沙箱侧不存在（execution not found）")
	assert.NotContains(t, *asked, bareID, "不得按单沙箱约定推断出裸 record.ID")
	assert.Equal(t, []int64{7590116637119703042}, got, "agent 侧已 Failed，必须判为终态命中")
	assert.Equal(t, "Failed (agent)", statuses[7590116637119703042])
}

// 从沙箱（列表里的非首个）单独进终态也必须命中，标签按 executeID 后缀落到对应角色（orch→orchestrator）。
func TestCheckSandboxTerminated_DualSandboxListExt_SubordinateTerminal(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		bareID  = "7590116637119703042"
		agentID = bareID + "-agent"
		orchID  = bareID + "-orch"
	)

	svc, asked := sandboxStatusIDsFixture(t, ctrl,
		map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: `["` + agentID + `","` + orchID + `"]`},
		func(executeID string) rpc.SandboxExecuteStatus {
			if executeID == orchID {
				return rpc.SandboxExecuteStatusCanceled
			}
			return rpc.SandboxExecuteStatusRunning
		})

	got, statuses := svc.CheckSandboxTerminated(context.Background(), 1, []int64{7590116637119703042})

	assert.ElementsMatch(t, []string{agentID, orchID}, *asked)
	assert.Equal(t, []int64{7590116637119703042}, got, "orchestrator 侧已 Canceled，必须判为终态命中")
	assert.Equal(t, "Canceled (orchestrator)", statuses[7590116637119703042])
}

// 单沙箱（ext 两个 key 都缺）必须保持原行为：仍按裸 record.ID 查一次，不多查也不少查。
// 这条钉的是"改动不回归单沙箱"——sandboxExecuteIDsOf 在 ext 缺省时正是退回 record.ID。
func TestCheckSandboxTerminated_SingleSandboxNoExt_KeepsBareRecordID(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, asked := sandboxStatusIDsFixture(t, ctrl, nil,
		func(string) rpc.SandboxExecuteStatus { return rpc.SandboxExecuteStatusFailed })

	got, statuses := svc.CheckSandboxTerminated(context.Background(), 1, []int64{7590116637119703042})

	assert.Equal(t, []string{"7590116637119703042"}, *asked,
		"单沙箱 ext 缺省时仍按 record.ID 查询，行为不变")
	assert.Equal(t, []int64{7590116637119703042}, got)
	assert.Equal(t, "Failed (main)", statuses[7590116637119703042])
}
