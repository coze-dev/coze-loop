// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnlineExptTurnEvalResult_ExtAccessors(t *testing.T) {
	t.Parallel()

	// 正常值
	res := &OnlineExptTurnEvalResult{Ext: map[string]string{
		"span_id":  "sid-1",
		"trace_id": "tid-1",
	}}
	assert.Equal(t, "sid-1", res.GetSpanIDFromExt())
	assert.Equal(t, "tid-1", res.GetTraceIDFromExt())

	// nil 接收者
	var nilRes *OnlineExptTurnEvalResult
	assert.Equal(t, "", nilRes.GetSpanIDFromExt())
	assert.Equal(t, "", nilRes.GetTraceIDFromExt())
}

func TestOnlineExptTurnEvalResult_TimeDerivation_WithSpanTimes(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startMS := now.Add(-time.Minute).UnixMilli()
	endMS := now.UnixMilli()

	res := &OnlineExptTurnEvalResult{Ext: map[string]string{
		"span_start_time": strconv.FormatInt(startMS, 10),
		"span_end_time":   strconv.FormatInt(endMS, 10),
	}}

	// storageDuration 传入任意值也应忽略（因为有 span_*）
	assert.Equal(t, startMS, res.GetStartTimeFromExt(7))
	assert.Equal(t, endMS, res.GetEndTimeFromExt())
}

func TestOnlineExptTurnEvalResult_TimeDerivation_FromStartTime(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startMilli := now.UnixMilli()
	// ext 中 start_time 使用微秒（之前逻辑如此），这里遵循同样格式：毫秒*1000
	res := &OnlineExptTurnEvalResult{Ext: map[string]string{
		"start_time": strconv.FormatInt(startMilli*1000, 10),
	}}

	// storageDuration 为 2 天
	gotStart := res.GetStartTimeFromExt(2)
	gotEnd := res.GetEndTimeFromExt()

	expectedStart := startMilli - (24 * time.Duration(2) * time.Hour).Milliseconds()
	expectedEnd := startMilli + time.Hour.Milliseconds()
	assert.Equal(t, expectedStart, gotStart)
	assert.Equal(t, expectedEnd, gotEnd)
}

func TestOnlineExptTurnEvalResult_RunID(t *testing.T) {
	t.Parallel()

	res := &OnlineExptTurnEvalResult{Ext: map[string]string{"run_id": "123"}}
	id, err := res.GetRunID()
	require.NoError(t, err)
	assert.Equal(t, int64(123), id)

	// 缺失 run_id
	resMissing := &OnlineExptTurnEvalResult{Ext: map[string]string{}}
	_, err = resMissing.GetRunID()
	assert.Error(t, err)

	// 非法 run_id
	resInvalid := &OnlineExptTurnEvalResult{Ext: map[string]string{"run_id": "abc"}}
	_, err = resInvalid.GetRunID()
	assert.Error(t, err)
}

func TestOnlineExptTurnEvalResult_Workspace_Task_User_Platform(t *testing.T) {
	t.Parallel()

	res := &OnlineExptTurnEvalResult{
		Ext: map[string]string{
			"workspace_id":  "9",
			"task_id":       "101",
			"platform_type": string(loop_span.PlatformCallbackAll),
		},
		BaseInfo: &BaseInfo{CreatedBy: &UserInfo{UserID: "u-1"}},
	}

	wsStr, ws := res.GetWorkspaceIDFromExt()
	assert.Equal(t, "9", wsStr)
	assert.Equal(t, int64(9), ws)
	assert.Equal(t, int64(101), res.GetTaskIDFromExt())
	assert.Equal(t, "u-1", res.GetUserID())

	pt, ok := res.GetPlatformType()
	assert.True(t, ok)
	assert.Equal(t, loop_span.PlatformCallbackAll, pt)

	// 无用户、无平台
	res2 := &OnlineExptTurnEvalResult{Ext: map[string]string{"workspace_id": "bad", "task_id": "xx"}}
	wsStr2, ws2 := res2.GetWorkspaceIDFromExt()
	assert.Equal(t, "", wsStr2)
	assert.Equal(t, int64(0), ws2)
	assert.Equal(t, int64(0), res2.GetTaskIDFromExt())
	assert.Equal(t, "", res2.GetUserID())
	pt2, ok2 := res2.GetPlatformType()
	assert.False(t, ok2)
	assert.Equal(t, loop_span.PlatformCallbackAll, pt2)
}

