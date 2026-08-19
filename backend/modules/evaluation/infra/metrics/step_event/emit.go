// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package step_event 评测链路（agent eval runtime 沙箱）阶段事件的在线指标上报。
//
// 指标名 evaluation_expt_sandbox_step，类型 [Counter, Timer]，靠 suffix 复用出三个指标：
//
//	evaluation_expt_sandbox_step.started    阶段开始数 (counter)
//	evaluation_expt_sandbox_step.finished   阶段结束数 (counter)
//	evaluation_expt_sandbox_step.duration   阶段耗时   (timer)
//
// 与前人的 evaluation_target_sandbox_agent 物理隔离：不同指标名、不同 tag 集合、不同上报接口。
package step_event

import (
	"strconv"
	"sync"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	metricName = "evaluation_expt_sandbox_step"

	suffixStarted  = "started"
	suffixFinished = "finished"
	suffixDuration = "duration"

	tagStepName  = "step_name"
	tagSuccess   = "success"
	tagErrorType = "error_type"
	tagErrorCode = "error_code"
	tagAgentType = "agent_type"
	tagRound     = "round"

	// tag 空值占位，遵循 fornax 平台约定。
	tagValuePlaceholder = "-"
)

// metricTagNames 是本指标的 tag **闭集**：tag 名在 NewMetric 时一次性固定，之后 Emit 只能填值，
// 不能新增 name。
//
// 恰好 6 个，全部有界（spec D3）：
//
//	step_name   13 个链路阶段名 + case 级保留名
//	success     true / false / -
//	error_type  - / engineering / non_engineering / unknown
//	error_code  runtime errno（几十个）
//	agent_type  agent 名（当前 10 个，月级增长，旧 agent 停跑后自然冷却）
//	round       轮次序号
//
// ⚠️ 不要加 event_type：started / finished 已经是两个不同的**指标名**（suffix 拼进最终名），
// 再加一个 tag 是冗余，且会在 started 指标下留一个恒为 "STARTED" 的 tag 白占一个闭集位置。
//
// ⚠️ 不要加 invoke_id / experiment_id / item_id / item_key / log_id / dataset_* / model_name。
// 理由见 eval_metrics.StepEventTags 的注释：它们是无界高基数标识，进 tag 会按笛卡尔积炸 series。
// 这些维度走 MQ 明细。
func metricTagNames() []string {
	return []string{
		tagStepName,
		tagSuccess,
		tagErrorType,
		tagErrorCode,
		tagAgentType,
		tagRound,
	}
}

var (
	once sync.Once
	impl eval_metrics.StepEventMetrics
)

// NewStepEventMetrics 构造进程级单例上报器。
//
// meter 缺失或 NewMetric 失败时降级为 no-op 并 warn，**绝不 panic / 返回 error**：埋点是
// best-effort 的旁路，让一个启动期的埋点配置问题拖垮整个服务是本末倒置。
func NewStepEventMetrics(meter metrics.Meter) eval_metrics.StepEventMetrics {
	once.Do(func() { impl = newStepEventMetrics(meter) })
	return impl
}

func newStepEventMetrics(meter metrics.Meter) eval_metrics.StepEventMetrics {
	if meter == nil {
		logs.Warn("[step_event_metrics] init: meter is nil, fallback to no-op")
		return &noopMetrics{}
	}
	m, err := meter.NewMetric(metricName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, metricTagNames())
	if err != nil || m == nil {
		logs.Warn("[step_event_metrics] init: NewMetric failed, fallback to no-op, err=%v, metric=%v", err, m)
		return &noopMetrics{}
	}
	logs.Info("[step_event_metrics] init: ok, metric=%s", metricName)
	return &metricsImpl{metric: m}
}

type metricsImpl struct {
	metric metrics.Metric
}

func (m *metricsImpl) EmitStepStarted(tags eval_metrics.StepEventTags) {
	if m == nil || m.metric == nil {
		return
	}
	// success / error_type / error_code 在 started 指标下**恒为占位符**。这不是 bug：三个指标
	// 共用一个 tag 闭集，必然有「某些 tag 在某些指标下没有意义」。填任何真值都是假数据——阶段刚
	// 开始，成败尚未发生。
	m.metric.Emit(buildTags(tags, tagValuePlaceholder, tagValuePlaceholder, 0),
		metrics.Counter(1, metrics.WithSuffix(suffixStarted)))
}

func (m *metricsImpl) EmitStepFinished(tags eval_metrics.StepEventTags, success bool, errorCode int32, durationMS int64) {
	if m == nil || m.metric == nil {
		return
	}
	// 负耗时 clamp 到 0：耗时是沙箱侧测的，跨机器时钟偏斜会产出负值，而负延迟比虚高更糟——
	// 虚高只是数字不准，负数会让读者对整个指标失去信任。
	if durationMS < 0 {
		durationMS = 0
	}
	m.metric.Emit(buildTags(tags, successTag(success), entity.ClassifyStepErrorType(success, errorCode), errorCode),
		metrics.Counter(1, metrics.WithSuffix(suffixFinished)),
		metrics.Timer(durationMS, metrics.WithSuffix(suffixDuration)))
}

func buildTags(t eval_metrics.StepEventTags, success, errType string, errCode int32) []metrics.T {
	return []metrics.T{
		// step_name / agent_type 源自平台外（沙箱上报），必须过字符集白名单。
		{Name: tagStepName, Value: sanitizeTagValue(t.StepName)},
		{Name: tagAgentType, Value: sanitizeTagValue(t.AgentType)},
		// round / error_code / success 是服务端从强类型字段格式化出来的，只可能是数字或
		// true/false，不需要 sanitize。
		{Name: tagRound, Value: strconv.FormatInt(int64(t.Round), 10)},
		{Name: tagSuccess, Value: success},
		{Name: tagErrorType, Value: errType},
		{Name: tagErrorCode, Value: errCodeTag(errCode)},
	}
}

func successTag(success bool) string {
	return strconv.FormatBool(success)
}

// errCodeTag 0 视为无错误码，走占位符；非 0 直接格式化。
func errCodeTag(v int32) string {
	if v == 0 {
		return tagValuePlaceholder
	}
	return strconv.FormatInt(int64(v), 10)
}

// sanitizeTagValue 把外部输入规整到 metrics 平台允许的字符集 a-zA-Z0-9._-/:%。
// 违规字符（含中文 / 空格 / 换行）统一替换为 '_'，空串返回占位符 '-'。
func sanitizeTagValue(v string) string {
	if v == "" {
		return tagValuePlaceholder
	}
	b := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.', c == '_', c == '-', c == '/', c == ':', c == '%':
			b = append(b, c)
		default:
			b = append(b, '_')
		}
	}
	return string(b)
}

type noopMetrics struct{}

func (n *noopMetrics) EmitStepStarted(_ eval_metrics.StepEventTags)                            {}
func (n *noopMetrics) EmitStepFinished(_ eval_metrics.StepEventTags, _ bool, _ int32, _ int64) {}
