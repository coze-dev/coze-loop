// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// SandboxStepEventMessage 一条评测链路阶段事件的 MQ 明细消息，消费侧同步到 Hive 做离线分析。
//
// **结构刻意是扁平的**：没有嵌套对象、没有 JSON-in-JSON。Dorado→Hive 的同步任务把每个 key 直接
// 映射成一列，嵌套结构要额外解析。前人把明细 append 到 eval_target_record.output_data 的 JSON
// 列里，既要行锁事务（SELECT ... FOR UPDATE → mutate → UPDATE，一次上报三次 DB 往返），也不是
// 分析友好的存储。
//
// **字段全带**：metric 只留了 6 个有界 tag，这里是唯一能回答「为什么」的地方——invoke_id 之类的
// 高基数标识在 metric 里是灾难，在 Hive 宽表里恰好是 join 键。宽表不怕多列。
type SandboxStepEventMessage struct {
	// 事件本体
	EventType string `json:"event_type"` // STARTED / FINISHED / 上报侧发来的未识别值原文
	StepName  string `json:"step_name"`
	AgentType string `json:"agent_type"`
	Round     int32  `json:"round"`

	// 结果三件套，仅 FINISHED 有意义。
	//
	// Success / DurationMs 是**指针**，STARTED 事件写 null 而不是零值。这不是洁癖：
	//   - success=false 是这张表上最重要的取值，用 bool 零值会让「阶段开始了」和「阶段失败了」
	//     在 Hive 里长得一模一样；
	//   - duration_ms 写 0 会让 avg(duration_ms) 把 STARTED 行算进分母，耗时直接腰斩；写 null
	//     则被 Hive 的聚合函数自动跳过。
	Success      *bool  `json:"success"`
	DurationMs   *int64 `json:"duration_ms"`
	ErrorCode    int32  `json:"error_code"`
	ErrorType    string `json:"error_type"` // 与 metric 的 error_type tag 同一套分类，便于两侧对账
	ErrorMessage string `json:"error_message"`

	// 仅 case 级事件携带的终态领域词汇，服务端只透传不解释。
	TrialStatus string `json:"trial_status"`
	EndReason   string `json:"end_reason"`

	// 身份 / join 键。全部来自上报侧，服务端不反查。
	WorkspaceID    int64  `json:"workspace_id"`
	InvokeID       int64  `json:"invoke_id"`
	ExperimentID   string `json:"experiment_id"`
	LogID          string `json:"log_id"`
	DatasetID      string `json:"dataset_id"`
	DatasetVersion string `json:"dataset_version"`
	ItemID         string `json:"item_id"`
	ItemKey        string `json:"item_key"`
	ModelName      string `json:"model_name"`

	// ⚠️ 两个时间戳，**基准不同**，名字必须能区分：
	//   SandboxEventTimeMs  事件在沙箱侧产生的时刻（上报侧的机器时钟），0 = 上报侧没给
	//   ServerReceiveTimeMs 服务端收到上报的时刻（本机时钟）
	// 两者之间隔一次 HTTP，且来自不同机器。混用会让任何时序分析得出错误结论——前人的
	// EventTimeMS 是服务端接收时刻而 DurationMS 是沙箱侧计时，这个坑已经踩过一次。
	SandboxEventTimeMs  int64 `json:"sandbox_event_time_ms"`
	ServerReceiveTimeMs int64 `json:"server_receive_time_ms"`
}
