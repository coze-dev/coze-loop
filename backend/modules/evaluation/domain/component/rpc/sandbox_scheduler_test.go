// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件钉住沙箱调度 RPC 的**跨服务 wire 契约**。这些枚举的整数值直接下发给沙箱调度服务，
// 两侧靠数值对齐、不靠名字：改一个数值在本仓编译通过、单测通过，运行期表现为
// "沙箱按另一种租户/另一种销毁方式创建", 且调度侧不会报错 —— 它只会照收到的数值执行。

// SandboxTenant 决定容器 start_cmd / env 注入 / case-file 等能力开关，Init 时下发且一个 task
// 只能配置一次。数值错了的后果按租户不同而不同, 但都不报错:
//
//   - 该给 FornaxEvalGeneral(4) 却发了 GeneralAgent(1)/LabelingAnalysis(2): 能力集不同，
//     双沙箱要的"裸沙箱 + 调用方全量下发"变成别的策略, orchestrator 起不来或被平台 env 污染;
//   - 该给 FornaxEvalGeneral(4) 却发了 Default(0): 调度侧按 FornaxTraeEval 注入平台 env
//     并自行写 case-file，与 operator 侧的编排打架。
//
// 因此这里逐值钉死, 而不是只断言"它们互不相同"。
func TestSandboxTenantWireValues(t *testing.T) {
	t.Parallel()

	assert.EqualValues(t, 0, SandboxTenantDefault, "Default = FornaxTraeEval, 单沙箱链路的兼容租户")
	assert.EqualValues(t, 1, SandboxTenantGeneralAgent)
	assert.EqualValues(t, 2, SandboxTenantLabelingAnalysis)
	assert.EqualValues(t, 3, SandboxTenantFornaxTraeEvalDualSandbox, "旧双沙箱链路")
	assert.EqualValues(t, 4, SandboxTenantFornaxEvalGeneral, "新双沙箱链路: 裸沙箱, 编排全在 operator 侧")

	// 五个租户必须占五个不同数值 —— 撞值意味着两种能力集在 wire 上不可区分。
	seen := map[SandboxTenant]bool{}
	for _, tenant := range []SandboxTenant{
		SandboxTenantDefault, SandboxTenantGeneralAgent, SandboxTenantLabelingAnalysis,
		SandboxTenantFornaxTraeEvalDualSandbox, SandboxTenantFornaxEvalGeneral,
	} {
		assert.False(t, seen[tenant], "租户数值 %d 重复", tenant)
		seen[tenant] = true
	}

	// Default 必须是零值: SandboxInitRequest.Tenant 不显式设置时就是它, 语义为"沿用调度侧默认租户"。
	// 若哪天把 Default 挪成非零, 所有没显式传 tenant 的调用会静默变成"租户 0 = 别的东西"。
	var unset SandboxInitRequest
	assert.Equal(t, SandboxTenantDefault, unset.Tenant,
		"未设置 Tenant 的 InitRequest 必须落在 Default 上")
}

// SandboxDestroyType 决定销的是**整个 task** 还是**单个 execution**。
// 这两个值搞反的后果极不对称, 且都不报错:
//
//   - 本该 Execute(1) 却发 Task(0): 一个 item 结束就把整个实验的沙箱任务销掉,
//     同实验其余正在跑的 item 全部被连带杀死 (表现为"实验莫名大面积失败");
//   - 本该 Task(0) 却发 Execute(1): 任务级清理只销掉一个 execution, 其余沙箱泄漏到 TTL。
//
// Task 必须是零值: 结构体零值即"销整个任务", 这是个危险默认 —— 任何忘记设 DestroyType 的
// 新调用点都会变成"销整个 task"。本断言把这个事实显式记下来, 便于评审新调用点时警觉。
func TestSandboxDestroyTypeWireValues(t *testing.T) {
	t.Parallel()

	assert.EqualValues(t, 0, SandboxDestroyTypeTask)
	assert.EqualValues(t, 1, SandboxDestroyTypeExecute)
	assert.NotEqual(t, SandboxDestroyTypeTask, SandboxDestroyTypeExecute)

	var unset SandboxDestroyRequest
	assert.Equal(t, SandboxDestroyTypeTask, unset.DestroyType,
		"DestroyType 零值是销整个 task —— 新增调用点必须显式设值")
}

// SandboxExecuteStatus 的数值来自调度服务, **刻意不连续** (Pending..Running 是 0-2,
// 终态跳到 10-13)。这段跳跃是有意义的: 判"是否终态"的地方按具体值枚举, 而不是按
// `status >= X` 之类的范围比较 —— 若有人"顺手补齐"中间的空号, 范围式判断会突然把
// 中间态当终态, 表现为**沙箱刚 Creating 就被判定失败并销毁**。
func TestSandboxExecuteStatusWireValues(t *testing.T) {
	t.Parallel()

	assert.EqualValues(t, 0, SandboxExecuteStatusPending)
	assert.EqualValues(t, 1, SandboxExecuteStatusCreating)
	assert.EqualValues(t, 2, SandboxExecuteStatusRunning)
	assert.EqualValues(t, 10, SandboxExecuteStatusSucceeded)
	assert.EqualValues(t, 11, SandboxExecuteStatusFailed)
	assert.EqualValues(t, 12, SandboxExecuteStatusCanceled)
	assert.EqualValues(t, 13, SandboxExecuteStatusFinished)

	// 中间态与终态之间必须留着空号 —— 空号被填掉时本断言 FAIL, 提醒复查所有状态判断。
	assert.Greater(t, int(SandboxExecuteStatusSucceeded), int(SandboxExecuteStatusRunning)+1,
		"中间态与终态之间刻意留空号; 若被填满, 任何范围式状态判断都要重新复查")

	seen := map[SandboxExecuteStatus]bool{}
	for _, s := range []SandboxExecuteStatus{
		SandboxExecuteStatusPending, SandboxExecuteStatusCreating, SandboxExecuteStatusRunning,
		SandboxExecuteStatusSucceeded, SandboxExecuteStatusFailed,
		SandboxExecuteStatusCanceled, SandboxExecuteStatusFinished,
	} {
		assert.False(t, seen[s], "状态数值 %d 重复", s)
		seen[s] = true
	}
}

