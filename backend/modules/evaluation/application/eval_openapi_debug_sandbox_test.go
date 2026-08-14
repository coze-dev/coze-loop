// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// debugRecordWithExecuteIDs 造一条带 sandbox_execute_ids ext 的 record。
func debugRecordWithExecuteIDs(raw string) *entity.EvalTargetRecord {
	return &entity.EvalTargetRecord{
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			Ext: map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: raw},
		},
	}
}

// 调试态销毁必须用 operator 真实回传的 execute id 列表, **不能推断**。
//
// 双沙箱的真实 id 带后缀 (`<invokeID>-agent` / `-orch`)。原实现硬编码裸 invokeID, 对不上任何一个
// execution → 两个沙箱一个都清不掉。而调试态又走不到 service 层的 destroySandboxExecuteIfNeeded
// (那里先按 record.TargetVersionID 查 version 判类型, 调试用的 patchy target 从未落库、查不到),
// 所以本函数是调试态唯一的清理来源。
func TestDebugSandboxExecuteIDs_ReadsDualIDsFromExt(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(1001), int64(3003)).
		Return(debugRecordWithExecuteIDs(`["3003-agent","3003-orch"]`), nil)

	app := &EvalOpenAPIApplication{targetSvc: targetSvc}
	ids := app.debugSandboxExecuteIDs(context.Background(), 1001, 3003)

	assert.Equal(t, []string{"3003-agent", "3003-orch"}, ids,
		"必须销毁两个真实 id, 而不是裸 invokeID")
	assert.NotContains(t, ids, "3003", "裸 invokeID 对不上任何 execution, 不该出现")
}

// 各种读不到 ext 的情形一律退回裸 invokeID —— 那是单沙箱的 executeID, 也是本改动前的唯一行为,
// 所以退化路径与改动前完全一致, 不会因为读 ext 失败而比原来更差。
func TestDebugSandboxExecuteIDs_FallsBackToInvokeID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		record *entity.EvalTargetRecord
		err    error
	}{
		{"查 record 报错", nil, errors.New("db down")},
		{"record 不存在", nil, nil},
		{"没有 output data", &entity.EvalTargetRecord{}, nil},
		{"ext 里没这个 key", debugRecordWithExecuteIDs(""), nil},
		{"ext 值非法 JSON", debugRecordWithExecuteIDs(`not-json`), nil},
		{"ext 是空数组", debugRecordWithExecuteIDs(`[]`), nil},
		{"ext 全是空串", debugRecordWithExecuteIDs(`["",""]`), nil},
	}

	for _, tc := range cases {
		c := tc
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
			targetSvc.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(c.record, c.err)

			app := &EvalOpenAPIApplication{targetSvc: targetSvc}
			assert.Equal(t, []string{strconv.FormatInt(3003, 10)},
				app.debugSandboxExecuteIDs(context.Background(), 1001, 3003))
		})
	}
}

// 单沙箱评测对象 (executeID 就是裸 invokeID) 不受影响: ext 里就是这一个 id, 原样返回。
func TestDebugSandboxExecuteIDs_SingleSandboxUnchanged(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(debugRecordWithExecuteIDs(`["3003"]`), nil)

	app := &EvalOpenAPIApplication{targetSvc: targetSvc}
	assert.Equal(t, []string{"3003"}, app.debugSandboxExecuteIDs(context.Background(), 1001, 3003))
}

// 调试态必须**两个 ext key 都认**: 双沙箱有两套 operator 实现并存、各写自己的 key
// (与 service 层 sandboxExecuteIDsOf 同一约定)。
//
// 原来这里只读 sandbox_execute_ids 那个列表 key, 于是写 sandbox_agent_extra_execute_id
// 的那套实现在调试态**静默漏掉从沙箱** —— 而调试态走不到 service 层的
// destroySandboxExecuteIfNeeded (那里按 record.TargetVersionID 查 version 判类型,
// 调试用的 patchy target 从未落库), 本函数是调试态唯一的清理来源, 漏了只能等平台侧 patrol。
func TestDebugSandboxExecuteIDs_ReadsExtraExecuteIDKey(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(1001), int64(3003)).
		Return(&entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				Ext: map[string]string{entity.SandboxAgentExtKeyExtraExecuteID: "srv-gen-sub-1"},
			},
		}, nil)

	app := &EvalOpenAPIApplication{targetSvc: targetSvc}
	ids := app.debugSandboxExecuteIDs(context.Background(), 1001, 3003)

	// 该形态下主 execution 就是裸 invokeID, 从 execution 是 extra key 的值, 两个都要销。
	assert.Equal(t, []string{"3003", "srv-gen-sub-1"}, ids,
		"extra key 形态下必须销毁主 + 从两个 execution, 不能只销主的")
}

// 两个 key 同时存在时取并集并去重, 不能因为读了第二个 key 就给列表凭空多加一个裸 id。
func TestDebugSandboxExecuteIDs_UnionOfBothKeysDeduped(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(1001), int64(3003)).
		Return(&entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				Ext: map[string]string{
					consts.OutputDataExtKeySandboxExecuteIDs: `["3003-agent","3003-orch"]`,
					entity.SandboxAgentExtKeyExtraExecuteID:  "3003-orch",
				},
			},
		}, nil)

	app := &EvalOpenAPIApplication{targetSvc: targetSvc}
	ids := app.debugSandboxExecuteIDs(context.Background(), 1001, 3003)

	assert.Equal(t, []string{"3003-agent", "3003-orch", "3003"}, ids,
		"两 key 取并集且去重: 重复的 3003-orch 只出现一次")
}

// 只有列表 key 时**不得**补裸 invokeID —— 那个 id 在双沙箱下不对应任何 execution,
// 多销一个不存在的 id 会污染 affected_count 一致性校验、平添假告警。
func TestDebugSandboxExecuteIDs_ListOnlyDoesNotAddBareID(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(1001), int64(3003)).
		Return(debugRecordWithExecuteIDs(`["3003-agent","3003-orch"]`), nil)

	app := &EvalOpenAPIApplication{targetSvc: targetSvc}
	ids := app.debugSandboxExecuteIDs(context.Background(), 1001, 3003)

	assert.Equal(t, []string{"3003-agent", "3003-orch"}, ids)
	assert.NotContains(t, ids, "3003", "列表形态下裸 invokeID 不对应任何 execution, 不该补")
}