func TestOnlineExptTurnEvalResult_InvalidNumbers(t *testing.T) {
	t.Parallel()

	// span_end_time 非法 -> GetEndTimeFromExt 返回 0
	res1 := &OnlineExptTurnEvalResult{Ext: map[string]string{"span_end_time": "abc"}}
	assert.Equal(t, int64(0), res1.GetEndTimeFromExt())

	// start_time 非法 -> GetStartTimeFromExt、GetEndTimeFromExt 返回 0
	res2 := &OnlineExptTurnEvalResult{Ext: map[string]string{"start_time": "not-a-number"}}
	assert.Equal(t, int64(0), res2.GetStartTimeFromExt(7))
	assert.Equal(t, int64(0), res2.GetEndTimeFromExt())

	// 同时存在 span_end_time（非法）和 start_time（合法） -> EndTime 优先取 span_end_time=0
	now := time.Now().UnixMilli()
	res3 := &OnlineExptTurnEvalResult{Ext: map[string]string{
		"span_end_time": "abc",
		"start_time":    strconv.FormatInt(now*1000, 10),
	}}
	assert.Equal(t, int64(0), res3.GetEndTimeFromExt())
}

func TestCorrectionEvent_ValidateAndAccessors(t *testing.T) {
	t.Parallel()

	// 校验成功
	evt := &CorrectionEvent{
		EvaluatorRecordID: 1,
		EvaluatorResult:   &EvaluatorResult{Correction: &Correction{Score: 0.5, Explain: "ok"}},
	}
	require.NoError(t, evt.Validate())

	// 缺失 recordID
	bad1 := &CorrectionEvent{EvaluatorResult: &EvaluatorResult{Correction: &Correction{}}}
	assert.Error(t, bad1.Validate())

	// 缺失 correction
	bad2 := &CorrectionEvent{EvaluatorRecordID: 2}
	assert.Error(t, bad2.Validate())
}