// ISandboxSchedulerAdapter 的方法集是双沙箱 bring-up 的**能力清单**。
//
// WriteFile / RunCommand 是本分支新加的两个方法, 双沙箱链路缺一不可:
// session 建好后要先 WriteFile 把 case-file 写进 orchestrator 沙箱, 再 RunCommand 启动
// orchestrator 进程。少一个, 双沙箱就只是"起了两个空容器"。
//
// 断言方法集而不是"能编译" 的理由: 接口方法删除后, 唯一的实现(占位实现/商业版实现)也一起删掉时
// 编译照样通过 —— 只有调用点会消失, 而调用点消失恰恰是静默失效。这里把清单钉成显式契约。
func TestISandboxSchedulerAdapterMethodSet(t *testing.T) {
	t.Parallel()

	iface := reflect.TypeOf((*ISandboxSchedulerAdapter)(nil)).Elem()
	got := make([]string, 0, iface.NumMethod())
	for i := 0; i < iface.NumMethod(); i++ {
		got = append(got, iface.Method(i).Name)
	}
	assert.ElementsMatch(t,
		[]string{"Init", "Run", "Get", "GetTaskInfo", "Destroy", "WriteFile", "RunCommand"},
		got,
		"WriteFile/RunCommand 是双沙箱 bring-up 必需 (写 case-file + 起 orchestrator); 少一个双沙箱就只是两个空容器")
}

// 双沙箱 bring-up 的请求体形状: 这些字段是平台侧唯一的下发通道, 缺省值语义必须稳定。
func TestSandboxBringUpRequestDefaults(t *testing.T) {
	t.Parallel()

	// Run: Sync 默认 false = 异步、不回 SessionID。双沙箱必须显式 Sync=true 才拿得到
	// SessionID, 而后续 WriteFile/RunCommand 都以 execution 为锚 —— 默认值反了会让
	// bring-up 拿到空 SessionID 却不报错。
	var run SandboxRunRequest
	assert.False(t, run.Sync, "Sync 默认异步; 双沙箱需显式置 true 才拿得到 SessionID")
	assert.Nil(t, run.Files, "未设 Files 时不该凭空写文件进容器")

	// RunCommand: Async 默认 false = 同步并回 stdout/stderr/exit_code。
	// 若默认改成 true, 启动 orchestrator 的失败(非零退出码)就再也拿不到 —— bring-up
	// 会"看起来成功", 实际 orchestrator 从没起来。
	var cmd SandboxRunCommandRequest
	assert.False(t, cmd.Async, "默认同步执行, 否则 orchestrator 启动失败会被静默吞掉")
	assert.Empty(t, cmd.Cwd, "Cwd 空 = 服务端默认 /, 平台不猜工作目录")

	// WriteFile: 单个文件默认按 UTF-8 文本写入, IsBase64 必须显式开启。
	// 反了的话二进制内容会被当文本写坏, 而写入调用照样返回成功。
	var file SandboxFileWrite
	assert.False(t, file.IsBase64, "默认按 UTF-8 文本写入; 二进制必须显式声明, 否则内容写坏且不报错")

	// Destroy: ZombieTimeout 默认 false —— 它标记"本次销毁由 zombie 超时触发",
	// 收尾命令因此不同。默认为 true 会让正常结束的销毁也走 zombie 收尾。
	var destroy SandboxDestroyRequest
	assert.False(t, destroy.ZombieTimeout)
	assert.Nil(t, destroy.ExecuteIDs)
}

// SandboxWriteFileResponse.Results **必须与入参顺序一致** —— 这是接口注释里承诺的契约,
// 调用方据此判断"第 i 个文件写了多少字节"。这里用一次形状校验把该承诺显式化:
// 若某天改成 map 或允许乱序, 调用方按下标取结果就会张冠李戴 (报告错误的文件写入成功)。
func TestSandboxWriteFileResponseKeepsRequestOrder(t *testing.T) {
	t.Parallel()

	resp := &SandboxWriteFileResponse{
		TotalBytesWritten: 30,
		Results: []*SandboxFileWriteResult{
			{Path: "/tmp/case.json", BytesWritten: 10},
			{Path: "/tmp/env", BytesWritten: 20},
		},
	}
	require.Len(t, resp.Results, 2)
	assert.Equal(t, "/tmp/case.json", resp.Results[0].Path, "Results 必须与入参顺序一致 (接口契约)")
	assert.Equal(t, "/tmp/env", resp.Results[1].Path)

	var total int64
	for _, r := range resp.Results {
		total += r.BytesWritten
	}
	assert.Equal(t, resp.TotalBytesWritten, total, "TotalBytesWritten 是各文件字节数之和")
}
