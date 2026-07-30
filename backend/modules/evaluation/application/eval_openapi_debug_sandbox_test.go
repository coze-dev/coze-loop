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