func TestCorrectionEvent_ExtAccessorsAndTimes(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startMS := now.Add(-time.Minute).UnixMilli()
	endMS := now.UnixMilli()

	// 使用 span_*
	evt := &CorrectionEvent{Ext: map[string]string{
		"span_id":         "sid-2",
		"trace_id":        "tid-2",
		"workspace_id":    "7",
		"task_id":         "88",
		"span_start_time": strconv.FormatInt(startMS, 10),
		"span_end_time":   strconv.FormatInt(endMS, 10),
	}}
	assert.Equal(t, "sid-2", evt.GetSpanIDFromExt())
	assert.Equal(t, "tid-2", evt.GetTraceIDFromExt())
	wsStr, ws := evt.GetWorkspaceIDFromExt()
	assert.Equal(t, "7", wsStr)
	assert.Equal(t, int64(7), ws)
	assert.Equal(t, int64(88), evt.GetTaskIDFromExt())
	assert.Equal(t, startMS, evt.GetStartTimeFromExt())
	assert.Equal(t, endMS, evt.GetEndTimeFromExt())

	// 使用 start_time 推导（微秒输入）
	startMicro := now.Add(-2*time.Minute).UnixMilli() * 1000
	evt2 := &CorrectionEvent{Ext: map[string]string{
		"start_time": strconv.FormatInt(startMicro, 10),
	}}
	expectedStart := (startMicro / 1000) - time.Hour.Milliseconds()
	expectedEnd := (startMicro / 1000) + time.Hour.Milliseconds()
	assert.Equal(t, expectedStart, evt2.GetStartTimeFromExt())
	assert.Equal(t, expectedEnd, evt2.GetEndTimeFromExt())

	// 平台类型与 updated_by
	evt3 := &CorrectionEvent{EvaluatorResult: &EvaluatorResult{Correction: &Correction{UpdatedBy: "u-2"}}, Ext: map[string]string{"platform_type": string(loop_span.PlatformCallbackAll)}}
	pt, ok := evt3.GetPlatformType()
	assert.True(t, ok)
	assert.Equal(t, loop_span.PlatformCallbackAll, pt)
	assert.Equal(t, "u-2", evt3.GetUpdateBy())

	// nil 接收者
	var nilEvt *CorrectionEvent
	assert.Equal(t, "", nilEvt.GetSpanIDFromExt())
	assert.Equal(t, "", nilEvt.GetTraceIDFromExt())
	assert.Equal(t, int64(0), nilEvt.GetTaskIDFromExt())
	wsStrNil, wsNil := nilEvt.GetWorkspaceIDFromExt()
	assert.Equal(t, "", wsStrNil)
	assert.Equal(t, int64(0), wsNil)
	assert.Equal(t, "", nilEvt.GetUpdateBy())
	// GetPlatformType 未对 nil 接收者做防御，调用将产生 panic
	require.Panics(t, func() { _, _ = nilEvt.GetPlatformType() })
}

func TestOnlineExptTurnEvalResult_InvalidSpanStartTime(t *testing.T) {
		// 当 span_start_time 非法时，GetStartTimeFromExt 返回 0（不会回退到 start_time）
	t.Parallel()

	res1 := &OnlineExptTurnEvalResult{Ext: map[string]string{"span_start_time": "bad"}}
	assert.Equal(t, int64(0), res1.GetStartTimeFromExt(7))

	// 同时存在非法的 span_start_time 与合法的 start_time，仍返回 0
	now := time.Now().UnixMilli()
	res2 := &OnlineExptTurnEvalResult{Ext: map[string]string{
		"span_start_time": "not-a-number",
		"start_time":      strconv.FormatInt(now*1000, 10),
	}}
	assert.Equal(t, int64(0), res2.GetStartTimeFromExt(7))
}

func TestCorrectionEvent_InvalidSpanStartTime(t *testing.T) {
		// 当 span_start_time 非法时，GetStartTimeFromExt 返回 0（不会回退到 start_time）
	t.Parallel()

	evt1 := &CorrectionEvent{Ext: map[string]string{"span_start_time": "NaN"}}
	assert.Equal(t, int64(0), evt1.GetStartTimeFromExt())

	// 同时存在非法的 span_start_time 与合法的 start_time，仍返回 0
	now := time.Now().UnixMilli()
	evt2 := &CorrectionEvent{Ext: map[string]string{
		"span_start_time": "bad",
		"start_time":      strconv.FormatInt(now*1000, 10),
	}}
	assert.Equal(t, int64(0), evt2.GetStartTimeFromExt())
}

func TestCorrectionEvent_InvalidNumbers(t *testing.T) {
	// 保留：测试 span_end_time 非法与 start_time 非法
	t.Parallel()

	// span_end_time 非法 -> GetEndTimeFromExt 返回 0
	evt1 := &CorrectionEvent{Ext: map[string]string{"span_end_time": "bad"}}
	assert.Equal(t, int64(0), evt1.GetEndTimeFromExt())

	// start_time 非法 -> GetStartTimeFromExt、GetEndTimeFromExt 返回 0
	evt2 := &CorrectionEvent{Ext: map[string]string{"start_time": "NaN"}}
	assert.Equal(t, int64(0), evt2.GetStartTimeFromExt())
	assert.Equal(t, int64(0), evt2.GetEndTimeFromExt())
}
